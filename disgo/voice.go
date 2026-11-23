package disgo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"
	davesession "github.com/thomas-vilte/dave-go/session"
)

const (
	voiceOpenTimeout  = 10 * time.Second
	voiceCloseTimeout = 5 * time.Second
	// daveReadyTimeout bounds how long an alert waits on top of its own length,
	// since the audio sender doesn't pull frames until the DAVE session is ready.
	daveReadyTimeout = 10 * time.Second
)

// ErrVoiceStateNotFound means the member isn't connected to voice (or isn't cached).
var ErrVoiceStateNotFound = errors.New("voice state not found")

// Adapter implements voice state and audio operations on disgo.
// Voice connections are kept per guild and reused across alerts.
type Adapter struct {
	rest   rest.Rest
	caches cache.Caches
	vm     voice.Manager

	mu         sync.Mutex
	guildLocks map[snowflake.ID]*sync.Mutex
}

func NewAdapter(r rest.Rest, caches cache.Caches, vm voice.Manager) *Adapter {
	return &Adapter{
		rest:       r,
		caches:     caches,
		vm:         vm,
		guildLocks: make(map[snowflake.ID]*sync.Mutex),
	}
}

func (a *Adapter) UpdateVoiceState(gid, uid string, mute, deaf bool) error {
	ids, err := ParseIDs(gid, uid)
	if err != nil {
		return err
	}
	_, err = a.rest.UpdateMember(ids[0], ids[1], discord.MemberUpdate{
		Mute: &mute,
		Deaf: &deaf,
	})
	return err
}

// MoveMember moves a member into the voice channel cid.
func (a *Adapter) MoveMember(gid, uid string, cid pomomo.VoiceChannelID) error {
	ids, err := ParseIDs(gid, uid, string(cid))
	if err != nil {
		return err
	}
	_, err = a.rest.UpdateMember(ids[0], ids[1], discord.MemberUpdate{
		ChannelID: omit.NewPtr(ids[2]),
	})
	return err
}

// GetVoiceState returns the member's cached voice state; errors if they aren't in a voice channel.
func (a *Adapter) GetVoiceState(gid, uid string) (pomomo.VoiceState, error) {
	ids, err := ParseIDs(gid, uid)
	if err != nil {
		return pomomo.VoiceState{}, err
	}
	vs, ok := a.caches.VoiceState(ids[0], ids[1])
	if !ok || vs.ChannelID == nil {
		return pomomo.VoiceState{}, fmt.Errorf("%w for guild %s, user %s", ErrVoiceStateNotFound, gid, uid)
	}
	return pomomo.VoiceState{
		ChannelID: FormatID[pomomo.VoiceChannelID](*vs.ChannelID),
		Mute:      vs.GuildMute,
		Deaf:      vs.GuildDeaf,
	}, nil
}

func (a *Adapter) guildLock(gid snowflake.ID) *sync.Mutex {
	a.mu.Lock()
	defer a.mu.Unlock()
	l, ok := a.guildLocks[gid]
	if !ok {
		l = &sync.Mutex{}
		a.guildLocks[gid] = l
	}
	return l
}

// SendOpusAudio plays opus packets in voice channel cID, reusing the guild's voice connection.
// Returns once all packets have been handed to the audio sender.
func (a *Adapter) SendOpusAudio(ctx context.Context, packets [][]byte, gID string, cID pomomo.VoiceChannelID) error {
	if len(packets) == 0 {
		return nil
	}
	ids, err := ParseIDs(gID, string(cID))
	if err != nil {
		return err
	}

	l := a.guildLock(ids[0])
	l.Lock()
	defer l.Unlock()

	conn, err := a.conn(ctx, ids[0], ids[1])
	if err != nil {
		return err
	}

	p := newFrameProvider(packets)
	conn.SetOpusFrameProvider(p)

	timeout := time.Duration(len(packets))*voice.OpusFrameSizeMs*time.Millisecond + daveReadyTimeout
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		p.Close()
		return ctx.Err()
	case <-t.C:
		p.Close()
		return fmt.Errorf("timeout playing audio, dave ready: %t, dave protocol version: %d", conn.DAVE().Ready(), daveProtocolVersion(conn.DAVE()))
	}
}

// DaveSessionCreateFunc creates dave-go sessions for the voice manager.
// The audio sender gates on Ready, but dave-go's Ready stays false forever on channels
// without E2EE (protocol version 0), so Ready reports whether frames can be sent instead.
func DaveSessionCreateFunc() godave.SessionCreateFunc {
	return func(l *slog.Logger, uid godave.UserID, cb godave.Callbacks) godave.Session {
		return daveSession{davesession.New(uid, cb, davesession.WithLogger(l))}
	}
}

type daveSession struct{ *davesession.Session }

func (s daveSession) Ready() bool { return !s.ShouldHoldFrames() }

// daveProtocolVersion returns the session's negotiated DAVE protocol version, or -1 if unknown.
func daveProtocolVersion(s godave.Session) int {
	if ds, ok := s.(daveSession); ok {
		return int(ds.State().ProtocolVersion)
	}
	return -1
}

// conn returns a ready voice connection to channel cid, (re)opening it if needed. Caller holds the guild lock.
func (a *Adapter) conn(ctx context.Context, gid, cid snowflake.ID) (voice.Conn, error) {
	conn := a.vm.GetConn(gid)
	if conn != nil {
		if ch := conn.ChannelID(); ch != nil && *ch == cid && conn.Gateway().Status() == voice.StatusReady {
			return conn, nil
		}
		closeCtx, cancel := context.WithTimeout(context.Background(), voiceCloseTimeout)
		conn.Close(closeCtx)
		cancel()
	}

	conn = a.vm.CreateConn(gid)
	openCtx, cancel := context.WithTimeout(ctx, voiceOpenTimeout)
	defer cancel()
	if err := conn.Open(openCtx, cid, false, true); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), voiceCloseTimeout)
		conn.Close(closeCtx)
		cancel()
		return nil, fmt.Errorf("failed opening voice conn: %w", err)
	}
	return conn, nil
}

// CloseVoice disconnects from voice in guild gID, if connected.
func (a *Adapter) CloseVoice(ctx context.Context, gID string) error {
	gid, err := ParseID(gID)
	if err != nil {
		return err
	}
	l := a.guildLock(gid)
	l.Lock()
	defer l.Unlock()

	if conn := a.vm.GetConn(gid); conn != nil {
		conn.Close(ctx)
	}
	return nil
}

// frameProvider hands out a fixed set of opus frames to a voice.AudioSender, then signals done.
type frameProvider struct {
	mu     sync.Mutex
	frames [][]byte
	done   chan struct{}
	once   sync.Once
}

var _ voice.OpusFrameProvider = (*frameProvider)(nil)

func newFrameProvider(frames [][]byte) *frameProvider {
	return &frameProvider{
		frames: frames,
		done:   make(chan struct{}),
	}
}

func (p *frameProvider) ProvideOpusFrame() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.frames) == 0 {
		p.finish()
		return nil, io.EOF
	}
	f := p.frames[0]
	p.frames = p.frames[1:]
	return f, nil
}

func (p *frameProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frames = nil
	p.finish()
}

func (p *frameProvider) finish() {
	p.once.Do(func() { close(p.done) })
}

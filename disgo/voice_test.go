package disgo

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave"
	"github.com/disgoorg/snowflake/v2"
)

func TestFrameProvider(t *testing.T) {
	p := newFrameProvider([][]byte{{1}, {2}})
	for _, want := range []byte{1, 2} {
		f, err := p.ProvideOpusFrame()
		if err != nil || len(f) != 1 || f[0] != want {
			t.Fatalf("got %v, %v; want [%d]", f, err, want)
		}
	}
	select {
	case <-p.done:
		t.Fatal("done before frames exhausted")
	default:
	}
	if f, err := p.ProvideOpusFrame(); f != nil || !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v, %v", f, err)
	}
	<-p.done
	p.Close() // idempotent
}

func TestAdapterUpdateVoiceStateAndMove(t *testing.T) {
	m, reqs := newTestMessenger(t)
	a := NewAdapter(m.rest, cache.New(), nil)

	if err := a.UpdateVoiceState("555", "666", false, true); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveMember("555", "666", "777"); err != nil {
		t.Fatal(err)
	}
	got := reqs()
	if len(got) != 2 {
		t.Fatalf("unexpected requests: %+v", got)
	}
	for _, r := range got {
		if r.Method != http.MethodPatch || !strings.HasSuffix(r.Path, "/guilds/555/members/666") {
			t.Fatalf("unexpected request: %+v", r)
		}
	}
	// mute=false must be sent explicitly to unmute
	if v, ok := got[0].Body["mute"]; !ok || v != false || got[0].Body["deaf"] != true {
		t.Errorf("unexpected voice state body: %v", got[0].Body)
	}
	if got[1].Body["channel_id"] != "777" || len(got[1].Body) != 1 {
		t.Errorf("unexpected move body: %v", got[1].Body)
	}
	if err := a.UpdateVoiceState("", "666", true, true); err == nil {
		t.Error("expected error for empty guild id")
	}
}

func TestAdapterGetVoiceState(t *testing.T) {
	caches := cache.New(cache.WithCaches(cache.FlagVoiceStates))
	cid := snowflake.ID(777)
	caches.AddVoiceState(discord.VoiceState{GuildID: 555, UserID: 666, ChannelID: &cid, GuildMute: true})
	a := NewAdapter(nil, caches, nil)

	vs, err := a.GetVoiceState("555", "666")
	if err != nil {
		t.Fatal(err)
	}
	if vs.ChannelID != "777" || !vs.Mute || vs.Deaf {
		t.Errorf("unexpected voice state: %+v", vs)
	}
	if _, err := a.GetVoiceState("555", "999"); err == nil {
		t.Error("expected error for user not in voice")
	}
}

// fakes for SendOpusAudio

type fakeGateway struct {
	voice.Gateway
	status voice.Status
}

func (g *fakeGateway) Status() voice.Status { return g.status }

type fakeConn struct {
	voice.Conn
	mu        sync.Mutex
	channelID *snowflake.ID
	gateway   *fakeGateway
	dave      godave.Session
	openErr   error
	opens     int
	closes    int
	sent      [][]byte
	onRemove  func()
}

func (c *fakeConn) ChannelID() *snowflake.ID { c.mu.Lock(); defer c.mu.Unlock(); return c.channelID }
func (c *fakeConn) Gateway() voice.Gateway   { return c.gateway }
func (c *fakeConn) DAVE() godave.Session     { return c.dave }

func (c *fakeConn) Open(_ context.Context, channelID snowflake.ID, _, _ bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opens++
	if c.openErr != nil {
		return c.openErr
	}
	c.channelID = &channelID
	c.gateway.status = voice.StatusReady
	return nil
}

func (c *fakeConn) Close(context.Context) {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	c.onRemove()
}

// SetOpusFrameProvider drains the provider like voice's audio sender, gated on DAVE readiness.
func (c *fakeConn) SetOpusFrameProvider(p voice.OpusFrameProvider) {
	go func() {
		for {
			if !c.dave.Ready() {
				time.Sleep(time.Millisecond)
				continue
			}
			f, err := p.ProvideOpusFrame()
			if err != nil {
				return
			}
			c.mu.Lock()
			c.sent = append(c.sent, f)
			c.mu.Unlock()
		}
	}()
}

type notReadyDave struct{ godave.Session }

func (notReadyDave) Ready() bool { return false }

type fakeManager struct {
	voice.Manager
	mu      sync.Mutex
	conns   map[snowflake.ID]*fakeConn
	created int
	newConn func() *fakeConn
}

func (m *fakeManager) GetConn(gid snowflake.ID) voice.Conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[gid]; ok {
		return c
	}
	return nil
}

func (m *fakeManager) CreateConn(gid snowflake.ID) voice.Conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.newConn()
	c.onRemove = func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.conns, gid)
	}
	m.conns[gid] = c
	m.created++
	return c
}

func newFakeManager(dave godave.Session, openErr error) *fakeManager {
	return &fakeManager{
		conns: make(map[snowflake.ID]*fakeConn),
		newConn: func() *fakeConn {
			return &fakeConn{gateway: &fakeGateway{}, dave: dave, openErr: openErr}
		},
	}
}

func noopDave() godave.Session {
	return godave.NewNoopSession(slog.New(slog.DiscardHandler), "1", godave.Callbacks(nil))
}

func TestSendOpusAudioReusesConn(t *testing.T) {
	vm := newFakeManager(noopDave(), nil)
	a := NewAdapter(nil, nil, vm)
	ctx := context.Background()
	packets := [][]byte{{1}, {2}, {3}}

	if err := a.SendOpusAudio(ctx, packets, "555", "777"); err != nil {
		t.Fatal(err)
	}
	if err := a.SendOpusAudio(ctx, packets, "555", "777"); err != nil {
		t.Fatal(err)
	}
	if vm.created != 1 {
		t.Fatalf("expected conn reuse, created %d", vm.created)
	}
	conn := vm.conns[555]
	conn.mu.Lock()
	if len(conn.sent) != 6 {
		t.Errorf("expected 6 frames sent, got %d", len(conn.sent))
	}
	conn.mu.Unlock()

	// different channel → reconnect
	if err := a.SendOpusAudio(ctx, packets, "555", "888"); err != nil {
		t.Fatal(err)
	}
	if vm.created != 2 || conn.closes != 1 {
		t.Errorf("expected reconnect to new channel: created=%d closes=%d", vm.created, conn.closes)
	}

	// dropped gateway → reconnect
	vm.conns[555].gateway.status = voice.StatusDisconnected
	if err := a.SendOpusAudio(ctx, packets, "555", "888"); err != nil {
		t.Fatal(err)
	}
	if vm.created != 3 {
		t.Errorf("expected reconnect after gateway drop: created=%d", vm.created)
	}

	if err := a.CloseVoice(ctx, "555"); err != nil {
		t.Fatal(err)
	}
	if vm.GetConn(555) != nil {
		t.Error("expected conn removed after CloseVoice")
	}
}

func TestSendOpusAudioOpenFailure(t *testing.T) {
	vm := newFakeManager(noopDave(), errors.New("no voice server"))
	a := NewAdapter(nil, nil, vm)
	if err := a.SendOpusAudio(context.Background(), [][]byte{{1}}, "555", "777"); err == nil {
		t.Fatal("expected open error")
	}
	if vm.GetConn(555) != nil {
		t.Error("failed conn should be closed and removed")
	}
}

func TestSendOpusAudioCtxCancelWhileDaveNotReady(t *testing.T) {
	vm := newFakeManager(notReadyDave{}, nil)
	a := NewAdapter(nil, nil, vm)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := a.SendOpusAudio(ctx, [][]byte{{1}}, "555", "777")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected ctx deadline while DAVE not ready, got %v", err)
	}
}

func TestDaveSessionReady(t *testing.T) {
	s := DaveSessionCreateFunc()(slog.New(slog.DiscardHandler), "1", godave.Callbacks(nil))
	defer s.Close() //nolint

	// no E2EE on the channel: dave-go's own Ready would stay false forever
	s.OnSelectProtocolAck(0)
	if !s.Ready() {
		t.Fatal("want ready on protocol version 0")
	}
	if v := daveProtocolVersion(s); v != 0 {
		t.Fatalf("got protocol version %d, want 0", v)
	}

	// E2EE negotiated but no epoch yet: hold frames
	s.OnSelectProtocolAck(1)
	if s.Ready() {
		t.Fatal("want not ready during handshake")
	}
	if v := daveProtocolVersion(noopDave()); v != -1 {
		t.Fatalf("got protocol version %d for noop session, want -1", v)
	}
}

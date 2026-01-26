package main

import (
	"encoding/binary"
	"io"
	"os"
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

type Audio uint8

const (
	_ Audio = iota
	PomodoroAudio
	LongBreakAudio
	ShortBreakAudio
	IdleAudio
)

type AudioPlayer interface {
	Play(audio Audio, gID string, cID pomomo.VoiceChannelID) error
	Close()
}

func NewAudioPlayer(audioToOpusContainerPath map[Audio]string, cl *discordgo.Session) AudioPlayer {
	audioPackets := make(map[Audio][][]byte)
	loadPackets := func(audio Audio, opusContainerPath string) {
		if opusContainerPath == "" {
			log.Info("no opusContainerPath - skip loading", "audio", audio)
			return
		}
		log.Info("loading packets", "audio", audio, "opusContainerPath", opusContainerPath)
		f, err := os.Open(opusContainerPath)
		if err != nil {
			panic(err)
		}
		defer f.Close() //nolint

		var frameLen int16
		// Don't wait for the first tick, run immediately.
		for {
			err = binary.Read(f, binary.LittleEndian, &frameLen)
			if err != nil {
				if err == io.EOF {
					return
				}
				panic("error reading file: " + err.Error())
			}

			// Read encoded pcm from dca file.
			packet := make([]byte, frameLen)
			if err := binary.Read(f, binary.LittleEndian, &packet); err != nil {
				// Should not be any end of file errors
				panic(err)
			}

			// Append encoded pcm data to the buffer.
			audioPackets[audio] = append(audioPackets[audio], packet)
		}
	}
	for audio, opusContainerPath := range audioToOpusContainerPath {
		loadPackets(audio, opusContainerPath)
	}
	return &audioPlayer{
		audioPackets: audioPackets,
		cl:           cl,
	}
}

type audioPlayer struct {
	audioPackets map[Audio][][]byte
	cl           *discordgo.Session
}

func (m *audioPlayer) Close() {
	var wg sync.WaitGroup
	for _, conn := range m.cl.VoiceConnections {
		wg.Go(func() {
			_ = conn.Disconnect()
		})
	}
	wg.Wait()
}

func (m *audioPlayer) Play(audio Audio, gID string, cID pomomo.VoiceChannelID) error {
	packets := m.audioPackets[audio]
	if packets == nil {
		log.Debug("no audio packets found - skipping play", "audio", audio)
		return nil
	}
	conn, err := m.cl.ChannelVoiceJoin(gID, string(cID), false, true)
	if err != nil {
		return err
	}
	if err := conn.Speaking(true); err != nil {
		return err
	}
	for _, p := range packets {
		conn.OpusSend <- p
	}
	return conn.Speaking(false)
}

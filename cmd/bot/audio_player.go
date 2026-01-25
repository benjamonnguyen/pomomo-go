package main

import (
	"io"
	"os"
	"sync"

	"github.com/bwmarrin/discordgo"
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
	Play(audio Audio, gID, cID string) error
	Close()
}

func NewAudioPlayer(audioToOpusContainerPath map[Audio]string, cl *discordgo.Session) AudioPlayer {
	audioData := make(map[Audio][]byte)
	loadAudio := func(audio Audio, opusContainerPath string) {
		f, err := os.Open(opusContainerPath)
		if err != nil {
			panic(err)
		}

		data, err := io.ReadAll(f)
		if err != nil {
			panic(err)
		}
		audioData[audio] = data
	}
	for audio, opusContainerPath := range audioToOpusContainerPath {
		loadAudio(audio, opusContainerPath)
	}
	return &audioPlayer{
		audioData: audioData,
		cl:        cl,
	}
}

type audioPlayer struct {
	audioData map[Audio][]byte
	cl        *discordgo.Session
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

func (m *audioPlayer) Play(audio Audio, gID, cID string) error {
	audioData := m.audioData[audio]
	if audioData == nil {
		panic("no audio data for " + string(audio))
	}
	conn, err := m.cl.ChannelVoiceJoin(gID, cID, false, true)
	if err != nil {
		return err
	}
	if err := conn.Speaking(true); err != nil {
		return err
	}
	conn.OpusSend <- audioData
	return conn.Speaking(false)
}

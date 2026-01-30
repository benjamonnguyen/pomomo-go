package main

import (
	"embed"
	"encoding/binary"
	"io"
	"path"

	"github.com/charmbracelet/log"
)

type audio string

const (
	PomodoroAudio   audio = "pomodoro.dca"
	LongBreakAudio  audio = "long_break.dca"
	ShortBreakAudio audio = "short_break.dca"
)

func newOpusAudioLoader(fs embed.FS) *opusAudioLoader {
	audioPackets := make(map[audio][][]byte)
	loadPackets := func(audio audio, fs embed.FS) {
		log.Info("loading packets", "audio", audio)
		f, err := fs.Open(path.Join("sounds", string(audio)))
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
	for _, a := range []audio{
		PomodoroAudio,
		LongBreakAudio,
		ShortBreakAudio,
	} {
		loadPackets(a, fs)
	}
	return &opusAudioLoader{
		audioPackets: audioPackets,
	}
}

type opusAudioLoader struct {
	audioPackets map[audio][][]byte
}

func (m *opusAudioLoader) Load(audio audio) [][]byte {
	return m.audioPackets[audio]
}

package main

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/charmbracelet/log"
)

type audio uint8

const (
	_ audio = iota
	PomodoroAudio
	LongBreakAudio
	ShortBreakAudio
	IdleAudio
)

func newOpusAudioLoader(audioToOpusContainerPath map[audio]string) *opusAudioLoader {
	audioPackets := make(map[audio][][]byte)
	loadPackets := func(audio audio, opusContainerPath string) {
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

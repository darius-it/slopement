package app

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/audio"
)

// The notification sound. The system's own notification sound differs by
// platform (and Linux servers often play none), so the app plays one of its own,
// a soft two-note chime made here in memory (no file shipped), through mygo's
// audio. The system notification is shown silent. Nothing is decoded or opened
// until the first message that sounds, and the output is not touched again until
// the next.

// chimeWAV is the chime as a 16-bit mono WAV: two sine notes (E6, then A6), each
// with a fast attack and a smooth decay, at a gentle level.
func chimeWAV() []byte {
	const rate = 44100
	notes := []struct {
		hz       float64
		start, d float64 // seconds
	}{{1318.5, 0, 0.30}, {1760.0, 0.11, 0.42}}
	total := 0.11 + 0.42 + 0.05
	n := int(total * rate)
	samples := make([]int16, n)
	for _, nt := range notes {
		first := int(nt.start * rate)
		for i := 0; i < int(nt.d*rate) && first+i < n; i++ {
			t := float64(i) / rate
			attack := math.Min(1, t/0.004)
			env := attack * math.Exp(-t*9/nt.d)
			v := math.Sin(2*math.Pi*nt.hz*t) * env * 0.22
			samples[first+i] += int16(v * 32767)
		}
	}
	var b bytes.Buffer
	data := len(samples) * 2
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+data))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&b, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(data))
	binary.Write(&b, binary.LittleEndian, samples)
	return b.Bytes()
}

var chime struct {
	once  sync.Once
	sound *audio.Sound
	last  time.Time
}

// playChime sounds the notification chime, at most once every 1.5 seconds (a
// burst of messages is one sound). A variable, so tests see when it would play.
var playChime = func() {
	if time.Since(chime.last) < 1500*time.Millisecond {
		return
	}
	chime.last = time.Now()
	chime.once.Do(func() {
		s, err := audio.DecodeSound(chimeWAV())
		if err != nil {
			log.Printf("chime: %v", err)
			return
		}
		chime.sound = s
	})
	if chime.sound != nil {
		chime.sound.Play()
	}
}

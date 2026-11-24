// Package voice is what a recorded voice message is made of, apart from the
// microphone: the samples written as a WAV file, the loudness bars shown for
// it, and the live level while recording. It has no platform code.
package voice

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"
)

// SampleRate is the rate voice is recorded and sent at: 16 kHz mono is enough
// for speech and keeps a minute under 2 MB as 16-bit WAV.
const SampleRate = 16000

// MaxDuration is the longest message that can be recorded.
const MaxDuration = 15 * time.Minute

// Duration is how long samples at SampleRate last.
func Duration(samples int) time.Duration {
	return time.Duration(samples) * time.Second / SampleRate
}

// PCM reads 16-bit little-endian samples from bytes, as the microphone modules
// yield them. A trailing odd byte is dropped.
func PCM(b []byte) []int16 {
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out
}

// EncodeWAV writes 16-bit mono PCM samples at SampleRate as a WAV file.
func EncodeWAV(pcm []int16) []byte {
	data := len(pcm) * 2
	var b bytes.Buffer
	b.Grow(44 + data)
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+data))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(&b, binary.LittleEndian, uint16(1)) // mono
	_ = binary.Write(&b, binary.LittleEndian, uint32(SampleRate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(SampleRate*2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(data))
	_ = binary.Write(&b, binary.LittleEndian, pcm)
	return b.Bytes()
}

// Level is the loudness of a run of samples from 0 (silence) to 1: the root
// mean square, lifted so that quiet speech still moves the meter.
func Level(pcm []int16) float32 {
	if len(pcm) == 0 {
		return 0
	}
	var sum float64
	for _, s := range pcm {
		f := float64(s) / 32768
		sum += f * f
	}
	rms := math.Sqrt(sum / float64(len(pcm)))
	// 20 dB per decade, from -50 dB (nothing) to 0 dB (full scale).
	db := 20 * math.Log10(rms+1e-9)
	return float32(math.Max(0, math.Min(1, (db+50)/50)))
}

// WaveformBars is how many bars a voice message's waveform has (Element's
// default is 100; the spec allows up to 1024 values, each 0 to 1024).
const WaveformBars = 100

// Waveform divides the recording into WaveformBars equal parts and gives each
// one's loudness as 0 to 1024, the form of org.matrix.msc1767.audio.
func Waveform(pcm []int16) []int {
	out := make([]int, WaveformBars)
	if len(pcm) == 0 {
		return out
	}
	for i := range out {
		lo, hi := i*len(pcm)/WaveformBars, (i+1)*len(pcm)/WaveformBars
		if hi <= lo {
			hi = min(lo+1, len(pcm))
		}
		out[i] = int(Level(pcm[lo:hi]) * 1024)
	}
	return out
}

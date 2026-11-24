package voice

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"
)

func tone(n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * 32767 * math.Sin(2*math.Pi*440*float64(i)/SampleRate))
	}
	return out
}

// The WAV is a real one: the right header, and the audio package decodes it to
// as long as it was recorded.
func TestEncodeWAVIsAValidFile(t *testing.T) {
	pcm := tone(SampleRate*2, 0.5) // two seconds
	w := EncodeWAV(pcm)
	if string(w[:4]) != "RIFF" || string(w[8:16]) != "WAVEfmt " || string(w[36:40]) != "data" {
		t.Fatalf("header: %q", w[:44])
	}
	if got := binary.LittleEndian.Uint32(w[40:44]); int(got) != len(pcm)*2 {
		t.Fatalf("data size %d, want %d", got, len(pcm)*2)
	}
	if got := binary.LittleEndian.Uint32(w[24:28]); got != SampleRate {
		t.Fatalf("sample rate %d", got)
	}
	s, err := audio.DecodeSound(w)
	if err != nil {
		t.Fatalf("the audio package cannot read it: %v", err)
	}
	if d := s.Duration(); d < 1900*time.Millisecond || d > 2100*time.Millisecond {
		t.Fatalf("decoded length %v, want about 2s", d)
	}
	if !bytes.Equal(EncodeWAV(nil)[:4], []byte("RIFF")) {
		t.Fatal("an empty recording has no header")
	}
	if Duration(SampleRate*3) != 3*time.Second {
		t.Fatal("Duration")
	}
}

// Bytes from the microphone module are little-endian 16-bit samples.
func TestPCMReadsLittleEndian(t *testing.T) {
	got := PCM([]byte{0x01, 0x00, 0xFF, 0xFF, 0x00, 0x80, 0x7F})
	want := []int16{1, -1, -32768}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v, want %v", got, want)
		}
	}
	if len(PCM(nil)) != 0 {
		t.Fatal("nil gives samples")
	}
}

// Silence reads 0, a loud tone near 1, and louder is higher.
func TestLevel(t *testing.T) {
	if Level(nil) != 0 || Level(make([]int16, 800)) != 0 {
		t.Fatal("silence is not 0")
	}
	quiet, loud := Level(tone(800, 0.02)), Level(tone(800, 0.8))
	if !(quiet > 0 && quiet < loud && loud <= 1) || loud < 0.8 {
		t.Fatalf("quiet %.2f, loud %.2f", quiet, loud)
	}
}

// The waveform has the spec's shape: 100 bars of 0 to 1024 that follow the
// loudness over the recording, quiet first and loud after.
func TestWaveformFollowsTheRecording(t *testing.T) {
	pcm := append(make([]int16, SampleRate), tone(SampleRate, 0.7)...) // 1 s of silence, 1 s of tone
	w := Waveform(pcm)
	if len(w) != WaveformBars {
		t.Fatalf("%d bars", len(w))
	}
	for i, v := range w {
		if v < 0 || v > 1024 {
			t.Fatalf("bar %d is %d", i, v)
		}
	}
	if w[10] != 0 || w[90] < 800 {
		t.Fatalf("bar 10 = %d, bar 90 = %d", w[10], w[90])
	}
	if len(Waveform(nil)) != WaveformBars || len(Waveform(tone(30, 0.5))) != WaveformBars {
		t.Fatal("a short recording does not give 100 bars")
	}
}

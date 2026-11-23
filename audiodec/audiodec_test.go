package audiodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/egoist/mygo/audio"
	"github.com/kazzmir/opus-go/ogg"
	"github.com/kazzmir/opus-go/opus"
	"github.com/tphakala/go-aac/pcm"
	"github.com/tphakala/go-m4a/aacm4a"
)

// tone is a sine at hz, as 16-bit mono PCM samples.
func tone(rate, hz int, seconds float64) []int16 {
	out := make([]int16, int(float64(rate)*seconds))
	for i := range out {
		out[i] = int16(12000 * math.Sin(2*math.Pi*float64(hz)*float64(i)/float64(rate)))
	}
	return out
}

// oggOpus encodes mono 48 kHz samples as an Ogg Opus file, with the encoder
// of the same library the decoder is from (there is no other in pure Go).
func oggOpus(t *testing.T, samples []int16) []byte {
	t.Helper()
	enc, err := opus.NewEncoder(48000, 1, opus.ApplicationAudio)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	preSkip, err := enc.PreSkip()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	pw := ogg.NewPageWriter(&out, 1234)
	head, err := ogg.BuildOpusHeadPacket(ogg.OpusHead{Version: 1, Channels: 1, PreSkip: uint16(preSkip), InputSampleRate: 48000})
	if err != nil {
		t.Fatal(err)
	}
	tags, err := ogg.BuildOpusTagsPacket(ogg.OpusTags{Vendor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := pw.WriteHeaderPacket(head, true); err != nil {
		t.Fatal(err)
	}
	if err := pw.WriteHeaderPacket(tags, false); err != nil {
		t.Fatal(err)
	}
	const frame = 960 // 20 ms
	packet := make([]byte, 4000)
	granule := uint64(preSkip)
	for at := 0; at < len(samples); at += frame {
		chunk := make([]int16, frame)
		copy(chunk, samples[at:min(at+frame, len(samples))])
		n, err := enc.Encode(chunk, frame, packet)
		if err != nil {
			t.Fatal(err)
		}
		granule += frame
		last := at+frame >= len(samples)
		if err := pw.WritePacket(append([]byte(nil), packet[:n]...), granule, last); err != nil {
			t.Fatal(err)
		}
	}
	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// parseWAV reads what wav() wrote.
func parseWAV(t *testing.T, b []byte) (rate, channels int, samples []int16) {
	t.Helper()
	if len(b) < 44 || string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || string(b[36:40]) != "data" {
		t.Fatalf("not a WAV: % x", b[:min(len(b), 44)])
	}
	if binary.LittleEndian.Uint16(b[20:]) != 1 || binary.LittleEndian.Uint16(b[34:]) != 16 {
		t.Fatal("not 16-bit PCM")
	}
	channels, rate = int(binary.LittleEndian.Uint16(b[22:])), int(binary.LittleEndian.Uint32(b[24:]))
	n := int(binary.LittleEndian.Uint32(b[40:]))
	if n != len(b)-44 {
		t.Fatalf("data size %d, file has %d", n, len(b)-44)
	}
	for i := 0; i+1 < n; i += 2 {
		samples = append(samples, int16(binary.LittleEndian.Uint16(b[44+i:])))
	}
	return
}

func peak(s []int16) int {
	p := 0
	for _, v := range s {
		p = max(p, int(math.Abs(float64(v))))
	}
	return p
}

// Ogg Opus decodes to a WAV of the clip's length, with sound in it, and tells
// the formats it does not know.
func TestOggOpusToWAV(t *testing.T) {
	file := oggOpus(t, tone(48000, 440, 0.6))
	if !IsOggOpus(file) || IsMP4(file) {
		t.Fatal("the file is not seen as Ogg Opus")
	}
	wavData, err := ToWAV(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := audio.DecodeSound(wavData); err != nil {
		t.Fatalf("mygo's audio cannot read the WAV: %v", err)
	}
	rate, ch, s := parseWAV(t, wavData)
	if rate != 48000 || ch != 1 {
		t.Fatalf("%d Hz, %d channels", rate, ch)
	}
	if secs := float64(len(s)) / 48000; secs < 0.55 || secs > 0.75 {
		t.Fatalf("the clip is %.2f s, want about 0.6", secs)
	}
	if p := peak(s); p < 4000 {
		t.Fatalf("the decoded clip is nearly silent (peak %d)", p)
	}

	if _, err := ToWAV([]byte("RIFF....WAVEfmt ")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("a WAV: %v", err)
	}
	if _, err := ToWAV(file[:60]); err == nil {
		t.Fatal("a file cut short decoded")
	}
}

// memFile is a WriteSeeker in memory (the encoder writes an .m4a to one).
type memFile struct {
	b   []byte
	pos int
}

func (m *memFile) Write(p []byte) (int, error) {
	if end := m.pos + len(p); end > len(m.b) {
		m.b = append(m.b, make([]byte, end-len(m.b))...)
	}
	copy(m.b[m.pos:], p)
	m.pos += len(p)
	return len(p), nil
}

func (m *memFile) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.pos = int(off)
	case io.SeekCurrent:
		m.pos += int(off)
	case io.SeekEnd:
		m.pos = len(m.b) + int(off)
	}
	return int64(m.pos), nil
}

// AAC in an .m4a decodes to a WAV with the clip's rate, channels and length.
func TestAACInM4AToWAV(t *testing.T) {
	samples := tone(44100, 440, 0.8)
	raw := make([]byte, 2*len(samples))
	for i, v := range samples {
		binary.LittleEndian.PutUint16(raw[2*i:], uint16(v))
	}
	var f memFile
	if err := aacm4a.EncodeInterleaved(&f, pcm.Config{SampleRate: 44100, BitDepth: 16, Channels: 1, Bitrate: 64000}, raw); err != nil {
		t.Fatal(err)
	}
	if !IsMP4(f.b) || IsOggOpus(f.b) {
		t.Fatal("the file is not seen as an MP4")
	}
	wavData, err := ToWAV(f.b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := audio.DecodeSound(wavData); err != nil {
		t.Fatalf("mygo's audio cannot read the WAV: %v", err)
	}
	rate, ch, s := parseWAV(t, wavData)
	if rate != 44100 || ch != 1 {
		t.Fatalf("%d Hz, %d channels", rate, ch)
	}
	if secs := float64(len(s)) / 44100; secs < 0.7 || secs > 1.0 {
		t.Fatalf("the clip is %.2f s, want about 0.8", secs)
	}
	if p := peak(s); p < 4000 {
		t.Fatalf("the decoded clip is nearly silent (peak %d)", p)
	}
}

package audiodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/egoist/mygo/audio"
	"github.com/kazzmir/opus-go/ogg"
	"github.com/kazzmir/opus-go/opus"
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
	if !IsOggOpus(file) {
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

// AAC in an .m4a is not decoded: it is said to be unsupported, not mistaken for
// Opus, so the card can offer the file instead.
func TestAACInM4AIsUnsupported(t *testing.T) {
	// The start of an .m4a: a box size, "ftyp", the "M4A " brand.
	m4a := append([]byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'M', '4', 'A', ' '}, make([]byte, 64)...)
	if IsOggOpus(m4a) {
		t.Fatal("an .m4a is seen as Ogg Opus")
	}
	if _, err := ToWAV(m4a); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("an .m4a: %v", err)
	}
}

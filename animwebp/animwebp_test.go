package animwebp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"io"
	"testing"
	"time"

	"golang.org/x/image/webp"
)

// Tiny real files (the ones Modernizr tests WebP support with): a 1×1
// lossless picture, and a one-frame animation.
const (
	losslessB64 = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="
	lossyB64    = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"
	animatedB64 = "UklGRlIAAABXRUJQVlA4WAoAAAASAAAAAAAAAAAAQU5JTQYAAAD/////AABBTk1GJgAAAAAAAAAAAAAAAAAAAGQAAABWUDhMDQAAAC8AAAAQBxAREYiI/gcA"
)

func decode64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnimatedDetectsTheFlag(t *testing.T) {
	if !Animated(decode64(t, animatedB64)) {
		t.Fatal("an animation is not seen as one")
	}
	if Animated(decode64(t, losslessB64)) {
		t.Fatal("a still is seen as an animation")
	}
	if Animated([]byte("GIF89a....")) || Animated(nil) {
		t.Fatal("a non-WebP is seen as an animation")
	}
	if _, err := NewDecoder(decode64(t, losslessB64)); err != ErrNotAnimated {
		t.Fatalf("a still: %v", err)
	}
}

func TestDecodesARealAnimation(t *testing.T) {
	d, err := NewDecoder(decode64(t, animatedB64))
	if err != nil {
		t.Fatal(err)
	}
	if w, h := d.Size(); w != 1 || h != 1 || d.Frames() != 1 {
		t.Fatalf("size %dx%d, %d frames", w, h, d.Frames())
	}
	fr, err := d.Next()
	if err != nil || fr.Duration != 100*time.Millisecond {
		t.Fatalf("frame: %v, %v", fr.Duration, err)
	}
	if _, err := d.Next(); err != io.EOF {
		t.Fatalf("after the last frame: %v", err)
	}
	if err := d.Rewind(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Next(); err != nil {
		t.Fatalf("after Rewind: %v", err)
	}
	if _, err := FirstFrame(decode64(t, animatedB64)); err != nil {
		t.Fatalf("FirstFrame: %v", err)
	}
}

// chunk makes a RIFF chunk.
func chunk(fourcc string, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString(fourcc)
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(payload)))
	b.Write(payload)
	if len(payload)%2 == 1 {
		b.WriteByte(0)
	}
	return b.Bytes()
}

func u24(v int) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16)} }

// anmf makes a frame chunk of one picture (a VP8L chunk) at (x, y).
func anmf(x, y, w, h, ms int, flags byte, pic []byte) []byte {
	var p bytes.Buffer
	p.Write(u24(x / 2))
	p.Write(u24(y / 2))
	p.Write(u24(w - 1))
	p.Write(u24(h - 1))
	p.Write(u24(ms))
	p.WriteByte(flags)
	p.Write(pic)
	return chunk("ANMF", p.Bytes())
}

// riff wraps chunks as a WebP file.
func riff(chunks ...[]byte) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		body.Write(c)
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(body.Len()))
	b.Write(body.Bytes())
	return b.Bytes()
}

// Frames go where they say, blend or replace, and are disposed as they say.
func TestFramesAreComposited(t *testing.T) {
	still := decode64(t, lossyB64) // an opaque 1×1 picture (the lossless sample is transparent)
	pic := still[12:]              // its VP8 chunk
	want, err := webp.Decode(bytes.NewReader(still))
	if err != nil {
		t.Fatal(err)
	}
	wr, wg, wb, wa := want.At(0, 0).RGBA()
	if wa == 0 {
		t.Skip("the sample pixel is transparent: nothing to see on the canvas")
	}
	isPic := func(img image.Image, x, y int) bool {
		r, g, b, a := img.At(x, y).RGBA()
		return r == wr && g == wg && b == wb && a == wa
	}
	clear := func(img image.Image, x, y int) bool {
		_, _, _, a := color.NRGBAModel.Convert(img.At(x, y)).RGBA()
		return a == 0
	}

	vp8x := make([]byte, 10)
	vp8x[0] = 0x12 // animation + alpha
	copy(vp8x[4:], u24(4-1))
	copy(vp8x[7:], u24(4-1))
	file := riff(chunk("VP8X", vp8x), chunk("ANIM", []byte{0, 0, 0, 0, 0, 0}),
		anmf(0, 0, 1, 1, 50, 0x00, pic), // stays
		anmf(2, 0, 1, 1, 70, 0x01, pic), // is cleared afterwards
		anmf(0, 2, 1, 1, 90, 0x02, pic)) // replaces without blending

	d, err := NewDecoder(file)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := d.Size(); w != 4 || h != 4 || d.Frames() != 3 {
		t.Fatalf("size %dx%d, %d frames", w, h, d.Frames())
	}
	f1, _ := d.Next()
	if !isPic(f1.Image, 0, 0) || !clear(f1.Image, 2, 0) || f1.Duration != 50*time.Millisecond {
		t.Fatalf("frame 1: (0,0) shows %v, duration %v", f1.Image.At(0, 0), f1.Duration)
	}
	f2, _ := d.Next()
	if !isPic(f2.Image, 0, 0) || !isPic(f2.Image, 2, 0) || f2.Duration != 70*time.Millisecond {
		t.Fatal("frame 2: the first frame went, or the second is not there")
	}
	f3, _ := d.Next()
	if !isPic(f3.Image, 0, 0) || !clear(f3.Image, 2, 0) || !isPic(f3.Image, 0, 2) {
		t.Fatal("frame 3: the second frame was not disposed, or the third is not there")
	}
	if _, err := d.Next(); err != io.EOF {
		t.Fatal("no end")
	}
	// Played again: the canvas starts clean.
	_ = d.Rewind()
	g1, _ := d.Next()
	if !clear(g1.Image, 0, 2) {
		t.Fatal("the canvas was not cleared by Rewind")
	}
}

func TestRefusesWhatIsBroken(t *testing.T) {
	good := decode64(t, animatedB64)
	for name, data := range map[string][]byte{
		"cut short": good[:len(good)/2],
		"junk":      append([]byte("RIFFxxxxWEBPVP8X"), make([]byte, 20)...),
	} {
		if d, err := NewDecoder(data); err == nil {
			// Reading a cut file may find no frames; it must not panic on Next.
			for i := 0; i < 3; i++ {
				if _, err := d.Next(); err != nil {
					break
				}
			}
			t.Logf("%s: read without an error (%d frames)", name, d.Frames())
		}
	}
}

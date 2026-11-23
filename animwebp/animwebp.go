// Package animwebp plays animated WebP, which neither mygo nor
// golang.org/x/image can: it reads the container (RIFF, VP8X, ANIM, ANMF),
// has x/image decode each frame's picture, and puts the frames on the canvas as
// the format says (their places, blending and disposal). Decoder is a
// ui.VideoDecoder, for ui.NewVideoPlayer, as mygo's GIF and APNG decoders are.
//
// Frames stay compressed and decode as they play; the canvas is the one
// picture in memory.
package animwebp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"io"
	"time"

	"golang.org/x/image/webp"

	"github.com/egoist/mygo/ui"
)

const (
	maxPixels = 16 << 20 // a canvas larger than this is refused
	maxFrames = 1000
)

// frame is one ANMF chunk: where its picture goes, how long it shows, how it
// meets what is under it and what happens to its place afterwards.
type frame struct {
	x, y, w, h int
	dur        time.Duration
	noBlend    bool // draw over what is there, ignoring it (else alpha-blend)
	dispose    bool // clear its place to transparent before the next frame
	data       []byte
	alpha      bool // its chunks include an ALPH (it needs a VP8X to decode)
}

// Animated reports a WebP file with an animation in it.
func Animated(data []byte) bool {
	if len(data) < 30 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" || string(data[12:16]) != "VP8X" {
		return false
	}
	return data[20]&0x02 != 0 // the animation flag of VP8X
}

// Decoder decodes an animated WebP's frames in order.
type Decoder struct {
	w, h    int
	frames  []frame
	next    int
	canvas  *image.NRGBA
	dispose *image.Rectangle // the place to clear before the next frame
}

var (
	_ ui.VideoDecoder = (*Decoder)(nil)

	// ErrNotAnimated is returned for a file that is not an animated WebP.
	ErrNotAnimated = errors.New("animwebp: not an animated WebP")
	errCorrupt     = errors.New("animwebp: corrupt file")
)

// NewDecoder reads an animated WebP's container (not its pictures: they decode
// as the frames come).
func NewDecoder(data []byte) (*Decoder, error) {
	if !Animated(data) {
		return nil, ErrNotAnimated
	}
	d := &Decoder{}
	rest := data[12:]
	for len(rest) >= 8 {
		fourcc, size := string(rest[:4]), int(binary.LittleEndian.Uint32(rest[4:8]))
		if size < 0 || 8+size > len(rest) {
			break // cut short: what was read is what there is
		}
		payload := rest[8 : 8+size]
		step := 8 + size + size%2 // chunks are padded to an even size
		switch fourcc {
		case "VP8X":
			if len(payload) < 10 {
				return nil, errCorrupt
			}
			d.w, d.h = int24(payload[4:7])+1, int24(payload[7:10])+1
		case "ANMF":
			if len(d.frames) >= maxFrames {
				return nil, errors.New("animwebp: too many frames")
			}
			f, err := parseFrame(payload)
			if err != nil {
				return nil, err
			}
			d.frames = append(d.frames, f)
		}
		if step > len(rest) {
			break
		}
		rest = rest[step:]
	}
	if d.w <= 0 || d.h <= 0 || d.w*d.h > maxPixels {
		return nil, errCorrupt
	}
	if len(d.frames) == 0 {
		return nil, errors.New("animwebp: no frames")
	}
	d.Rewind()
	return d, nil
}

func int24(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }

func parseFrame(p []byte) (frame, error) {
	if len(p) < 16 {
		return frame{}, errCorrupt
	}
	f := frame{
		x: int24(p[0:3]) * 2, y: int24(p[3:6]) * 2,
		w: int24(p[6:9]) + 1, h: int24(p[9:12]) + 1,
		dur:     time.Duration(int24(p[12:15])) * time.Millisecond,
		dispose: p[15]&0x01 != 0, noBlend: p[15]&0x02 != 0,
		data: p[16:],
	}
	for sub := f.data; len(sub) >= 8; {
		size := int(binary.LittleEndian.Uint32(sub[4:8]))
		if string(sub[:4]) == "ALPH" {
			f.alpha = true
		}
		step := 8 + size + size%2
		if size < 0 || step > len(sub) {
			break
		}
		sub = sub[step:]
	}
	return f, nil
}

// Size is the canvas: every frame is this big.
func (d *Decoder) Size() (w, h int) { return d.w, d.h }

// Frames is how many frames the animation has.
func (d *Decoder) Frames() int { return len(d.frames) }

// Rewind goes back to before the first frame.
func (d *Decoder) Rewind() error {
	d.next, d.dispose = 0, nil
	d.canvas = image.NewNRGBA(image.Rect(0, 0, d.w, d.h))
	return nil
}

// Next decodes the next frame onto the canvas and returns the canvas.
func (d *Decoder) Next() (ui.VideoFrame, error) {
	if d.next >= len(d.frames) {
		return ui.VideoFrame{}, io.EOF
	}
	f := d.frames[d.next]
	if d.dispose != nil { // the previous frame's place goes clear
		draw.Draw(d.canvas, *d.dispose, image.Transparent, image.Point{}, draw.Src)
		d.dispose = nil
	}
	pic, err := decodeFrame(f)
	if err != nil {
		return ui.VideoFrame{}, err
	}
	at := image.Rect(f.x, f.y, f.x+f.w, f.y+f.h).Intersect(d.canvas.Bounds())
	op := draw.Over
	if f.noBlend {
		op = draw.Src
	}
	draw.Draw(d.canvas, at, pic, pic.Bounds().Min.Add(at.Min.Sub(image.Pt(f.x, f.y))), op)
	if f.dispose {
		r := at
		d.dispose = &r
	}
	d.next++
	return ui.VideoFrame{Image: d.canvas, Duration: f.dur}, nil
}

// FirstFrame is the picture the animation starts with, as its own image (for a
// still, a thumbnail, or a player paused).
func FirstFrame(data []byte) (image.Image, error) {
	d, err := NewDecoder(data)
	if err != nil {
		return nil, err
	}
	fr, err := d.Next()
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(fr.Image.Bounds())
	draw.Draw(out, out.Bounds(), fr.Image, fr.Image.Bounds().Min, draw.Src)
	return out, nil
}

// decodeFrame has x/image decode a frame's picture: its chunks are a WebP file
// of their own, which needs a VP8X header (with the alpha flag) when it has an
// ALPH chunk.
func decodeFrame(f frame) (image.Image, error) {
	var b bytes.Buffer
	b.WriteString("RIFF")
	b.Write([]byte{0, 0, 0, 0}) // the size, filled below
	b.WriteString("WEBP")
	if f.alpha {
		b.WriteString("VP8X")
		_ = binary.Write(&b, binary.LittleEndian, uint32(10))
		b.Write([]byte{0x10, 0, 0, 0}) // flags: alpha; reserved
		put24(&b, f.w-1)
		put24(&b, f.h-1)
	}
	b.Write(f.data)
	out := b.Bytes()
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return webp.Decode(bytes.NewReader(out))
}

func put24(b *bytes.Buffer, v int) { b.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16)}) }

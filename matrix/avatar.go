package matrix

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// avatarSide is the picture's size as uploaded: a profile picture is shown
// at most a few hundred pixels across, and a smaller file is faster
// everywhere it is loaded.
const avatarSide = 512

// prepareAvatar turns a picture the user chose into a profile picture: the
// middle square of it, no larger than avatarSide, as a PNG when it has
// transparency and a JPEG when it does not. It returns the bytes and their
// media type.
func prepareAvatar(data []byte) ([]byte, string, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", errors.New("that file is not a picture this app can read")
	}
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	if side <= 0 {
		return nil, "", errors.New("the picture is empty")
	}
	// The middle square.
	min0 := image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2)
	crop := image.Rectangle{Min: min0, Max: min0.Add(image.Pt(side, side))}
	out := min(side, avatarSide)
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, xdraw.Src, nil)

	var buf bytes.Buffer
	if hasAlpha(dst) {
		if err := png.Encode(&buf, dst); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	}
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

// hasAlpha says some pixel is not fully opaque.
func hasAlpha(img *image.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y).A != 255 {
				return true
			}
		}
	}
	return false
}

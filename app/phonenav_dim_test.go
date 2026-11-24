package app

import (
	"image"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// lum is the mean brightness (0–255) of a band of the image: a column of
// pixels x0..x1, every row.
func lum(img *image.RGBA, x0, x1 int) float64 {
	b := img.Bounds()
	var sum, n float64
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := x0; x < x1 && x < b.Max.X; x++ {
			o := img.PixOffset(x, y)
			sum += (0.2126*float64(img.Pix[o]) + 0.7152*float64(img.Pix[o+1]) + 0.0722*float64(img.Pix[o+2]))
			n++
		}
	}
	return sum / n
}

// A diagnostic, run with SNAP_DIR set: the brightness of three bands of the
// screen (and PNGs) as a chat slides out. mygo's router darkens the page
// beneath (6% black) and gives the page on top a soft shadow; in the dark
// theme the band just ahead of the sliding chat dips below both pages'
// brightness for a moment, which reads as a fade. Those two overlays are the
// router's own (ui/router.go), with no setting.
func TestPhoneRoomDoesNotDimAsItLeaves(t *testing.T) {
	if os.Getenv("SNAP_DIR") == "" {
		t.Skip("a diagnostic: set SNAP_DIR to write the frames")
	}
	a := testApp()
	a.theme = 2 // dark
	a.pushed = false
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	a.open("!dm-alex")
	settle(tt)
	img := tt.Image()
	w := img.Bounds().Dx()
	rest := lum(img, w/2, w/2+8) // the chat's middle, at rest
	t.Logf("at rest: %.2f", rest)

	a.pushed = false // Back
	tt.Frame()
	for i := 0; i < 14; i++ {
		tt.Advance(30 * time.Millisecond)
		tt.Frame()
		im := tt.Image()
		if i%2 == 1 {
			snap(t, "pop"+string(rune('a'+i/2)), tt)
		}
		t.Logf("pop +%3dms: left %.2f  mid %.2f  right %.2f", (i+1)*30,
			lum(im, 4, 12), lum(im, w/2, w/2+8), lum(im, w-12, w-4))
	}
}

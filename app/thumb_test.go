package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// A picture waiting in the composer shows as itself — a thumbnail tile —
// not as a chip with its name; other files stay chips.
func TestComposerShowsPictureThumbnail(t *testing.T) {
	a, _, room := actApp(t)
	posted := make(chan func(), 4)
	a.runUI = func(f func()) { posted <- f }

	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 30, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	a.addFile(room.ID, model.File{Name: "red.png", Data: buf.Bytes()})
	a.addFile(room.ID, model.File{Name: "notes.pdf", Data: make([]byte, 2000)})

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	// The decode finishes off the UI thread and lands through runUI.
	select {
	case f := <-posted:
		f()
	case <-time.After(5 * time.Second):
		t.Fatal("the thumbnail was never decoded")
	}
	settle(tt)
	snap(t, "composer-thumbnail", tt)

	tile, ok := tt.Find("red.png")
	if !ok {
		t.Fatal("no tile for the picture")
	}
	if tile.W < 60 || tile.W > 90 || tile.H < 60 || tile.H > 90 {
		t.Fatalf("the tile is %vx%v, want a thumbnail", tile.W, tile.H)
	}
	px := tt.Image().RGBAAt(int(tile.X+tile.W/2), int(tile.Y+tile.H/2))
	if px.R < 150 || px.G > 90 || px.B > 90 {
		t.Fatalf("the tile's middle is %v, not the picture's red", px)
	}
	if !tt.HasText("notes.pdf") {
		t.Fatal("a file that is no picture lost its chip")
	}
}

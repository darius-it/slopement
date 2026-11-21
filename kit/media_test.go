package kit

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// gifPlayer is a tiny two-frame GIF's player.
func gifPlayer(t *testing.T) *ui.VideoPlayer {
	t.Helper()
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
		img.SetColorIndex(i, i, 1)
		g.Image = append(g.Image, img)
		// 1 s per frame: still playing (not ended) within the test's time.
		g.Delay = append(g.Delay, 100)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	d, err := ui.NewGIFDecoder(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	p, err := ui.NewVideoPlayer(d)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMediaStarHover pins the star's visibility: pointer hover only, never
// persistent because the GIF is starred — and never kept by the focus a
// click leaves behind.
func TestMediaStarHover(t *testing.T) {
	m := MediaSpec{ID: "g", Kind: MediaGIF, W: 100, H: 80, Label: "Demo GIF", Starred: true,
		Star: func(on bool) {}}
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Padding(10).Children(func() { MediaPreview(c, m, 200, 160, nil) })
	}
	tt := ui.NewTester(view, 300, 220)
	tt.Frame()
	if tt.HasText("Remove from favorite GIFs") {
		t.Fatal("the star shows without hover")
	}
	var tile ui.Rect
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "Demo GIF" {
			tile = a.Bounds
		}
	}
	tt.Move(tile.X+tile.W/2, tile.Y+tile.H/2)
	tt.Frame()
	if !tt.HasText("Remove from favorite GIFs") {
		t.Fatal("the star does not show on hover")
	}
	tt.Move(4, 4)
	tt.Frame()
	if tt.HasText("Remove from favorite GIFs") {
		t.Fatal("the star stayed after the pointer left")
	}
}

// TestGIFPaused is the "Animate GIFs" setting: a Paused GIF shows its
// first frame and never moves on; without Paused it plays.
func TestGIFPaused(t *testing.T) {
	for _, tc := range []struct {
		name   string
		paused bool
	}{
		{"plays", false},
		{"paused", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := gifPlayer(t)
			m := MediaSpec{ID: "g", Kind: MediaGIF, W: 40, H: 30, Player: p, Paused: tc.paused}
			view := func(c *ui.Context) {
				Apply(c)
				ui.Column(c).Padding(10).Children(func() { MediaPreview(c, m, 120, 90, nil) })
			}
			tt := ui.NewTester(view, 200, 120)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			if p.Playing() != !tc.paused {
				t.Fatalf("playing = %v, want %v", p.Playing(), !tc.paused)
			}
			if tc.paused && (p.Position() != 0 || p.Ended()) {
				t.Fatalf("a paused GIF moved: %v, ended %v", p.Position(), p.Ended())
			}
		})
	}
}

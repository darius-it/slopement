package kit

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"

	"github.com/egoist/mygo/ui"
)

// solidPlayer is a two-frame GIF of one colour, as a player.
func solidPlayer(t *testing.T, col color.RGBA) *ui.VideoPlayer {
	t.Helper()
	pal := color.Palette{col, color.RGBA{0, 0, 0, 255}}
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
		g.Image = append(g.Image, img)
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

func countColor(img image.Image, want color.RGBA) int {
	n := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if absInt(int(r>>8)-int(want.R)) < 24 && absInt(int(g>>8)-int(want.G)) < 24 && absInt(int(b>>8)-int(want.B)) < 24 {
				n++
			}
		}
	}
	return n
}

// A GIF picture shows its still until it is to play: always (the setting, the
// profile card) or while the pointer is on it. It asks for its animation only
// when it would play, and goes back to the still when it need not.
func TestAvatarAnimatesWhereAsked(t *testing.T) {
	red, blue := color.RGBA{220, 30, 30, 255}, color.RGBA{30, 60, 220, 255}
	still := bitmapOf(t, blue)
	anim := solidPlayer(t, red)
	var wanted int
	spec := AvatarSpec{Name: "Kai", ID: "@kai:example.org", Size: 64, Image: still, Anim: anim,
		WantAnim: func() { wanted++ }}
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Padding(20).Children(func() { Avatar(c, spec) })
	}
	frame := func(tt *ui.Tester) image.Image {
		for i := 0; i < 3; i++ {
			tt.Frame()
		}
		return tt.Image()
	}

	// Idle: the still, and no request for the file.
	tt := ui.NewTester(view, 200, 140)
	img := frame(tt)
	if countColor(img, blue) < 200 || countColor(img, red) != 0 || wanted != 0 {
		t.Fatalf("idle: blue %d red %d, asked %d times", countColor(img, blue), countColor(img, red), wanted)
	}

	// The pointer on it: it plays (and is asked for).
	tt.Move(52, 52)
	img = frame(tt)
	if countColor(img, red) < 200 || wanted == 0 {
		t.Fatalf("hovered: red %d, blue %d, asked %d", countColor(img, red), countColor(img, blue), wanted)
	}

	// The pointer gone: the still again.
	tt.Move(190, 130)
	img = frame(tt)
	if countColor(img, blue) < 200 || countColor(img, red) != 0 {
		t.Fatalf("after hover: blue %d red %d", countColor(img, blue), countColor(img, red))
	}

	// Always (the setting, a profile card): plays with no pointer.
	spec.Animate = true
	tt2 := ui.NewTester(view, 200, 140)
	img = frame(tt2)
	if countColor(img, red) < 200 || countColor(img, blue) != 0 {
		t.Fatalf("animate: red %d blue %d", countColor(img, red), countColor(img, blue))
	}

	// Not yet fetched: the still, and the request made.
	wanted = 0
	spec.Anim = nil
	tt3 := ui.NewTester(view, 200, 140)
	img = frame(tt3)
	if countColor(img, blue) < 200 || wanted == 0 {
		t.Fatalf("not fetched: blue %d, asked %d", countColor(img, blue), wanted)
	}

	// A picture that is no GIF (nothing to ask) is just its still, hovered or not.
	spec.WantAnim, spec.Animate = nil, false
	tt4 := ui.NewTester(view, 200, 140)
	tt4.Move(52, 52)
	if img = frame(tt4); countColor(img, blue) < 200 {
		t.Fatal("a still lost its picture")
	}
}

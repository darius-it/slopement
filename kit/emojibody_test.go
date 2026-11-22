package kit

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/format"
)

// A custom emoji in a message is drawn as its picture, in line with the
// words, one picture per emoji; a picture that has not arrived leaves the
// room for it, empty.
func TestCustomEmojiInline(t *testing.T) {
	var asked []string
	red := bitmapOf(t, color.RGBA{220, 30, 30, 255})
	blue := bitmapOf(t, color.RGBA{30, 60, 220, 255})
	EmojiImage = func(mxc string) ui.ImageSource {
		asked = append(asked, mxc)
		switch mxc {
		case "mxc://x/red":
			return red
		case "mxc://x/blue":
			return blue
		}
		return nil // not arrived
	}
	defer func() { EmojiImage = nil }()
	doc := format.ParseHTML(`<p>before <img data-mx-emoticon src="mxc://x/red" alt=":red:"> mid ` +
		`<img data-mx-emoticon src="mxc://x/blue" alt=":blue:"> and <img data-mx-emoticon src="mxc://x/late" alt=":late:"> after</p>`)
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Padding(10).Width(420).Children(func() {
			MessageBody(c, "m1", doc, nil, BodyStyle{})
		})
	}
	tt := ui.NewTester(view, 460, 80)
	tt.Frame()
	tt.Frame()
	seen := map[string]bool{}
	for _, m := range asked {
		seen[m] = true
	}
	if !seen["mxc://x/red"] || !seen["mxc://x/blue"] || !seen["mxc://x/late"] {
		t.Fatalf("pictures asked for: %v (each emoji is drawn by its own url)", seen)
	}
	img := tt.Image()
	count := func(want color.RGBA) int {
		n := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if absInt(int(r>>8)-int(want.R)) < 20 && absInt(int(g>>8)-int(want.G)) < 20 && absInt(int(b>>8)-int(want.B)) < 20 {
					n++
				}
			}
		}
		return n
	}
	if r, b := count(color.RGBA{220, 30, 30, 255}), count(color.RGBA{30, 60, 220, 255}); r < 50 || b < 50 {
		t.Fatalf("red pixels %d, blue pixels %d: the pictures are not in the text", r, b)
	}
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		if f, err := os.Create(filepath.Join(dir, "custom-emoji.png")); err == nil {
			_ = png.Encode(f, img)
			f.Close()
		}
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// bitmapOf is a solid 16×16 picture.
func bitmapOf(t *testing.T, col color.RGBA) *ui.Bitmap {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			im.Set(x, y, col)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	bmp, err := ui.DecodeBitmapScaled(b.Bytes(), 64)
	if err != nil {
		t.Fatal(err)
	}
	return bmp
}

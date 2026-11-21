package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/format"
)

// TestSpoilerDust reveals a spoiler: the dust plays, then is forgotten.
// With SNAP_DIR it writes frames of it.
func TestSpoilerDust(t *testing.T) {
	rv := Reveals{}
	doc := format.Parse("the ending is ||the rail grows a pip|| really")
	view := func(c *ui.Context) {
		Apply(c)
		c.SetZoom(3)
		ui.Column(c).Padding(10).Children(func() { MessageBody(c, "m1", doc, rv, BodyStyle{}) })
	}
	tt := ui.NewTester(view, 1200, 200)
	tt.Frame()
	if err := tt.Click("Spoiler"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !rv["m1#0:1"] {
		t.Fatalf("not revealed: %v", rv)
	}
	if len(dusts) == 0 {
		t.Fatal("no dust started")
	}
	dir := os.Getenv("SNAP_DIR")
	for _, at := range []time.Duration{0, 120 * time.Millisecond, 300 * time.Millisecond} {
		tt.Advance(at)
		tt.Frame()
		if dir != "" {
			f, _ := os.Create(filepath.Join(dir, "dust-"+at.String()+".png"))
			png.Encode(f, tt.Image())
			f.Close()
		}
	}
	tt.Advance(time.Second)
	tt.Frame()
	tt.Frame()
	if len(dusts) != 0 {
		t.Fatal("dust kept after it settled")
	}
}

package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTabsSlide clicks the third tab and checks the indicator's journey:
// on its way at 90 ms, arrived by 600 ms. With SNAP_DIR it writes frames.
func TestTabsSlide(t *testing.T) {
	sel := 0
	view := func(c *ui.Context) {
		Apply(c)
		c.SetZoom(3)
		ui.Column(c).Padding(8).Children(func() { Tabs(c, &sel, PickerTabs...).Width(300) })
	}
	tt := ui.NewTester(view, 1000, 140)
	tt.Frame()
	if err := tt.Click("Emoji"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if sel != TabEmoji {
		t.Fatalf("selected = %d", sel)
	}
	dir := os.Getenv("SNAP_DIR")
	for _, at := range []time.Duration{90 * time.Millisecond, 600 * time.Millisecond} {
		tt.Advance(at)
		tt.Frame()
		if dir != "" {
			f, _ := os.Create(filepath.Join(dir, "tabs-"+at.String()+".png"))
			png.Encode(f, tt.Image())
			f.Close()
		}
		at = 0
	}
	if tt.FramePending() {
		t.Error("the indicator is still moving after 690 ms")
	}
}

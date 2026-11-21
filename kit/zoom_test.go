package kit

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestZoomedDetails renders small parts at 4× for checking alignment by
// eye: SNAP_DIR=… go test -run Zoomed ./kit/
func TestZoomedDetails(t *testing.T) {
	dir := os.Getenv("SNAP_DIR")
	if dir == "" {
		t.Skip("SNAP_DIR not set")
	}
	for _, dark := range []bool{false, true} {
		view := func(c *ui.Context) {
			Apply(c)
			c.SetZoom(3)
			ui.Column(c).Padding(8).Gap(8).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					Count(c, 1, true)
					Count(c, 4, false)
					Count(c, 42, true)
					Count(c, 1200, false)
					Kbd(c, "⌘K")
					Pill(c, "Admin")
				})
				Message(c, MessageData{ID: "m", Sender: "Alex", SenderID: "@alex:x", Time: "14:00", First: true,
					Reply: &Quote{Sender: "Mira", SenderID: "@mira:x", Body: "Pushed the new rail spacing. The indicator now grows…"},
					Body:  "Looks great."}, nil)
			})
		}
		tt := ui.NewTesterWith(view, 1600, 560, ui.TesterOptions{Dark: dark})
		tt.Frame()
		name := "zoom-light.png"
		if dark {
			name = "zoom-dark.png"
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
}

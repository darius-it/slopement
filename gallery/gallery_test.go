package gallery

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func allPages() []string {
	var out []string
	for _, g := range groups {
		out = append(out, g.pages...)
	}
	return out
}

// TestPagesRender builds every page at a desktop and a phone width, light
// and dark, for a few frames: no panics, no duplicate keys. With
// SNAP_DIR set, it also writes a PNG of each, to look at.
func TestPagesRender(t *testing.T) {
	snap := os.Getenv("SNAP_DIR")
	for _, size := range []struct {
		name string
		w, h int
	}{{"wide", 1440, 1000}, {"phone", 390, 844}} {
		for _, dark := range []bool{false, true} {
			for _, page := range allPages() {
				a := New()
				a.page = page
				a.themeMode = 1 // the system's: the tester's appearance
				tt := ui.NewTesterWith(a.View, size.w, size.h, ui.TesterOptions{Dark: dark})
				for i := 0; i < 3; i++ {
					tt.Frame()
				}
				if snap != "" {
					mode := "light"
					if dark {
						mode = "dark"
					}
					name := strings.NewReplacer(" ", "-", "&", "and").Replace(strings.ToLower(page))
					writePNG(t, filepath.Join(snap, size.name+"-"+mode+"-"+name+".png"), tt)
				}
			}
		}
	}
}

func writePNG(t *testing.T, path string, tt *ui.Tester) {
	f, err := os.Create(path)
	if err != nil {
		t.Logf("snapshot: %v", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Logf("snapshot: %v", err)
	}
}

// TestSwitcherKeyboard opens the quick switcher with Cmd+K, types, and
// opens the best match with Enter.
func TestSwitcherKeyboard(t *testing.T) {
	a := New()
	tt := ui.NewTester(a.View, 1280, 900)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if !a.switcher.Open {
		t.Fatal("Cmd+K did not open the switcher")
	}
	tt.Type("alex")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.switcher.Open {
		t.Fatal("Enter did not close the switcher")
	}
	if a.room != "!r2" {
		t.Fatalf("room = %q, want Alex Rivera's !r2", a.room)
	}
}

// TestMuteFromMenu mutes a room for two hours from its menu.
func TestMuteFromMenu(t *testing.T) {
	a := New()
	a.page = "Menus & sheets"
	a.themeMode = 1
	tt := ui.NewTester(a.View, 1280, 900)
	tt.Frame()
	if err := tt.Click("design-crit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("Mute", "For 2 hours"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	st := a.roomStates["!r1"]
	if !st.Muted || st.MutedUntil.Sub(a.now) < 119*60e9 {
		t.Fatalf("state = %+v", st)
	}
}

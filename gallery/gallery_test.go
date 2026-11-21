package gallery

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/picker"
)

// testApp is the gallery with a picker that has no KLIPY key: no network.
func testApp() *App {
	return NewWith(picker.New(nil))
}

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
				a := testApp()
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

// TestConfirm opens the leave confirmation, desktop and phone, and
// cancels it with Escape.
func TestConfirm(t *testing.T) {
	snap := os.Getenv("SNAP_DIR")
	for _, size := range []struct {
		name string
		w, h int
	}{{"wide", 1100, 700}, {"phone", 390, 844}} {
		for _, dark := range []bool{false, true} {
			a := testApp()
			a.page, a.themeMode = "Menus & sheets", 1
			a.dialog.Open = true
			tt := ui.NewTesterWith(a.View, size.w, size.h, ui.TesterOptions{Dark: dark})
			tt.Frame()
			tt.Advance(time.Second)
			tt.Frame()
			if !tt.HasText("Leave design-crit?") {
				t.Fatalf("%s: the confirmation is not showing", size.name)
			}
			if snap != "" {
				mode := map[bool]string{false: "light", true: "dark"}[dark]
				writePNG(t, filepath.Join(snap, "confirm-"+size.name+"-"+mode+".png"), tt)
			}
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			tt.Frame()
			if a.dialog.Open || a.did != "Stayed" {
				t.Fatalf("%s: after Escape open=%v did=%q", size.name, a.dialog.Open, a.did)
			}
		}
	}
}

// TestTallPages renders long pages whole, for review (SNAP_DIR).
func TestTallPages(t *testing.T) {
	snap := os.Getenv("SNAP_DIR")
	if snap == "" {
		t.Skip("SNAP_DIR not set")
	}
	for _, dark := range []bool{false, true} {
		a := testApp()
		a.page, a.themeMode = "Media & embeds", 1
		tt := ui.NewTesterWith(a.View, 1440, 2300, ui.TesterOptions{Dark: dark})
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		mode := map[bool]string{false: "light", true: "dark"}[dark]
		writePNG(t, filepath.Join(snap, "tall-media-"+mode+".png"), tt)
	}
	// Polls and the Desktop, animations settled.
	for _, pg := range []string{"Polls", "Desktop", "Phone", "Status & profile", "Navigation: overview", "A · Chats & Communities", "B · Chats on the rail", "C · One inbox"} {
		a := testApp()
		a.page, a.themeMode = pg, 1
		tt := ui.NewTesterWith(a.View, 1440, 1500, ui.TesterOptions{})
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		name := strings.NewReplacer(" ", "-", "&", "and", "·", "", ":", "").Replace(strings.ToLower(pg))
		writePNG(t, filepath.Join(snap, "settled-"+name+".png"), tt)
	}
	// Verification in "Both", light beside dark.
	a := testApp()
	a.page, a.themeMode = "Verification", 0
	tt := ui.NewTesterWith(a.View, 1600, 1100, ui.TesterOptions{Dark: true})
	tt.Frame()
	writePNG(t, filepath.Join(snap, "verify-both.png"), tt)
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
	a := testApp()
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
	a := testApp()
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

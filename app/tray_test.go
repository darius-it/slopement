package app

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// A big icon is scaled to what a tray shows.
func TestTrayIconIsScaledDown(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			big.Set(x, y, color.RGBA{120, 80, 200, 255})
		}
	}
	var src bytes.Buffer
	if err := png.Encode(&src, big); err != nil {
		t.Fatal(err)
	}
	out := trayPNG(src.Bytes())
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("the tray icon is %dx%d, want 64x64", b.Dx(), b.Dy())
	}
	if len(out) >= src.Len() {
		t.Fatalf("not smaller: %d bytes from %d", len(out), src.Len())
	}
	if got := trayPNG([]byte("not a picture")); string(got) != "not a picture" {
		t.Fatal("a bad icon was not handed on as it was")
	}
}

// Minimize to tray is off by default, is a switch in Settings → Advanced, and is
// kept with the other settings.
func TestMinimizeToTraySetting(t *testing.T) {
	a := testApp()
	if defaultPrefs().minimizeToTray {
		t.Fatal("it is on by default")
	}
	tt := ui.NewTester(a.View, 1100, 1200)
	a.view, a.settings = viewSettings, "Advanced"
	settle(tt)
	if !tt.HasText("Minimize to tray") {
		t.Fatal("no switch for it in Settings → Advanced")
	}
	a.prefs.minimizeToTray = true
	if !a.prefs.state().prefs().minimizeToTray {
		t.Fatal("the setting was not kept")
	}
}

// Where no tray can be made (Linux without the library), the setting turns
// itself off with a word about why; nothing is hidden, and with the setting off
// no tray is made.
func TestNoTrayMeansNoHiding(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldIcon, oldMaker, oldFlag := TrayIcon, trayMaker, trayWithoutWindow
	t.Cleanup(func() { TrayIcon, trayMaker, trayWithoutWindow = oldIcon, oldMaker, oldFlag })
	TrayIcon, trayWithoutWindow = []byte("icon"), true
	made := 0
	trayMaker = func(*App) (*mygo.Tray, error) { made++; return nil, errors.New("no libayatana-appindicator3") }

	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.traySync()
	if made != 0 {
		t.Fatal("a tray was made with the setting off")
	}
	a.prefs.minimizeToTray = true
	a.traySync()
	if made != 1 || a.prefs.minimizeToTray || a.tray != nil {
		t.Fatalf("made %d, setting %v, tray %v: it should have turned itself off", made, a.prefs.minimizeToTray, a.tray)
	}
	if a.toast == "" {
		t.Fatal("no word about why")
	}
	a.minimizedToTray() // no tray, no window: nothing happens, nothing panics
}

package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo"

	"slopement/kit"
	"slopement/model"
)

func solidPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// The tray icon gets a red dot with white digits in its corner; with nothing
// unread it is the plain icon.
func TestTrayBadgeDrawsADotWithTheNumber(t *testing.T) {
	base := solidPNG(t, 64, 64, color.RGBA{30, 30, 30, 255})
	if got := trayBadgePNG(base, 0); !bytes.Equal(got, base) {
		t.Fatal("an icon was drawn on for zero")
	}
	for _, n := range []int{3, 12, 150} {
		img, err := png.Decode(bytes.NewReader(trayBadgePNG(base, n)))
		if err != nil {
			t.Fatal(err)
		}
		d := kit.DarkPalette.Danger
		red, white, other := 0, 0, 0
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				switch {
				case uint8(r>>8) == d.R && uint8(g>>8) == d.G && uint8(b>>8) == d.B:
					red++
					if x < 24 && y < 24 {
						t.Fatalf("n=%d: the dot reaches the upper left, not just the lower right corner", n)
					}
				case r>>8 > 240 && g>>8 > 240 && b>>8 > 240:
					white++
				default:
					other++
				}
			}
		}
		if red < 200 || white < 20 {
			t.Fatalf("n=%d: %d red and %d white pixels: no dot with digits", n, red, white)
		}
	}
	if got := trayBadgePNG([]byte("junk"), 4); string(got) != "junk" {
		t.Fatal("a bad icon was not handed on as it was")
	}
	if badgeText(7) != "7" || badgeText(100) != "99" {
		t.Fatalf("badge text: %q %q", badgeText(7), badgeText(100))
	}
}

// The total counts the unread of the rooms that notify, not a muted room's.
func TestUnreadTotalSkipsMutedRooms(t *testing.T) {
	a := testApp()
	rooms := a.src.Rooms()
	want := 0
	for _, r := range rooms {
		if !r.Muted {
			want += r.Unread
		}
	}
	if got := a.unreadTotal(); got != want {
		t.Fatalf("total %d, want %d", got, want)
	}
	_ = model.Room{}
}

// Closing the window hides it in the tray only when that is chosen and an icon
// exists to bring it back; a quit is never stopped.
func TestClosingTheWindowWithTheTray(t *testing.T) {
	a := testApp()
	closing := func() bool {
		e := &mygo.CloseEvent{}
		a.windowClosing(e)
		return e.DefaultPrevented()
	}
	if closing() {
		t.Fatal("close was held with no tray")
	}
	if defaultPrefs().closeMode != closeQuit {
		t.Fatal("keeping in the tray is the default")
	}
	a.holdOnClose.Store(true) // "Keep in tray" chosen, and the icon is there
	if !closing() {
		t.Fatal("the window closed although it should stay in the tray")
	}
	a.quitting.Store(true) // ⌘Q, the tray's Quit, the system's
	if closing() {
		t.Fatal("a quit was stopped")
	}
	// Chosen and kept with the other settings.
	a.prefs.closeMode = closeToTray
	if a.prefs.state().prefs().closeMode != closeToTray {
		t.Fatal("the choice was not kept")
	}
}

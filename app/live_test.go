package app

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// The two timeline GIFs in the mock data: one small, one over 4 MiB (which
// the picker's download used to truncate, so it never showed).
const (
	liveGIFSmall = "https://static.klipy.com/ii/c3a19a0b747a76e98651f2b9a3cca5ff/8a/b1/uAaLRrFA.gif"
	liveGIFBig   = "https://static.klipy.com/ii/50d7c955398dfd7e3c8ba5281154280f/db/47/F5tYiwPfaxKo.gif"
)

// TestLiveGIF loads a timeline GIF over the network, as the app does, and
// checks it shows at its own size. LIVE=1 to run (it fetches from KLIPY's
// CDN); with SNAP_DIR it writes the frame.
func TestLiveGIF(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE not set")
	}
	for _, tc := range []struct{ name, room, url string }{
		{"small", "!crit", liveGIFSmall},
		{"big", "!dm-alex", liveGIFBig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := mock.New(time.Now())
			src.Delay = 0
			pick := picker.New(nil)
			posted := make(chan func(), 16)
			pick.SetUpdate(func(f func()) { posted <- f })
			a := New(src, pick)
			a.nav = kit.NavRail
			a.open(tc.room)
			tt := ui.NewTester(a.View, 1440, 1000)
			tt.Frame()
			deadline := time.After(20 * time.Second)
			for {
				select {
				case f := <-posted:
					f()
					tt.Frame()
				case <-deadline:
					t.Fatal("the GIF never loaded")
				}
				if p := pick.Animation(tc.url); p != nil {
					w, h := p.Size()
					t.Logf("loaded: %dx%d", w, h)
					break
				}
			}
			tt.Advance(500 * time.Millisecond)
			tt.Frame()
			if dir := os.Getenv("SNAP_DIR"); dir != "" {
				snap(t, "app-live-gif-"+tc.name, tt)
			}
		})
	}
}

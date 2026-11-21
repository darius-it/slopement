package gallery

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/internal/dotenv"
	"slopement/kit"
	"slopement/picker"
)

// TestLivePicker opens the picker against the real KLIPY API and writes
// what it shows: KLIPY_LIVE=1 SNAP_DIR=… go test -run LivePicker ./gallery/
// (the key from .env).
func TestLivePicker(t *testing.T) {
	dir := os.Getenv("SNAP_DIR")
	if os.Getenv("KLIPY_LIVE") == "" || dir == "" {
		t.Skip("KLIPY_LIVE or SNAP_DIR not set")
	}
	wd, _ := os.Getwd()
	os.Chdir(filepath.Dir(wd)) // .env is at the module's root
	dotenv.Load()
	os.Chdir(wd)

	var mu sync.Mutex
	var queued []func()
	a := NewWith(picker.NewWithKey())
	a.pick.SetUpdate(func(fn func()) { mu.Lock(); queued = append(queued, fn); mu.Unlock() })
	a.page, a.themeMode = "Composer & picker", 1

	for _, tab := range []struct {
		name string
		tab  int
	}{{"gifs", kit.TabGIFs}, {"stickers", kit.TabStickers}, {"emoji", kit.TabEmoji}} {
		for _, dark := range []bool{false, true} {
			a.picker.Open, a.picker.Tab, a.picker.Browse, a.picker.Query = true, tab.tab, "", ""
			tt := ui.NewTesterWith(a.View, 1100, 760, ui.TesterOptions{Dark: dark})
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				mu.Lock()
				fns := queued
				queued = nil
				mu.Unlock()
				for _, f := range fns {
					f()
				}
				tt.Frame()
				time.Sleep(50 * time.Millisecond)
			}
			mode := "light"
			if dark {
				mode = "dark"
			}
			writePNG(t, filepath.Join(dir, "live-picker-"+tab.name+"-"+mode+".png"), tt)
		}
	}
}

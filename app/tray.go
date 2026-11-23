package app

import (
	"bytes"
	"image"
	"image/png"
	"log"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/egoist/mygo"
	"golang.org/x/image/draw"
)

// Minimize to tray (Settings → Advanced → Window), off by default. On, an icon
// is kept in the menu bar or notification area, and minimizing the window hides
// it there instead of leaving it in the Dock or taskbar; a click on the icon
// brings it back, and its menu has Open and Quit. Closing the window still quits.
// The icon only exists while the setting is on; where a tray cannot be made (on
// Linux without libayatana-appindicator3) minimizing is left alone, so the
// window is never hidden with no way back.

// TrayIcon is the app's icon as a PNG, set by main (it embeds the file).
var TrayIcon []byte

// trayPNG scales the icon down to what a tray shows (a 1024-pixel one is only
// slow to hand to the system).
func trayPNG(src []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return src
	}
	const size = 64
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return src
	}
	return out.Bytes()
}

// trayWithoutWindow lets a test run traySync with no window.
var trayWithoutWindow bool

// trayMaker makes the tray; a variable so tests can see what would be made.
var trayMaker = func(a *App) (*mygo.Tray, error) {
	open := func() { a.win.Update(a.openFromTray) }
	items := []*mygo.MenuItem{
		{Label: "Open slopement", Click: func(*mygo.MenuItem, *mygo.Window) { open() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}
	opts := mygo.TrayOptions{ID: "slopement", Icon: trayPNG(TrayIcon), ToolTip: "slopement"}
	if runtime.GOOS == "linux" {
		// A panel's AppIndicator opens a menu; that is what it can do.
		opts.Menu = mygo.NewMenu(items)
	}
	t, err := mygo.NewTray(opts)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "linux" {
		// A click brings the window back; a right click opens the menu.
		t.OnClick(open)
		t.OnRightClick(func() {
			t.SetMenu(mygo.NewMenu(items))
			t.PopUpMenu()
			t.SetMenu(nil)
		})
	}
	return t, nil
}

// traySync keeps the tray as the setting says, once a frame: made when the
// setting is on, taken away when it is off. A tray that cannot be made is said
// once, and the setting turned off again.
func (a *App) traySync() {
	if (a.win == nil && !trayWithoutWindow) || len(TrayIcon) == 0 {
		return
	}
	want := a.prefs.minimizeToTray || a.prefs.closeMode == closeToTray
	switch {
	case want && a.tray == nil:
		t, err := trayMaker(a)
		if err != nil {
			log.Printf("tray: %v", err)
			a.prefs.minimizeToTray = false
			a.prefs.closeMode = closeQuit
			a.toast = "Could not put slopement in the tray: " + trayWhy(err)
			return
		}
		a.tray = t
	case !want && a.tray != nil:
		a.tray.Destroy()
		a.tray = nil
	}
	// What the close handler reads (it runs off the frame): the window is kept
	// only while there is an icon to get it back from.
	a.holdOnClose.Store(a.prefs.closeMode == closeToTray && a.tray != nil)
}

// What closing the window does (Settings → Advanced → Window). The values are
// the positions of the two choices.
const (
	closeQuit   = 0 // the app quits
	closeToTray = 1 // the app keeps running in the tray
)

// windowClosing is the window's close button (or ⌘W, Alt+F4): with "Keep running
// in the tray" the window is hidden instead of closed, and the sync and the
// notifications go on. A quit (⌘Q, the tray's Quit, the system's) is not stopped.
// It runs on the window's own thread.
func (a *App) windowClosing(e *mygo.CloseEvent) {
	if !a.holdOnClose.Load() || a.quitting.Load() {
		return
	}
	e.PreventDefault()
	if a.win != nil {
		a.win.Hide()
	}
}

// quitWatch is the app that hears of a quit (one listener for the process, as
// accounts come and go: it points at the app on screen).
var quitWatch struct {
	once sync.Once
	app  atomic.Pointer[App]
}

// watchQuit has a quit mark the app as quitting, so closing its window is then
// allowed. A quit that something else cancels leaves the mark; the next close
// then closes the window, which is as safe as the default.
func watchQuit(a *App) {
	quitWatch.app.Store(a)
	quitWatch.once.Do(func() {
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
			if cur := quitWatch.app.Load(); cur != nil {
				cur.quitting.Store(true)
			}
		})
	})
}

// trayWhy is a tray's failure in words.
func trayWhy(err error) string {
	if runtime.GOOS == "linux" {
		return "the tray needs libayatana-appindicator3 on Linux (" + err.Error() + ")"
	}
	return err.Error()
}

// minimizedToTray is the window being minimized: with the tray in place it is
// hidden there. UI thread.
func (a *App) minimizedToTray() {
	if a.win == nil || a.tray == nil || !a.prefs.minimizeToTray {
		return
	}
	a.win.Hide()
}

// openFromTray brings the window back from the tray, in front. UI thread.
func (a *App) openFromTray() {
	if a.win == nil {
		return
	}
	a.win.Show()
	a.win.Restore()
	a.win.Focus()
}

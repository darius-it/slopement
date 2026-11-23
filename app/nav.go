package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// Back and forward: where the user has been (a room, People, Settings, a
// community), reached with a mouse's side buttons, ⌘[ and ⌘] on macOS, and
// Alt+Left and Alt+Right elsewhere.

// place is one stop of the history.
type place struct {
	view      view
	room      string // only in a room
	community string
	settings  string // only in Settings: the page
}

// navHistory is the stops, and which one the user is at.
type navHistory struct {
	stops []place
	at    int
}

// navMax bounds the history.
const navMax = 60

// here is where the user is now.
func (a *App) here() place {
	p := place{view: a.view, community: a.community}
	switch a.view {
	case viewRoom:
		p.room = a.room
	case viewSettings:
		p.settings = a.settings
	}
	return p
}

// navTrack records a place the user went to by any means (a click, the
// switcher, a jump). A place reached by Back or Forward is already the
// history's current one, so it adds nothing; one reached after going back
// drops the stops that were ahead, as a browser does. Call it each frame.
func (a *App) navTrack() {
	h := a.here()
	n := &a.trail
	if len(n.stops) == 0 {
		n.stops, n.at = []place{h}, 0
		return
	}
	if n.stops[n.at] == h {
		return
	}
	// A new place that shows a chat — Chats from a space, back from Settings or
	// People, a room — puts the keyboard in that chat's composer (not on phones).
	if h.view == viewRoom && h.room != "" {
		a.focusRoom = h.room
	}
	n.stops = append(n.stops[:n.at+1], h)
	n.at++
	if len(n.stops) > navMax {
		n.stops = n.stops[len(n.stops)-navMax:]
		n.at = len(n.stops) - 1
	}
}

func (a *App) canGoBack() bool    { return a.trail.at > 0 }
func (a *App) canGoForward() bool { return a.trail.at+1 < len(a.trail.stops) }

// navGo moves by one stop (−1 back, +1 forward), past stops that are gone
// (a room that was left).
func (a *App) navGo(by int) {
	n := &a.trail
	for i := n.at + by; i >= 0 && i < len(n.stops); i += by {
		p := n.stops[i]
		if p.view == viewRoom && p.room != "" {
			if _, ok := a.src.Room(p.room); !ok {
				continue
			}
		}
		n.at = i
		a.view, a.community, a.settings = p.view, p.community, p.settings
		if p.view == viewRoom && p.room != "" {
			a.stopTyping(a.room)
			a.room, a.pushed = p.room, true
			a.profileMsg = ""
			a.src.MarkRead(p.room)
			a.focusRoom = p.room
		}
		return
	}
}

// navKeys goes back and forward for the keys that do.
func (a *App) navKeys(c *ui.Context) {
	backKey, fwdKey := ui.Alt, ui.Alt
	left, right := ui.KeyLeft, ui.KeyRight
	if runtime.GOOS == "darwin" {
		backKey, fwdKey = ui.Cmd, ui.Cmd
		left, right = ui.KeyBracketLeft, ui.KeyBracketRight
	}
	if a.canGoBack() && (c.Shortcut(0, ui.KeyBack) || c.Shortcut(backKey, left)) {
		a.navGo(-1)
	}
	if a.canGoForward() && (c.Shortcut(0, ui.KeyForward) || c.Shortcut(fwdKey, right)) {
		a.navGo(1)
	}
}

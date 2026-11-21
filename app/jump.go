package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// flashFor is how long a message that was jumped to stays highlighted.
const flashFor = 1600 * time.Millisecond

// jumpTo asks for the timeline to scroll to an event of a room, when the
// next frame has the list at hand (the click came from inside it).
func (a *App) jumpTo(room, event string) {
	a.jumpRoom, a.jumpID = room, event
	if a.win != nil {
		a.win.Update(func() {}) // a frame, to scroll in
	}
}

// jumpStep carries out a jump asked for: the list scrolls the message to
// the middle of the timeline and the message is highlighted for a moment.
// Call it each frame before building the room's list.
func (a *App) jumpStep(room string, rows []trow, ls *ui.ListState) {
	if a.jumpID == "" || a.jumpRoom != room {
		return
	}
	id := a.jumpID
	a.jumpRoom, a.jumpID = "", ""
	for i := range rows {
		if rows[i].ev != nil && rows[i].ev.ID == id {
			ls.ScrollTo(i, ui.Center)
			a.flash = id
			if a.win != nil {
				win := a.win
				time.AfterFunc(flashFor, func() {
					win.Update(func() {
						if a.flash == id {
							a.flash = ""
						}
					})
				})
			}
			return
		}
	}
	// Not in what the room holds (older than the timeline kept): nothing to
	// scroll to.
}

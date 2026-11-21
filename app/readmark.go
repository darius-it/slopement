package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// readStep marks the open room read as messages come in while the user is
// there: the window focused, the room shown, and the timeline at its end
// (scrolled back through history, they have not seen the new ones). Opening
// a room marks it read (open); this is the rest — without it, a message that
// arrives in the chat being read counts unread until the next time it is
// opened, and the server's own count, which every sync repeats, never goes
// down.
//
// Called each frame by the room's view, after the list's state is known.
func (a *App) readStep(r model.Room, rows []trow, ls *ui.ListState) {
	if r.Unread == 0 && r.Mentions == 0 {
		return
	}
	if a.view != viewRoom || r.ID != a.room || !ls.AtEnd() {
		return
	}
	if a.win != nil && !a.win.IsFocused() {
		return // the user is elsewhere; their return marks it (Attach)
	}
	last := ""
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].ev != nil && !rows[i].ev.Pending {
			last = rows[i].ev.ID
			break
		}
	}
	if last == "" || a.readMarked[r.ID] == last {
		return // that one was already marked: the count is the server's, catching up
	}
	a.readMarked[r.ID] = last
	a.src.MarkRead(r.ID)
}

package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// nearTop is how many rows from the first one the view counts as "at the
// top": the page is asked for before the user runs out of messages.
const nearTop = 6

// nearTopTouch is nearTop for a finger: a fling crosses a screenful of rows
// in well under a second, faster than a page comes from the server, so the
// next page is asked for while the top is still a few screens away. With
// only six rows of margin the fling ran out of rows, hit the top and
// stopped dead, then the page arrived into a list that was no longer moving.
const nearTopTouch = 45

// pageStep asks the source for older messages when the user has scrolled
// near the top of the room's timeline; they arrive above the rows in view,
// which stay put (the list keeps its place by row key). A source that has
// no history to page in, or none left, does nothing. Called each frame by
// the room's view.
func (a *App) pageStep(room string, ls *ui.ListState) {
	h, ok := a.src.(model.History)
	if !ok {
		return
	}
	near := nearTop
	if kit.PhoneOS() {
		near = nearTopTouch
	}
	if first, last := ls.Visible(); last >= first && first < near && h.HasOlder(room) {
		h.LoadOlder(room) // one page at a time: asking again while it loads is a no-op
	}
}

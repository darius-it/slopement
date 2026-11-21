package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// nearTop is how many rows from the first one the view counts as "at the
// top": the page is asked for before the user runs out of messages.
const nearTop = 6

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
	if first, last := ls.Visible(); last >= first && first < nearTop && h.HasOlder(room) {
		h.LoadOlder(room) // one page at a time: asking again while it loads is a no-op
	}
}

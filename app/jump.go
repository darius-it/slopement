package app

import (
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// flashFor is how long a message that was jumped to stays highlighted.
const flashFor = 1600 * time.Millisecond

// jumpSeek is how long a jump to a message older than what is held keeps
// paging history in, looking for it, before it gives up.
const jumpSeek = 90 * time.Second

// jumpTo asks for the timeline to scroll to an event of a room, when the
// next frame has the list at hand (the click came from inside it).
func (a *App) jumpTo(room, event string) {
	a.jumpRoom, a.jumpID, a.jumpSince, a.jumpAt, a.jumpLooking = room, event, time.Now(), time.Time{}, false
	if a.win != nil {
		a.win.Update(func() {}) // a frame, to scroll in
	}
}

// jumpToEvent is jumpTo for an event whose time is known (a pin): the
// seek stops once the history held reaches back past it, rather than
// paging the whole room in for a message that will not show (redacted,
// or a kind the timeline leaves out).
func (a *App) jumpToEvent(room string, ev model.Event) {
	a.jumpTo(room, ev.ID)
	a.jumpAt = ev.Time
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
			if a.jumpLooking {
				debugf("jump %s: found after %s, row %d of %d", id, time.Since(a.jumpSince).Round(time.Millisecond), i, len(rows))
			}
			a.jumpLooking = false
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
	// Not in what the room holds (older than what was loaded): page older
	// history in and look again as each page lands. LoadOlder is a no-op
	// while a page is on its way, so the frames in between (the pointer
	// moving, a hover) ask nothing; the seek is bounded by time, not by
	// frames, which a moving pointer would use up before the first page.
	oldest := time.Time{}
	for i := range rows {
		if rows[i].ev != nil {
			oldest = rows[i].ev.Time
			break
		}
	}
	notFound := func(why string) {
		debugf("jump %s: gave up after %s: %s (%d rows held, oldest %s)", id,
			time.Since(a.jumpSince).Round(time.Millisecond), why, len(rows), oldest.Format(time.DateTime))
		a.jumpLooking = false
		a.toast = "Couldn't find that message in this room's history"
	}
	hist, ok := a.src.(model.History)
	if !ok || !hist.HasOlder(room) {
		notFound("the whole history is held")
		return
	}
	if !a.jumpAt.IsZero() && oldest.Before(a.jumpAt.Add(-time.Minute)) {
		notFound("held back past its time (" + a.jumpAt.Format(time.DateTime) + ")")
		return
	}
	if time.Since(a.jumpSince) > jumpSeek {
		notFound("took too long")
		return
	}
	if !a.jumpLooking {
		a.jumpLooking = true // the timeline shows the seeking pill (seekPill)
		debugf("jump %s: not held (%d rows, oldest %s, target %s): paging back", id,
			len(rows), oldest.Format(time.DateTime), a.jumpAt.Format(time.DateTime))
	}
	a.jumpRoom, a.jumpID = room, id
	if seek, ok := a.src.(model.HistorySeek); ok {
		seek.LoadOlderBy(room, seekPage)
	} else {
		hist.LoadOlder(room)
	}
}

// seekPill shows, over a room's timeline, that a jump is still looking for
// its message in the history; gone once found or given up.
func (a *App) seekPill(c *ui.Context, room string) {
	if a.jumpLooking && a.jumpRoom == room && a.jumpID != "" {
		kit.SeekingPill(c, "Finding message")
	}
}

// seekPage is how many events a jump asks for at once while it looks.
const seekPage = 500

// debugOn: SLOPEMENT_DEBUG=1 logs jumps to stderr.
var debugOn = os.Getenv("SLOPEMENT_DEBUG") != ""

func debugf(format string, args ...any) {
	if debugOn {
		fmt.Fprintf(os.Stderr, "slopement: "+format+"\n", args...)
	}
}

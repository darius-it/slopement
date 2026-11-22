package app

import (
	"fmt"
	"log"
	"runtime/debug"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// safeRow builds one timeline row so that a panic in it — an event of a kind
// or shape nothing here expected — shows as a quiet "Couldn't show this
// message" tile with a View source, not as a broken timeline.
//
// The recover is inside the row's own container: a panic leaves the builder
// pointing at whatever was open, and the container's Children puts it back
// when the function returns.
func (a *App) safeRow(c *ui.Context, tr trow, build func()) {
	ui.Column(c.Key("safe:" + tr.key)).Children(func() {
		defer func() {
			if r := recover(); r != nil {
				id := ""
				if tr.ev != nil {
					id = tr.ev.ID
				}
				why := fmt.Sprint(r)
				a.rowPanics = append(a.rowPanics, why)
				log.Printf("timeline: row %s panicked: %s\n%s", id, why, debug.Stack())
				a.brokenRow(c, tr)
			}
		}()
		if a.testRowHook != nil && tr.ev != nil {
			a.testRowHook(tr.ev)
		}
		build()
	})
}

// brokenRow is the tile a row that could not be built shows.
func (a *App) brokenRow(c *ui.Context, tr trow) {
	if tr.ev == nil {
		return
	}
	_, canSource := a.src.(interface {
		EventSource(room, id string, done func(original, decrypted, err string))
	})
	if kit.BrokenEvent(c, tr.ev.ID, canSource) {
		a.openSource(tr.ev.Room, tr.ev.ID)
	}
}

package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// A phone's lists and the room pushed over them are the two pages of a
// ui.Router, which gives each system its own way of going back, as native
// as the app can have it without the system's navigation bar:
//
//   - iOS: a swipe from the left edge takes the room off with the finger,
//     the lists beneath it moving less and dimmed, as UIKit's stacks do;
//   - Android: the back key and the system's predictive back gesture pop
//     the room (what is open, a sheet or a menu, takes the back first), and
//     from the lists the back key leaves the app, as every Android app does;
//   - the Back button of the room's header goes the same way, by the same
//     spring (kit.PushSpring).
//
// The app's own state stays the source of what is shown (a.pushed, a.room,
// a.view); the router is kept in step with it each frame, and the other way
// round when the system took the page back.
const (
	pathLists = "/"
	pathRoom  = "/room"
)

// phoneNavSync makes the router show what the app wants, and the app learn
// what a gesture or the back key did. Call it before building the router.
func (a *App) phoneNavSync() {
	if a.phoneNav == nil {
		a.phoneNav = ui.NewRouter(pathLists)
		a.navSeen = pathLists
	}
	r := a.phoneNav
	// Back to the normal slide, after the first frame's push of a room the
	// last run left open (which should not slide in as the app starts).
	r.Transition = ui.TransitionAuto

	// The system took the room back (the edge swipe, the back key): the app
	// follows. Only a change since the last frame counts: the app's own is
	// below.
	if p := r.Path(); p != a.navSeen {
		a.navSeen = p
		switch p {
		case pathLists:
			a.pushed = false
		case pathRoom:
			a.pushed = true // forward in the history
		}
	}

	if _, ok := a.src.Room(a.room); a.pushed && (!ok || a.view != viewRoom) {
		a.pushed = false // no such room (left, or not loaded), or another view is shown
	}
	want := pathLists
	if a.pushed {
		want = pathRoom
	}
	if r.Path() == want {
		return
	}
	switch {
	case want == pathRoom:
		if !a.navStarted {
			r.Transition = ui.TransitionNone // the room of the last run: there at once
		}
		r.Push(pathRoom)
	case r.CanGoBack():
		r.Back()
	default:
		r.Replace(pathLists)
	}
	a.navSeen = r.Path()
	a.navStarted = true
}

// phoneNavView builds the phone's pages.
func (a *App) phoneNavView(c *ui.Context) {
	a.phoneNavSync()
	a.navStarted = true
	a.phoneNav.Animation = &kit.PushSpring
	a.phoneNav.View(c, func(r *ui.Route) {
		if r.Match(pathRoom) {
			a.phoneRoom(c)
			return
		}
		a.phoneLists(c)
	})
}

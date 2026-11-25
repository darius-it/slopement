package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// swipeBackRestarts says mygo's iOS edge swipe loses where the finger let go
// (as of the mygo this builds against): its endEdgeSwipe clears the router's
// "swiping" before going Back, so Back's endSwipe returns early and never
// records the position (Router.resume), and the router's slide out starts from
// the very start, its page (background and shadow) filling the screen again.
// Android's back gesture ends through the back key, which does record it, so it
// is iOS only.
//
// The app works around it: as the swipe commits, the router changes page at
// once (no transition) and the app slides the room out itself, from where the
// finger left it, over the lists (swipeOut). TODO: remove swipeBackRestarts,
// swipeOut, popLeft, swipeGap, noteRoomX/lostByRouter and the fields they use
// once a mygo with the fix is pinned (the fix: commitSwipe records
// r.resume = r.swipe before Back, or endEdgeSwipe commits before clearing
// r.swiping). Not reported upstream yet (AGENTS.md, phone navigation).
var swipeBackRestarts = runtime.GOOS == "ios"

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

// popLeft is how much of a swipe back's slide out is still to go, 1 as it
// begins and 0 as it ends, along kit.PopCurve (the curve of every other pop),
// by the frame's clock.
func (a *App) popLeft(c *ui.Context) float32 {
	if !a.swipeOut {
		return 0
	}
	t := float32(c.Now().Sub(a.popAt)) / float32(kit.PopCurve.Duration)
	if t >= 1 {
		a.swipeOut = false
		return 0
	}
	return 1 - kit.PopCurve.Ease(max(t, 0))
}

// swipeGap is how far in from the window's right edge the room still reaches
// as the app slides it out after a swipe back: 0 when it does not.
func (a *App) swipeGap(c *ui.Context) float32 {
	k := a.popLeft(c)
	if k == 0 {
		return 0
	}
	w, _ := c.Size()
	return max(w-a.popFrom, 0) * k
}

// noteRoomX remembers where the room's page is, from its last two frames, for
// the swipe back that is let go next: the router moves the page by a layout
// inset, so its bounds say where the finger had it.
func (a *App) noteRoomX(x float32) { a.roomXPrev, a.roomX = a.roomX, x }

// lostByRouter is where the room was as the finger let go, as far as the last
// frames say (their bounds are a frame behind): one more step along where it
// was going, within the window.
func (a *App) lostByRouter(width float32) float32 {
	return min(max(a.roomX+(a.roomX-a.roomXPrev), 0), width)
}

// phoneNavSync makes the router show what the app wants, and the app learn
// what a gesture or the back key did. Call it before building the router.
func (a *App) phoneNavSync(c *ui.Context) {
	if a.phoneNav == nil {
		a.phoneNav = ui.NewRouter(pathLists)
		a.navSeen = pathLists
	}
	r := a.phoneNav
	// Back to the normal slide, after the first frame's push of a room the
	// last run left open (which should not slide in as the app starts), or a
	// swipe back the app slides out itself.
	r.Transition = ui.TransitionAuto

	// The system took the room back (the edge swipe, the back key): the app
	// follows. Only a change since the last frame counts: the app's own is
	// below.
	if p := r.Path(); p != a.navSeen {
		a.navSeen = p
		switch p {
		case pathLists:
			a.pushed = false
			if swipeBackRestarts {
				// The router would slide from the start: it shows the lists at
				// once, and the app slides the room out from the finger's place.
				w, _ := c.Size()
				a.popFrom, a.popAt, a.swipeOut = a.lostByRouter(w), c.Now(), true
				r.Transition = ui.TransitionNone
			} else {
				a.navPop = true // the room slides out by PopCurve
			}
		case pathRoom:
			a.pushed = true // forward in the history
			a.swipeOut = false
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
		a.swipeOut = false // a room opened as another slid out: it takes the screen
		r.Push(pathRoom)
	case r.CanGoBack():
		r.Back()
		a.navPop = true
	default:
		r.Replace(pathLists)
		a.navPop = true
	}
	a.navSeen = r.Path()
	a.navStarted = true
}

// phoneNavView builds the phone's pages.
func (a *App) phoneNavView(c *ui.Context) {
	a.phoneNavSync(c)
	a.navStarted = true
	// How the pages move is chosen as the router starts the slide, in the frame
	// it is asked for: a push by a spring, a pop by a curve of fixed length (so
	// it does not stall and creep after a swipe, nor keep both pages being built
	// for a second after it looks over: see kit.PopCurve).
	if a.navPop {
		a.phoneNav.Animation = &kit.PopCurve
	} else {
		a.phoneNav.Animation = &kit.PushSpring
	}
	a.navPop = false
	ui.Column(c).Grow(1).AlignSelf(ui.Stretch).MinWidth(0).MinHeight(0).Clip().Children(func() {
		a.phoneNav.View(c, func(r *ui.Route) {
			if r.Match(pathRoom) {
				a.phoneRoom(c)
				return
			}
			a.phoneLists(c)
		})
		a.swipeOutView(c)
	})
}

// swipeOutView is the room sliding out after a swipe back, drawn by the app
// over the lists (see swipeBackRestarts): it goes on from where the finger let
// it go, with the router's shadow, and takes no input, as a page going away.
// Its scroll and its draft are the app's (listState, the composer's state), so
// it shows as it was.
func (a *App) swipeOutView(c *ui.Context) {
	gap := a.swipeGap(c)
	if gap <= 0 {
		return
	}
	c.AnimationFrame()
	w, _ := c.Size()
	ui.Column(c.Key("swipe-out")).Absolute().Top(0).Fill().Translate(w-gap, 0).
		Shadow(0, 0, 16, 0, kit.PageShadow).IgnoreInput(true).AccessHidden(true).
		Children(func() { a.phoneRoom(c) })
}

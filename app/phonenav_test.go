package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A phone's room is a page of a router, so the system's way back (iOS's edge
// swipe, Android's back key) pops it, and the app follows; the app's own
// Back and a room opened from the lists move the router the other way.
func TestPhoneRoomIsARouterPage(t *testing.T) {
	a := testApp()
	a.pushed = false
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	if a.phoneNav == nil || a.phoneNav.Path() != pathLists {
		t.Fatal("the phone starts on the lists")
	}

	// A room opened by the app pushes the page.
	a.open("!dm-alex")
	settle(tt)
	if a.phoneNav.Path() != pathRoom || !a.pushed {
		t.Fatalf("after opening a room: router at %q, pushed %v", a.phoneNav.Path(), a.pushed)
	}
	if !a.phoneNav.CanGoBack() {
		t.Fatal("a pushed room has nothing to go back to")
	}

	// The system goes back (what the edge swipe and Android's back key do):
	// the app's state follows, with no Back button pressed.
	a.phoneNav.Back()
	settle(tt)
	if a.pushed || a.phoneNav.Path() != pathLists {
		t.Fatalf("after the system's back: router at %q, pushed %v", a.phoneNav.Path(), a.pushed)
	}

	// The room again, then the header's own Back.
	a.open("!dm-alex")
	settle(tt)
	if !a.pushed || a.phoneNav.Path() != pathRoom {
		t.Fatalf("reopened: router at %q, pushed %v", a.phoneNav.Path(), a.pushed)
	}
	a.pushed = false // what the Back button does
	settle(tt)
	if a.phoneNav.Path() != pathLists {
		t.Fatalf("after Back: router at %q", a.phoneNav.Path())
	}
	if a.phoneNav.CanGoBack() {
		t.Fatal("the history grew: going back left entries behind")
	}
}

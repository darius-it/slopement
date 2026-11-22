package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// On a phone the find bar, the pins and a thread have a place too: the find
// bar under the header (from the room's sheet), the pins and threads in a
// sheet where wide windows have a side panel.
func TestPhoneFindPinsAndThread(t *testing.T) {
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)

	a.roomSheet.Open, a.roomSheetID = true, "!dm-alex"
	settle(tt)
	if !tt.HasText("Find in chat") || !tt.HasText("Pinned messages") {
		t.Fatal("the room's sheet lacks Find in chat / Pinned messages")
	}
	if err := tt.Click("Find in chat"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !a.find.Open || a.find.Room != "!dm-alex" || !tt.HasText("Find in this chat") {
		t.Fatalf("find open %v, bar shown %v", a.find.Open, tt.HasText("Find in this chat"))
	}

	a.roomSheet.Open, a.roomSheetID = true, "!dm-alex"
	settle(tt)
	if err := tt.Click("Pinned messages"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.panel != panelPins || !a.panelSheet.Open {
		t.Fatalf("panel %v, sheet open %v", a.panel, a.panelSheet.Open)
	}
	// Closing the sheet clears the panel.
	a.panelSheet.Open = false
	settle(tt)
	settle(tt)
	if a.panel != panelNone {
		t.Fatalf("panel %v after closing", a.panel)
	}
	// A thread opens the same sheet.
	a.panel = panelThread
	settle(tt)
	if !a.panelSheet.Open || !tt.HasText("Thread") {
		t.Fatal("a thread has no sheet on a phone")
	}
}

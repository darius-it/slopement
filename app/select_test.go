package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// One drag selects across messages, as on the web: from the words of one
// message to those of another, with the ones between, and Copy takes them.
func TestDragSelectsAcrossMessages(t *testing.T) {
	a, _, room := actApp(t)
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	from, ok1 := tt.Find("message number 26")
	to, ok2 := tt.Find("message number 28")
	if !ok1 || !ok2 {
		t.Fatal("the messages are not drawn")
	}
	_ = room
	tt.Move(from.X+2, from.Y+from.H/2)
	tt.Frame()
	tt.Press(from.X+2, from.Y+from.H/2)
	tt.Frame()
	tt.Move(to.X+to.W-2, to.Y+to.H/2)
	tt.Frame()
	tt.Release(to.X+to.W-2, to.Y+to.H/2)
	tt.Frame()
	snap(t, "select-across-messages", tt)
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()
	got := tt.Clipboard()
	for _, want := range []string{"message number 26", "message number 27", "message number 28"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the drag copied %q, missing %q", got, want)
		}
	}
}

// A right click on a hyperlink offers to open or copy that link; one on the
// message elsewhere is the message's own menu, whose link item says it is
// the message's.
func TestLinkContextMenu(t *testing.T) {
	a, src, room := actApp(t)
	ev := src.tl[27]
	ev.Body = "see the docs now"
	ev.Formatted = `see <a href="https://example.com/docs">the docs</a> now`
	src.tl[27] = ev
	_ = room
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	l, ok := tt.Find("the docs")
	if !ok {
		t.Fatal("the link is not drawn")
	}
	tt.RightClickAt(l.X+l.W/2, l.Y+l.H/2)
	settle(tt)
	menu := strings.Join(tt.Menu(), "|")
	if !strings.Contains(menu, "Open link") || !strings.Contains(menu, "Copy link") {
		t.Fatalf("the link's menu = %q", menu)
	}
	if strings.Contains(menu, "Reply") {
		t.Fatalf("the message's menu opened over the link's: %q", menu)
	}
	if err := tt.ChooseMenuItem("Copy link"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "https://example.com/docs" {
		t.Fatalf("copied %q", got)
	}

	// Beside the link, on the message: its own menu.
	tt.RightClickAt(l.X+l.W+120, l.Y+l.H/2)
	settle(tt)
	menu = strings.Join(tt.Menu(), "|")
	if !strings.Contains(menu, "Copy message link") || strings.Contains(menu, "Open link") {
		t.Fatalf("the message's menu = %q", menu)
	}
}

// A click on a message leaves nothing stuck: once the pointer leaves, its
// hover gray and toolbar are gone (keyboard focus left in the row by the
// click does not keep them).
func TestClickLeavesNoStuckHighlight(t *testing.T) {
	a, _, _ := actApp(t)
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	msg, ok := tt.Find("message number 28")
	if !ok {
		t.Fatal("the message is not drawn")
	}
	x, y := msg.X+msg.W/2, msg.Y+msg.H/2
	tt.Move(x, y)
	settle(tt)
	if _, ok := tt.Find("Add reaction"); !ok {
		t.Fatal("no toolbar while hovered")
	}
	tt.Press(x, y)
	tt.Frame()
	tt.Release(x, y)
	tt.Frame()
	tt.Move(200, 600) // off the timeline, into the sidebar
	settle(tt)
	settle(tt)
	if _, ok := tt.Find("Add reaction"); ok {
		t.Fatal("the toolbar (and hover gray) stayed after the pointer left")
	}
}

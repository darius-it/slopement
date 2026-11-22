package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// toolbarNames are the accessible names on screen after hovering a message.
func toolbarNames(t *testing.T, st MessageMenuState, shift bool) map[string]bool {
	t.Helper()
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Padding(20).Children(func() {
			Message(c, MessageData{ID: "m", Sender: "Alex", SenderID: "@alex:x", Time: "14:00", First: true,
				Body: "Looks great.", Menu: st}, nil)
		})
	}
	tt := ui.NewTester(view, 900, 300)
	tt.Frame()
	tt.Move(300, 70)
	if shift {
		tt.HoldModifiers(ui.Shift)
	}
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	out := map[string]bool{}
	for _, a := range tt.AccessAll() {
		out[a.Name] = true
	}
	return out
}

// Hovering one's own message shows Edit beside Reply; Delete only with Shift.
// Someone else's message has neither.
func TestToolbarEditAndShiftDelete(t *testing.T) {
	mine := MessageMenuState{Mine: true, HasText: true}
	got := toolbarNames(t, mine, false)
	if !got["Reply"] || !got["Edit"] || got["Delete"] {
		t.Fatalf("hover: %v", got)
	}
	got = toolbarNames(t, mine, true)
	if !got["Edit"] || !got["Delete"] {
		t.Fatalf("shift: %v", got)
	}
	got = toolbarNames(t, MessageMenuState{HasText: true}, true)
	if !got["Reply"] || got["Edit"] || got["Delete"] {
		t.Fatalf("someone else's: %v", got)
	}
	// A message with no words (a picture) can be deleted but not edited.
	got = toolbarNames(t, MessageMenuState{Mine: true}, true)
	if got["Edit"] || !got["Delete"] {
		t.Fatalf("no words: %v", got)
	}
}

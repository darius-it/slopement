package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// actSource records the actions asked of it.
type actSource struct {
	timeline
	edits   [][3]string
	redacts []string
	pins    map[string]bool
}

func (s *actSource) Edit(room, event, text string) { s.edits = append(s.edits, [3]string{room, event, text}) }
func (s *actSource) Redact(room, event string)     { s.redacts = append(s.redacts, event) }
func (s *actSource) Pin(room, event string, on bool) {
	if s.pins == nil {
		s.pins = map[string]bool{}
	}
	if on {
		s.pins[event] = true
	} else {
		delete(s.pins, event)
	}
}
func (s *actSource) IsPinned(room, event string) bool { return s.pins[event] }
func (s *actSource) Pinned(room string) []string {
	var out []string
	for _, e := range s.tl {
		if s.pins[e.ID] {
			out = append(out, e.ID)
		}
	}
	return out
}

func actApp(t *testing.T) (*App, *actSource, model.Room) {
	t.Helper()
	a := testApp()
	room, tl := longTimeline(a, 30)
	src := &actSource{timeline: timeline{a.src, tl}}
	a.src = src
	a.open(room.ID)
	return a, src, room
}

// What the menu of a message offers depends on whose it is and what state it is in.
func TestMessageMenuRules(t *testing.T) {
	a, _, room := actApp(t)
	me := a.src.Me().ID
	text := func(sender string) model.Event {
		return model.Event{ID: "$x", Room: room.ID, Sender: sender, Kind: model.EventText, Body: "words"}
	}
	if st := a.msgMenu(text(me)); !st.Mine || !st.HasText || st.Off {
		t.Fatalf("own text: %+v", st)
	}
	if st := a.msgMenu(text("@kai:example.org")); st.Mine || !st.HasText || st.Off {
		t.Fatalf("someone else's text: %+v", st)
	}
	pending := text(me)
	pending.Pending = true
	if !a.msgMenu(pending).Off {
		t.Fatal("a message still being sent has a menu")
	}
	locked := text("@kai:example.org")
	locked.Kind = model.EventUndecryptable
	if !a.msgMenu(locked).Off {
		t.Fatal("a lock tile has a menu")
	}
	file := model.Event{ID: "$f", Sender: me, Kind: model.EventMedia, Body: "pic.png"}
	if st := a.msgMenu(file); st.HasText {
		t.Fatalf("a picture has words to edit or copy: %+v", st)
	}
}

// Editing puts the message's words in the composer; sending replaces the
// message instead of sending a new one; emptying it, or Escape, stops.
func TestEditFlow(t *testing.T) {
	a, src, room := actApp(t)
	ev := model.Event{ID: "$mine", Room: room.ID, Sender: a.src.Me().ID, Kind: model.EventText, Body: "tyop"}

	a.messageChoice(room.ID, ev, kit.MsgEdit)
	st := a.draft(room.ID)
	if st.Editing != "$mine" || st.Text != "tyop" {
		t.Fatalf("composer = %+v", st)
	}
	st.Text = "typo"
	if !a.editStep(room.ID, st) {
		t.Fatal("the send was not taken as an edit")
	}
	if len(src.edits) != 1 || src.edits[0] != [3]string{room.ID, "$mine", "typo"} {
		t.Fatalf("edits = %v", src.edits)
	}
	if st.Editing != "" || st.Text != "" {
		t.Fatalf("the composer stayed in edit mode: %+v", st)
	}

	a.messageChoice(room.ID, ev, kit.MsgEdit)
	st.Text = "  "
	a.editStep(room.ID, st)
	if len(src.edits) != 1 {
		t.Fatal("an empty edit was sent")
	}
	if a.editStep(room.ID, st) {
		t.Fatal("a plain send was taken as an edit")
	}
}

// Pins: chosen from the menu, shown in a bar under the header, which goes
// to the message and on to the next.
func TestPinBar(t *testing.T) {
	a, src, room := actApp(t)
	tl := src.tl
	a.messageChoice(room.ID, tl[3], kit.MsgPin)
	a.messageChoice(room.ID, tl[20], kit.MsgPin)
	if !src.pins[tl[3].ID] || !src.pins[tl[20].ID] {
		t.Fatalf("pins = %v", src.pins)
	}
	if st := a.msgMenu(tl[3]); !st.Pinned {
		t.Fatal("a pinned message does not offer Unpin")
	}

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	// No bar under the header: pins live in the panel behind the pin button.
	if tt.HasText("Go to pinned message") || tt.HasText("Pinned 1 of 2") {
		t.Fatal("a pin bar is still drawn under the header")
	}
	if len(a.pinnedEvents(room.ID)) != 2 {
		t.Fatalf("pinned events = %d", len(a.pinnedEvents(room.ID)))
	}

	a.messageChoice(room.ID, tl[3], kit.MsgUnpin)
	a.messageChoice(room.ID, tl[20], kit.MsgUnpin)
	if len(a.pinnedEvents(room.ID)) != 0 {
		t.Fatal("still pinned after Unpin")
	}
	_ = time.Second
}

// The pins panel (the pin button beside call and search) lists what is
// pinned, and says so when nothing is.
func TestPinsPanel(t *testing.T) {
	a, _, room := actApp(t)
	tl := a.src.Timeline(room.ID)
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if err := tt.Click("Pinned messages"); err != nil {
		t.Fatalf("no pin button in the header: %v", err)
	}
	settle(tt)
	if !tt.HasText("Nothing pinned yet") {
		t.Fatal("the empty pins panel says nothing")
	}
	a.messageChoice(room.ID, tl[3], kit.MsgPin)
	settle(tt)
	if tt.HasText("Nothing pinned yet") || !tt.HasText("message number 3") {
		t.Fatal("the pinned message is not listed")
	}
}

// The new controls at phone width: a waiting file and the mention popup
// together (a snapshot with SNAP_DIR) — and nothing overflows.
func TestPhoneWithNewControls(t *testing.T) {
	a, _, room := actApp(t)
	a.addFile(room.ID, model.File{Name: "quarterly-report-final-v2.pdf", Data: make([]byte, 340<<10)})
	partner, _ := a.src.Person(room.Partner)
	a.draft(room.ID).Text = "see " + "@" + partner.Name[:2]
	tt := ui.NewTester(a.View, 390, 844)
	a.pushed = true
	settle(tt)
	snap(t, "phone-new-controls", tt)
	if !tt.HasText("quarterly-report-final-v2.pdf") || !tt.HasText(partner.Name) {
		t.Fatal("the phone view is missing the file chip or the suggestion")
	}
}

// A long press on a phone opens the message's menu over the blurred chat, and a
// choice from it acts.
func TestMessageMenuOnPhone(t *testing.T) {
	a, src, room := actApp(t)
	ev := src.tl[len(src.tl)-3] // one in view, as a long press has
	a.messageChoice(room.ID, ev, kit.MsgOpenSheet)
	if !a.msgPop.Open || a.msgID != ev.ID {
		t.Fatalf("the menu did not open: %+v %q", a.msgPop, a.msgID)
	}
	tt := ui.NewTester(a.View, 390, 844)
	settle(tt)
	snap(t, "message-menu", tt)
	if a.msgPop.At.W <= 0 || a.msgPop.At.H <= 0 {
		t.Fatalf("the message's place was not found: %+v", a.msgPop.At)
	}
	if !tt.HasText("Copy message link") || !tt.HasText("Reply") {
		t.Fatal("the menu does not list the actions")
	}
	if err := tt.Click("Pin"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !src.pins[ev.ID] || a.msgPop.Open {
		t.Fatalf("Pin did not pin and close: pins=%v open=%v", src.pins, a.msgPop.Open)
	}
}

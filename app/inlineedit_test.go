package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// On a wide window a message is edited where it is: what was being written in
// the composer stays there, Enter saves, Escape leaves the message as it was.
func TestEditInPlaceKeepsTheDraft(t *testing.T) {
	a, src, room := actApp(t)
	ev := model.Event{ID: "$mine", Room: room.ID, Sender: a.src.Me().ID, Kind: model.EventText, Body: "tyop"}
	src.timeline.tl = append(src.timeline.tl, ev)
	a.draft(room.ID).Text = "half a new message"

	tt := ui.NewTester(a.View, 1100, 700)
	tt.Frame()
	a.messageChoice(room.ID, ev, kit.MsgEdit)
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("Save"); !ok {
		t.Fatal("no edit box in the timeline")
	}
	snap(t, "inline-edit", tt)
	tt.Type("!")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if len(src.edits) != 1 || src.edits[0] != [3]string{room.ID, "$mine", "tyop!"} {
		t.Fatalf("edits = %v", src.edits)
	}
	if got := a.draft(room.ID).Text; got != "half a new message" || a.edit != nil {
		t.Fatalf("after saving: draft %q, editing %v", got, a.edit != nil)
	}

	a.messageChoice(room.ID, ev, kit.MsgEdit)
	tt.Frame()
	tt.Frame()
	tt.Type("?")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if len(src.edits) != 1 || a.edit != nil || a.draft(room.ID).Text != "half a new message" {
		t.Fatalf("after Escape: edits %v, draft %q", src.edits, a.draft(room.ID).Text)
	}
}

// A phone edits in the composer, and has the draft back afterwards; the draft,
// not the message being edited, is what is kept between launches.
func TestPhoneEditSetsTheDraftAside(t *testing.T) {
	a, src, room := actApp(t)
	a.narrow = true
	ev := model.Event{ID: "$mine", Room: room.ID, Sender: a.src.Me().ID, Kind: model.EventText, Body: "tyop"}
	st := a.draft(room.ID)
	st.Text = "half a new message"

	a.messageChoice(room.ID, ev, kit.MsgEdit)
	a.messageChoice(room.ID, ev, kit.MsgEdit) // choosing again keeps the first draft
	if st.Text != "tyop" || a.state().Drafts[room.ID] != "half a new message" {
		t.Fatalf("field %q, kept %q", st.Text, a.state().Drafts[room.ID])
	}
	st.Text = "typo"
	if !a.editStep(room.ID, st) || len(src.edits) != 1 {
		t.Fatalf("edits = %v", src.edits)
	}
	if st.Text != "half a new message" || st.Editing != "" {
		t.Fatalf("after saving: %q, editing %q", st.Text, st.Editing)
	}
}

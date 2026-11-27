package app

import (
	"testing"

	"slopement/model"
)

// ↑ in an empty composer edits the last message of yours that can be edited,
// passing over one still sending, one that failed, and other people's.
func TestEditLastPicksTheLastEditableOfMine(t *testing.T) {
	a, src, room := actApp(t)
	me := a.src.Me().ID
	src.timeline.tl = append(src.timeline.tl,
		model.Event{ID: "$mine-old", Room: room.ID, Sender: me, Kind: model.EventText, Body: "first"},
		model.Event{ID: "$mine-new", Room: room.ID, Sender: me, Kind: model.EventText, Body: "the last one"},
		model.Event{ID: "$theirs", Room: room.ID, Sender: "@kai:example.org", Kind: model.EventText, Body: "not mine"},
		model.Event{ID: "$sending", Room: room.ID, Sender: me, Kind: model.EventText, Body: "on its way", Pending: true},
		model.Event{ID: "$failed", Room: room.ID, Sender: me, Kind: model.EventText, Body: "nope", Failed: true},
	)
	a.editLast(room.ID)
	if a.edit == nil {
		t.Fatal("nothing is being edited")
	}
	st := &a.edit.st
	if st.Editing != "$mine-new" || st.Text != "the last one" {
		t.Fatalf("editing %q with %q", st.Editing, st.Text)
	}
}

package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// ↑ with nothing written asks to edit the last message; with words in the
// field, while already editing, or on a touch composer, it is the field's own.
func TestUpKeyInTheComposer(t *testing.T) {
	st := &ComposerState{}
	if !fieldKey(st, ui.KeyUp, 0, false) || !st.wantEditLast {
		t.Fatal("↑ in an empty field did not ask to edit the last message")
	}
	st = &ComposerState{Text: "typing"}
	if fieldKey(st, ui.KeyUp, 0, false) || st.wantEditLast {
		t.Fatal("↑ was taken while there are words in the field")
	}
	st = &ComposerState{Editing: "$x"}
	if fieldKey(st, ui.KeyUp, 0, false) {
		t.Fatal("↑ was taken while already editing")
	}
	st = &ComposerState{}
	if fieldKey(st, ui.KeyUp, 0, true) {
		t.Fatal("↑ was taken on a touch composer")
	}
	var r ComposerResult
	st = &ComposerState{wantEditLast: true}
	st.takePlus(&r)
	if !r.EditLast || st.wantEditLast {
		t.Fatalf("the request did not reach the result: %+v", r)
	}
}

package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// While suggestions show: Up and Down move the highlight (round), Tab takes
// the highlighted one (so does Enter), Escape puts them away.
func TestSuggestKeys(t *testing.T) {
	st := &ComposerState{Text: "hi @a", Suggest: []SuggestItem{{Insert: "@a1"}, {Insert: "@a2"}, {Insert: "@a3"}}}

	if !suggestKey(st, ui.TextKey{Key: ui.KeyEnter}) || !st.wantPick {
		t.Fatal("Enter did not take the highlighted suggestion")
	}
	st.wantPick = false
	// With a modifier (Shift+Enter breaks the line) the suggestions let go.
	if suggestKey(st, ui.TextKey{Key: ui.KeyEnter, Mods: ui.Shift}) {
		t.Fatal("Shift+Enter was taken by the suggestions")
	}
	for i, want := range []int{1, 2, 0} { // Down, round from the last to the first
		if !suggestKey(st, ui.TextKey{Key: ui.KeyDown}) || st.sel != want {
			t.Fatalf("Down #%d: sel = %d, want %d", i+1, st.sel, want)
		}
	}
	if !suggestKey(st, ui.TextKey{Key: ui.KeyUp}) || st.sel != 2 {
		t.Fatalf("Up from the first: sel = %d, want 2", st.sel)
	}
	if !suggestKey(st, ui.TextKey{Key: ui.KeyTab}) || !st.wantPick {
		t.Fatal("Tab did not take the highlighted suggestion")
	}
	st.wantPick = false
	if !suggestKey(st, ui.TextKey{Key: ui.KeyEscape}) || !st.SuggestionsOff() {
		t.Fatal("Escape did not put the suggestions away")
	}
	st.Text = "hi @al"
	if st.SuggestionsOff() {
		t.Fatal("they stayed away after the text changed")
	}

	// With none showing, the keys are the field's own (Tab moves on, Up and
	// Down move the caret).
	st.Suggest = nil
	for _, k := range []ui.Key{ui.KeyTab, ui.KeyDown, ui.KeyUp, ui.KeyEnter} {
		if suggestKey(st, ui.TextKey{Key: k}) {
			t.Fatalf("key %v taken with no suggestions", k)
		}
	}
}

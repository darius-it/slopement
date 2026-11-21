package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWordAtEnd(t *testing.T) {
	for text, want := range map[string]string{"": "", "hi": "hi", "hi @al": "@al", "a\n:smi": ":smi", "trailing ": ""} {
		if got, _ := wordAtEnd(text); got != want {
			t.Errorf("wordAtEnd(%q) = %q, want %q", text, got, want)
		}
	}
}

// After an @ the people of the room are suggested, by name, most to least
// likely; after a : the emoji; nothing for another word.
func TestSuggestions(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 6)
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	st := a.draft(room.ID)
	partner, _ := a.src.Person(room.Partner)
	first := strings.ToLower(partner.Name[:2])

	st.Text = "hello @" + first
	got := a.suggestions(room.ID, st)
	if len(got) == 0 || got[0].ID != room.Partner || got[0].Insert != "@"+partner.Name {
		t.Fatalf("mention suggestions = %+v", got)
	}
	for _, it := range got {
		if it.ID == a.src.Me().ID {
			t.Fatal("the user is suggested to themselves")
		}
	}

	st.Text = "so :smi"
	em := a.suggestions(room.ID, st)
	if len(em) == 0 || em[0].Emoji == "" || !strings.HasPrefix(em[0].Sub, ":") {
		t.Fatalf("emoji suggestions = %+v", em)
	}
	for _, text := range []string{"plain words", "an email@example.org", ":s", "ratio 1:2"} {
		st.Text = text
		if got := a.suggestions(room.ID, st); len(got) != 0 {
			t.Errorf("%q suggests %+v", text, got)
		}
	}
}

// A picked mention replaces the word, and becomes a link to the person in
// what is sent; an emoji replaces the word as it is.
func TestApplySuggestionAndMentionLinks(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 6)
	a.src = timeline{a.src, tl}
	st := a.draft(room.ID)
	partner, _ := a.src.Person(room.Partner)

	st.Text = "hi @" + strings.ToLower(partner.Name[:2])
	got := a.suggestions(room.ID, st)
	a.applySuggestion(room.ID, st, got[0])
	if st.Text != "hi @"+partner.Name+" " {
		t.Fatalf("text = %q", st.Text)
	}
	out := a.expandMentions(room.ID, st.Text+"you there?")
	want := "hi [@" + partner.Name + "](https://matrix.to/#/" + room.Partner + ") you there?"
	if out != want {
		t.Fatalf("sent = %q, want %q", out, want)
	}
	if len(a.mentions[room.ID]) != 0 {
		t.Fatal("the room's chosen mentions were not cleared by sending")
	}
	// A mention edited out of the text is not turned into a link.
	st.Text = "x @" + strings.ToLower(partner.Name[:2])
	a.applySuggestion(room.ID, st, a.suggestions(room.ID, st)[0])
	if got := a.expandMentions(room.ID, "no mention now"); got != "no mention now" {
		t.Fatalf("expanded %q", got)
	}

	st.Text = "ok :smi"
	a.applySuggestion(room.ID, st, a.suggestions(room.ID, st)[0])
	if !strings.HasPrefix(st.Text, "ok ") || strings.Contains(st.Text, ":smi") {
		t.Fatalf("text = %q", st.Text)
	}
}

// The popup shows over the composer, and pressing a line takes it.
func TestSuggestionPopupRenders(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 6)
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	partner, _ := a.src.Person(room.Partner)
	st := a.draft(room.ID)
	st.Text = "@" + strings.ToLower(partner.Name[:2])

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if !tt.HasText(partner.Name) {
		t.Fatal("the suggestion does not show")
	}
	snap(t, "suggest-popup", tt)
	if err := tt.Click(room.Partner); err != nil { // the address line is only in the popup
		t.Fatal(err)
	}
	settle(tt)
	if st.Text != "@"+partner.Name+" " {
		t.Fatalf("pressing the suggestion left %q", st.Text)
	}
	if got := a.suggestions(room.ID, st); len(got) != 0 {
		t.Fatalf("suggestions linger after a pick: %+v", got)
	}
}

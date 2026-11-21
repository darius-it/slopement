package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
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

// The field keeps the keyboard focus while suggestions show: typing goes on,
// Up and Down move the highlight, Tab takes the highlighted one (and not
// Enter, which sends), and typing goes on after it — nothing in between asks
// the user to click back into the field.
func TestTypingAndKeysWhileSuggestionsShow(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 6)
	// A second person who spoke, so there is something to arrow to.
	tl = append(tl, model.Event{ID: "$avery", Room: room.ID, Sender: "@avery:example.com", Kind: model.EventText,
		Body: "hi", Time: time.Now()})
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	st := a.draft(room.ID)

	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if err := tt.Click("Message " + room.Name); err != nil { // into the field
		t.Fatal(err)
	}
	settle(tt)
	tt.Type("hi @a")
	settle(tt)
	if got := a.suggestions(room.ID, st); len(got) < 2 {
		t.Fatalf("need two suggestions to arrow between, have %+v (text %q)", got, st.Text)
	}
	tt.Type("v") // the popup is up: typing must still reach the field
	settle(tt)
	if st.Text != "hi @av" {
		t.Fatalf("typing stopped when the suggestions appeared: %q", st.Text)
	}

	// With the narrowed list, Down and Up only move the highlight: the text is untouched.
	tt.Key(0, ui.KeyBackspace) // back to "@a": both are listed again
	settle(tt)
	tt.Key(0, ui.KeyDown)
	settle(tt)
	if st.Text != "hi @a" {
		t.Fatalf("Down changed the text: %q", st.Text)
	}
	tt.Key(0, ui.KeyTab)
	settle(tt)
	if !strings.HasPrefix(st.Text, "hi @") || !strings.HasSuffix(st.Text, " ") || st.Text == "hi @a" {
		t.Fatalf("Tab did not take a suggestion: %q", st.Text)
	}
	second := st.Text
	tt.Type("ok")
	settle(tt)
	if st.Text != second+"ok" {
		t.Fatalf("typing after the pick was lost: %q (was %q)", st.Text, second)
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
	tt.Type("ok") // pressing a line must not have taken the focus off the field
	settle(tt)
	if st.Text != "@"+partner.Name+" ok" {
		t.Fatalf("typing after pressing a suggestion: %q", st.Text)
	}
	if got := a.suggestions(room.ID, st); len(got) != 0 {
		t.Fatalf("suggestions linger after a pick: %+v", got)
	}
}

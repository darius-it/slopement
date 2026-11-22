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
	// The text is as typed; what is sent has the mention as a link to the person.
	if out, want := st.Draft(), "hi [@"+partner.Name+"](https://matrix.to/#/"+room.Partner+") "; out != want {
		t.Fatalf("draft = %q, want %q", out, want)
	}
	// A mention replaced (the text set from outside) is plain text again: no link.
	st.Text = "no mention now"
	if got := st.Draft(); got != "no mention now" {
		t.Fatalf("draft %q", got)
	}

	// An emoji replaces the word, and is no link.
	st.Text = "ok :smi"
	a.applySuggestion(room.ID, st, a.suggestions(room.ID, st)[0])
	if !strings.HasPrefix(st.Text, "ok ") || strings.Contains(st.Text, ":smi") {
		t.Fatalf("text = %q", st.Text)
	}
	if st.Draft() != st.Text {
		t.Fatalf("an emoji became a link: %q", st.Draft())
	}
}

// After a # the rooms are suggested (not chats with a person, nor the one
// open), by name or address; picking one writes its address and the draft
// links it, where a room with no address is linked by its ID.
func TestRoomSuggestions(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 4)
	a.src = timeline{a.src, tl}
	st := a.draft(room.ID)

	var group model.Room
	for _, r := range a.src.Rooms() {
		if r.Kind != model.KindDM && r.ID != room.ID {
			group = r
			break
		}
	}
	if group.ID == "" {
		t.Skip("no room besides the DM in the mock data")
	}
	st.Text = "see #" + strings.ToLower(group.Name[:2])
	got := a.suggestions(room.ID, st)
	if len(got) == 0 {
		t.Fatalf("no room suggested for %q", st.Text)
	}
	for _, it := range got {
		if r, _ := a.src.Room(it.ID); r.Kind == model.KindDM {
			t.Fatalf("a chat with a person was suggested as a room: %+v", it)
		}
	}
	a.applySuggestion(room.ID, st, got[0])
	if !strings.HasPrefix(st.Text, "see #") {
		t.Fatalf("text = %q", st.Text)
	}
	out := st.Draft()
	if !strings.Contains(out, "](https://matrix.to/#/") {
		t.Fatalf("the room is not a link in what is sent: %q", out)
	}
	// The room open now is not offered to itself.
	st.Text = "#"
	for _, it := range a.suggestions(room.ID, st) {
		if it.ID == room.ID {
			t.Fatal("the open room is suggested to itself")
		}
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

// Taking a suggestion shows at once: the frame the key (or click) leads to
// already holds the new text, and no further frame is left waiting for the
// next event to draw it (that wait was the second-long stutter after Tab).
func TestPickingShowsAtOnce(t *testing.T) {
	setup := func() (*App, *ui.Tester, model.Room) {
		a := testApp()
		room, tl := longTimeline(a, 6)
		tl = append(tl, model.Event{ID: "$avery", Room: room.ID, Sender: "@avery:example.com", Kind: model.EventText,
			Body: "hi", Time: time.Now()})
		a.src = timeline{a.src, tl}
		a.open(room.ID)
		tt := ui.NewTester(a.View, 1100, 700)
		settle(tt)
		if err := tt.Click("Message " + room.Name); err != nil {
			t.Fatal(err)
		}
		settle(tt)
		tt.Type("see @av")
		settle(tt)
		return a, tt, room
	}

	// By Tab: the text is already replaced when the key has been handled —
	// before any frame — so the frame that follows the key draws it.
	a, tt, room := setup()
	st := a.draft(room.ID)
	tt.Key(0, ui.KeyTab) // no settle: what this one key leads to is all there is
	if st.Text != "see @avery " {
		t.Fatalf("Tab did not take the mention in its own handling: %q", st.Text)
	}

	// By mouse: the pick is made while the frame is built, so it must ask for
	// the next frame itself, or the change waits for the next event.
	a2, tt2, room2 := setup()
	if err := tt2.Click("@avery:example.com"); err != nil { // the popup's address line
		t.Fatal(err)
	}
	if a2.draft(room2.ID).Text != "see @avery " {
		t.Fatalf("the click did not take the mention: %q", a2.draft(room2.ID).Text)
	}
	if !tt2.FramePending() {
		t.Fatal("the pick by click left no frame queued: it would draw only at the next event")
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

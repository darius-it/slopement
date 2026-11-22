package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// ⌘F opens a bar under the header (not the global search); typing finds the
// room's matches only; Enter and the arrows step through them, newest
// first, jumping to each; ✕ closes it.
func TestFindInThisChat(t *testing.T) {
	old := searchDebounce
	searchDebounce = 0
	defer func() { searchDebounce = old }()

	a := testApp()
	a.open("!dm-alex")
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	tt.Key(ui.Cmd, ui.KeyF)
	settle(tt)
	if !a.find.Open || a.view != viewRoom || !tt.HasText("Find in this chat") && !a.find.Open {
		t.Fatalf("⌘F: find open %v view %v", a.find.Open, a.view)
	}
	if a.switcher.Open {
		t.Fatal("⌘F opened the quick switcher")
	}

	a.find.Text = "tomorrow"
	settle(tt)
	settle(tt)
	if len(a.find.Hits) != 2 {
		t.Fatalf("hits = %d, want the 2 messages of this chat", len(a.find.Hits))
	}
	if !tt.HasText("1 of 2") {
		t.Fatal("the count is not shown")
	}
	first := a.find.Cur
	a.jumpAt = time.Time{}
	if err := tt.Click("Older match"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.find.Cur == first || !tt.HasText("2 of 2") || a.jumpAt.IsZero() {
		t.Fatalf("Older match: at %d cur %q (was %q)", a.find.At, a.find.Cur, first)
	}
	if err := tt.Click("Older match"); err != nil { // round the end
		t.Fatal(err)
	}
	settle(tt)
	if a.find.At != 0 {
		t.Fatalf("stepping past the oldest should wrap to the newest: %d", a.find.At)
	}

	a.find.Text = "zzzqqq"
	settle(tt)
	settle(tt)
	if !tt.HasText("No matches") {
		t.Fatal("no word on a text nothing says")
	}
	// Only this room: a word of another room is not found.
	a.find.Text = "dinner"
	settle(tt)
	settle(tt)
	for _, h := range a.find.Hits {
		if h.Room != "!dm-alex" {
			t.Fatalf("a match from another room: %+v", h)
		}
	}

	if err := tt.Click("Close find"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.find.Open {
		t.Fatal("✕ did not close the bar")
	}
	// Another room's find is its own: opening a room closes the bar.
	a.openFind()
	a.open("!crit")
	settle(tt)
	settle(tt)
	if a.find.Open {
		t.Fatal("the bar stayed open in another room")
	}
}

// deepSource is a source whose room holds only recent messages; paging back
// brings an older one, as an encrypted room's history does.
type deepSource struct {
	model.Source
	room   string
	held   []model.SearchHit
	older  model.SearchHit
	paged  int
	more   bool
	locked int
}

func (s *deepSource) Search(q model.SearchQuery, done func([]model.SearchHit, bool, string)) {
	var out []model.SearchHit
	for _, h := range s.held {
		if q.Room == "" || q.Room == h.Room {
			if strings.Contains(strings.ToLower(h.Body), strings.ToLower(q.Words)) {
				out = append(out, h)
			}
		}
	}
	done(out, true, "")
}

func (s *deepSource) PageBack(room string, pages int, progress func(done, more bool)) {
	s.paged++
	s.held = append(s.held, s.older) // the page arrives
	progress(false, s.more)
	progress(true, s.more)
}

// crawlSource has two encrypted rooms and one not; looking further back pages
// the encrypted ones' history in, and a message turns up.
type crawlSource struct {
	model.Source
	paged []string
	found bool
}

func (s *crawlSource) Rooms() []model.Room {
	return []model.Room{
		{ID: "!enc1", Name: "Secret one", Encrypted: true},
		{ID: "!plain", Name: "Open room"},
		{ID: "!enc2", Name: "Secret two", Encrypted: true},
	}
}

func (s *crawlSource) Search(q model.SearchQuery, done func([]model.SearchHit, bool, string)) {
	if s.found {
		done([]model.SearchHit{{Room: "!enc2", Event: "$e", Sender: "@x:y", Body: "the lunch from long ago", Time: time.Now().AddDate(-2, 0, 0)}}, true, "")
		return
	}
	done(nil, true, "")
}

func (s *crawlSource) PageBack(room string, pages int, progress func(done, more bool)) {
	s.paged = append(s.paged, room)
	if room == "!enc2" {
		s.found = true
	}
	progress(false, false)
	progress(true, false)
}

// "Search further back" on the all-chats search pages the encrypted rooms'
// history in, a room at a time (not the ones the server can search), and
// searches again as it goes.
func TestSearchFurtherBackInEncryptedChats(t *testing.T) {
	old := searchDebounce
	searchDebounce = 0
	defer func() { searchDebounce = old }()

	a := testApp()
	cs := &crawlSource{Source: a.src}
	a.src = cs
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	a.openSearch("lunch")
	settle(tt)
	settle(tt)
	if !tt.HasText("Nothing found.") || !tt.HasText("Search further back") {
		t.Fatal("no way to look further back when nothing is found")
	}
	if err := tt.Click("Search further back"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	settle(tt)
	settle(tt)
	if len(cs.paged) != 2 || cs.paged[0] != "!enc1" || cs.paged[1] != "!enc2" {
		t.Fatalf("paged %v, want the two encrypted rooms in order", cs.paged)
	}
	if len(a.srch.Hits) != 1 || a.srch.Hits[0].Event != "$e" {
		t.Fatalf("the old message was not found after looking further back: %+v", a.srch.Hits)
	}
	if !tt.HasText("Search even further back") {
		t.Fatal("no offer to go further after it ran")
	}
}

// A find asks for older history while it looks, and what that brings is
// found: the match from long ago appears after the newest, without moving
// what the user is on; "Keep looking" asks for more while there is more.
func TestFindPagesOlderHistoryIn(t *testing.T) {
	old := searchDebounce
	searchDebounce = 0
	defer func() { searchDebounce = old }()

	a := testApp()
	room := a.src.Rooms()[0].ID
	ds := &deepSource{Source: a.src, room: room, more: true,
		held:  []model.SearchHit{{Room: room, Event: "$new", Body: "the lunch plan", Time: time.Now()}},
		older: model.SearchHit{Room: room, Event: "$old", Body: "lunch last year", Time: time.Now().AddDate(-1, 0, 0)}}
	a.src = ds
	a.open(room)
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	a.openFind()
	a.find.Text = "lunch"
	settle(tt)
	settle(tt)
	settle(tt)
	if ds.paged != 1 {
		t.Fatalf("history paged %d times, want once for the first search", ds.paged)
	}
	if len(a.find.Hits) != 2 || a.find.Hits[0].Event != "$new" || a.find.Hits[1].Event != "$old" {
		t.Fatalf("hits = %+v (the old one after the newest)", a.find.Hits)
	}
	if a.find.Cur != "$new" {
		t.Fatalf("the old match moved the user off their place: %q", a.find.Cur)
	}
	if !tt.HasText("Keep looking") || !tt.HasText("Searched the last 3000 messages.") {
		t.Fatal("no offer to keep looking while the chat has more")
	}
	if err := tt.Click("Keep looking"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if ds.paged != 2 {
		t.Fatalf("Keep looking did not ask for more: %d", ds.paged)
	}
	ds.more = false
	ds.PageBack(room, 1, func(done, more bool) { a.find.More = more })
	settle(tt)
	if tt.HasText("Keep looking") {
		t.Fatal("offered more when the chat is searched to its start")
	}
}

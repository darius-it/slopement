package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

func TestHighlightParts(t *testing.T) {
	join := func(ps []part) string {
		var b strings.Builder
		for _, p := range ps {
			if p.Hit {
				b.WriteString("[" + p.Text + "]")
			} else {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	for _, tc := range []struct {
		text  string
		words []string
		want  string
	}{
		{"Lunch on sunday, lunch again", []string{"lunch"}, "[Lunch] on sunday, [lunch] again"},
		{"no match here", []string{"zzz"}, "no match here"},
		{"a b ab", []string{"a", "b"}, "[a] [b] [ab]"},
		{"anything", nil, "anything"},
		{"Mixed Case WORD", []string{"word", "mixed"}, "[Mixed] Case [WORD]"},
		{"", []string{"x"}, ""},
	} {
		if got := join(highlightParts(tc.text, tc.words)); got != tc.want {
			t.Errorf("highlightParts(%q, %v) = %q, want %q", tc.text, tc.words, got, tc.want)
		}
	}
}

func TestSnippet(t *testing.T) {
	if got := snippet("  a\n\n b   c ", nil, 50); got != "a b c" {
		t.Fatalf("whitespace: %q", got)
	}
	long := strings.Repeat("filler ", 60) + "needle" + strings.Repeat(" more", 60)
	got := snippet(long, []string{"needle"}, 80)
	if !strings.Contains(got, "needle") || !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Fatalf("a long message is cut around the match: %q", got)
	}
	if n := len([]rune(got)); n > 82 {
		t.Fatalf("snippet is %d runes", n)
	}
	// Cutting must not break a multi-byte character.
	if s := snippet(strings.Repeat("ü", 300), []string{"ü"}, 40); !strings.Contains(s, "ü") {
		t.Fatalf("multi-byte: %q", s)
	}
}

// The search screen finds what is said, grouped by room, narrows by the
// filter chips (which are the typed tokens), and opens the room at the
// message pressed.
func TestSearchScreen(t *testing.T) {
	old := searchDebounce
	searchDebounce = 0
	defer func() { searchDebounce = old }()

	a := testApp()
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	a.openSearch("tomorrow")
	settle(tt)
	settle(tt)
	if len(a.srch.Hits) == 0 {
		t.Fatal("nothing found for a word the mock says")
	}
	if !tt.HasText("tomorrow") {
		t.Fatal("the hits are not shown")
	}
	snap(t, "search-screen", tt)

	// A word nothing says.
	a.srch.Text = "zzzqqq"
	settle(tt)
	settle(tt)
	if len(a.srch.Hits) != 0 || !tt.HasText("Nothing found.") {
		t.Fatalf("hits for nonsense: %d", len(a.srch.Hits))
	}

	// The chips are the tokens.
	a.srch.Text = "tomorrow"
	settle(tt)
	if err := tt.Click("Link"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.srch.Text != "tomorrow has:link" {
		t.Fatalf("the Link chip wrote %q", a.srch.Text)
	}
	if err := tt.Click("Link"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.srch.Text != "tomorrow" {
		t.Fatalf("the Link chip did not take it off again: %q", a.srch.Text)
	}

	// A press on a hit opens the room at the message.
	settle(tt)
	settle(tt)
	if len(a.srch.Hits) == 0 {
		t.Fatal("no hits to press")
	}
	hit := a.srch.Hits[0]
	a.jumpAt = time.Time{}
	who, _ := a.src.Person(hit.Sender)
	if err := tt.Click("Go to the message from " + who.Name); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.view != viewRoom || a.room != hit.Room {
		t.Fatalf("view %v room %q, want the hit's room %q", a.view, a.room, hit.Room)
	}
	if a.jumpAt.IsZero() {
		t.Fatal("no jump to the message was asked for")
	}
}

// The switcher's "Search messages for …" opens the screen with what was
// typed, and ⌘⇧F opens it empty with its help.
func TestSearchEntryPoints(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	settle(tt)
	if a.view != viewSearch || !tt.HasText("Search your messages") {
		t.Fatalf("⌘⇧F: view %v", a.view)
	}
	a.openSearch("from:kai")
	if a.srch.Text != "from:kai" || a.view != viewSearch {
		t.Fatalf("openSearch: %+v", a.srch)
	}
	q := model.ParseSearch(a.srch.Text)
	if q.From != "kai" {
		t.Fatal("the typed filter is not read")
	}
}

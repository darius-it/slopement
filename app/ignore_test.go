package app

import (
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

type ignoreSource struct {
	model.Source
	ignored map[string]bool
	calls   []string
}

func (s *ignoreSource) IsIgnored(m string) bool { return s.ignored[m] }
func (s *ignoreSource) SetIgnored(m string, on bool, done func(string)) {
	s.calls = append(s.calls, m)
	if s.ignored == nil {
		s.ignored = map[string]bool{}
	}
	if on {
		s.ignored[m] = true
	} else {
		delete(s.ignored, m)
	}
	done("")
}
func (s *ignoreSource) IgnoredUsers() []string {
	var out []string
	for m := range s.ignored {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// A person's card offers Ignore (and then Unignore); ignored people are
// listed in Privacy, where they can be un-ignored.
func TestIgnoreFromTheProfileCard(t *testing.T) {
	a := testApp()
	is := &ignoreSource{Source: a.src}
	a.src = is
	var other model.Person
	for _, p := range a.src.People() {
		if p.ID != a.src.Me().ID {
			other = p
			break
		}
	}
	a.profileOpen, a.profileSheet.Open = other.ID, true
	tt := ui.NewTester(a.View, 1100, 900)
	settle(tt)
	// The card's Ignore (the button itself is tested in kit). A dialog may
	// build its card twice in a frame, so the action must be the same the
	// second time: it asks the source once.
	a.profileActs(other.ID, kit.ProfileAction{Ignore: true})
	a.profileActs(other.ID, kit.ProfileAction{Ignore: true})
	settle(tt)
	if !is.ignored[other.ID] || a.profileSheet.Open || len(is.calls) != 1 {
		t.Fatalf("ignored %v, card open %v, calls %v", is.ignored, a.profileSheet.Open, is.calls)
	}

	a.view, a.settings = viewSettings, "Privacy"
	settle(tt)
	if !tt.HasText("Ignored people") || !tt.HasText(other.ID) {
		t.Fatal("the ignored person is not listed in Privacy")
	}
	if err := tt.Click("Unignore " + other.Name); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(is.ignored) != 0 {
		t.Fatalf("still ignored: %v", is.ignored)
	}

	// The card now offers Ignore again; for oneself it offers nothing.
	if a.profile(other.ID).Ignored || !a.profile(other.ID).Ignorable {
		t.Fatal("the card state is wrong after un-ignoring")
	}
}

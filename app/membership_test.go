package app

import (
	"testing"
	"time"

	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

func TestJoinNames(t *testing.T) {
	for n, want := range map[int]string{
		1: "Ada", 2: "Ada and Bo", 3: "Ada, Bo and Cy", 4: "Ada, Bo and 2 others", 7: "Ada, Bo and 5 others",
	} {
		if got := joinNames([]string{"Ada", "Bo", "Cy", "Di", "Ed", "Fy", "Gus"}[:n]); got != want {
			t.Errorf("joinNames(%d) = %q, want %q", n, got, want)
		}
	}
}

// runSource has people joining one after another, a message, two leaving, and a
// ban in the middle of joins.
type runSource struct {
	model.Source
	room string
}

func (s *runSource) Timeline(room string) []model.Event {
	if room != s.room {
		return s.Source.Timeline(room)
	}
	at := time.Now().Add(-time.Hour)
	line := func(id, who, membership string) model.Event {
		at = at.Add(time.Second)
		return model.Event{ID: id, Room: room, Time: at, Sender: "@" + who + ":example.org", Kind: model.EventSystem,
			Body: who + " " + membership, Membership: membership}
	}
	evs := []model.Event{
		line("$j1", "mira", "join"), line("$j2", "sam", "join"), line("$j3", "kai", "join"),
		line("$j4", "noor", "join"), line("$j5", "ola", "join"),
	}
	at = at.Add(time.Minute)
	evs = append(evs, model.Event{ID: "$m", Room: room, Time: at, Sender: "@mira:example.org", Kind: model.EventText, Body: "hi all"})
	evs = append(evs, line("$l1", "lena", "leave"), line("$l2", "tess", "leave"),
		line("$b", "kai", "ban"), line("$j6", "lena", "join"))
	return evs
}

// A run of joins (or of leaves) is one line; a message, a ban or a different
// kind of line ends the run; a single join stays as it is.
func TestRunsOfJoinsAreOneLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(&runSource{Source: m, room: "!crit"}, picker.New(nil))
	bodies := func() []string {
		var out []string
		for _, r := range a.rows("!crit") {
			if r.ev != nil && r.ev.Kind == model.EventSystem {
				out = append(out, r.ev.Body)
			}
		}
		return out
	}
	got := bodies()
	want := []string{"mira, sam and 3 others joined", "lena and tess left", "kai ban", "lena join"}
	if len(got) != len(want) {
		t.Fatalf("lines = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}

	// "Hide joins and leaves" takes the runs away too.
	a.prefs.hideJoins = true
	for _, b := range bodies() {
		if b == "mira, sam and 3 others joined" || b == "lena and tess left" {
			t.Fatalf("a hidden run showed: %q", b)
		}
	}
}

package app

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/matrix"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// fixtureSource serves the converted compatibility fixtures as one room's
// timeline.
type fixtureSource struct {
	model.Source
	room string
	evs  []model.Event
}

func (s *fixtureSource) Timeline(room string) []model.Event {
	if room != s.room {
		return s.Source.Timeline(room)
	}
	return s.evs
}

// loadFixtures converts every testdata/events/*.json as the sync would.
func loadFixtures(t *testing.T, room string) []model.Event {
	t.Helper()
	files, _ := filepath.Glob("../testdata/events/*.json")
	if len(files) == 0 {
		t.Fatal("no fixtures in ../testdata/events")
	}
	sort.Strings(files)
	var evs []model.Event
	at := time.Now().Add(-time.Hour)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		cv, err := matrix.ConvertEvent("@me:example.org", raw)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if cv.Event.ID == "" {
			continue // an edit, a reaction, a redaction: no tile of its own
		}
		e := cv.Event
		e.Room, e.Time = room, at
		at = at.Add(time.Minute)
		evs = append(evs, e)
	}
	return evs
}

// Every fixture renders in the real timeline, wide and on a phone, with no
// row panicking; and the ones that matter say what they should.
func TestFixturesRender(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	src := &fixtureSource{Source: m, room: "!crit"}
	src.evs = loadFixtures(t, "!crit")
	a := New(src, picker.New(nil))
	a.nav = kit.NavRail

	for _, size := range [][2]int{{1100, 5200}, {390, 5200}} {
		a.open("!crit")
		tt := ui.NewTesterWith(a.View, size[0], size[1], ui.TesterOptions{Dark: true})
		settle(tt)
		if len(a.rowPanics) > 0 {
			t.Fatalf("%v: a row panicked: %v", size, a.rowPanics)
		}
		for _, want := range []string{
			"Brandenburger Tor", "Open in maps", // a location card with its words
			"Shared location",
			"somewhere nice", // a location with no usable place
			"waves at everyone",
			"Build 42 passed",
			"yes, at 7", // a reply, its quote taken off
			"Pizza or pasta?", "Which days?",
			"diagram.svg", // an SVG as a file, not drawn
			"A widget was shared",
			"Mira joined",
		} {
			if !tt.HasText(want) {
				t.Errorf("%v: %q is not shown", size, want)
			}
		}
		if tt.HasText("> <@sam") {
			t.Errorf("%v: a reply's '> ' fallback shows", size)
		}
		if tt.HasText("alert(1)") {
			t.Errorf("%v: a script's text shows", size)
		}
		name := "app-fixtures-wide"
		if size[0] < 720 {
			name = "app-fixtures-phone"
		}
		snap(t, name, tt)
	}
}

// A row that panics is a quiet tile with View source; the rows around it, and
// the view, are fine.
func TestARowThatPanicsIsATile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	src := &fixtureSource{Source: m, room: "!crit", evs: []model.Event{
		{ID: "$a", Room: "!crit", Sender: "@x:example.org", Time: time.Now().Add(-3 * time.Minute), Body: "before the bad one"},
		{ID: "$bad", Room: "!crit", Sender: "@x:example.org", Time: time.Now().Add(-2 * time.Minute), Body: "bad"},
		{ID: "$c", Room: "!crit", Sender: "@x:example.org", Time: time.Now().Add(-time.Minute), Body: "after the bad one"},
	}}
	a := New(src, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!crit")
	a.testRowHook = func(ev *model.Event) {
		if ev.ID == "$bad" {
			panic("unexpected event shape")
		}
	}
	tt := ui.NewTesterWith(a.View, 1100, 800, ui.TesterOptions{Dark: true})
	settle(tt)
	if len(a.rowPanics) == 0 {
		t.Fatal("the panic was not caught")
	}
	for _, want := range []string{"Couldn't show this message", "before the bad one", "after the bad one"} {
		if !tt.HasText(want) {
			t.Errorf("%q is not shown", want)
		}
	}
}

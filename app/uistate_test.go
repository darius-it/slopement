package app

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"slopement/kit"
	"slopement/model/mock"
)

// What the user set and where they were comes back after a restart.
func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui.json")
	src := mock.New(time.Now())

	a := New(src, nil)
	rooms := src.Rooms()
	if len(rooms) < 2 {
		t.Fatal("the mock needs rooms")
	}
	a.theme, a.nav, a.sidebar = 2, kit.NavTabs, false
	a.prefs.gifAutoplay, a.prefs.myColor, a.prefs.peopleSub, a.prefs.gifFiles = false, 3, 1, true
	a.settings = "Privacy"
	a.expanded["c1"] = true
	a.open(rooms[1].ID)
	a.draft(rooms[1].ID).Text = "half-typed"
	b, err := json.Marshal(a.state())
	if err != nil {
		t.Fatal(err)
	}
	writeState(path, b)

	a2 := New(mock.New(time.Now()), nil)
	a2.SetStatePath(path)
	switch {
	case a2.theme != 2, a2.nav != kit.NavTabs, a2.sidebar:
		t.Fatalf("appearance: theme=%d nav=%d sidebar=%v", a2.theme, a2.nav, a2.sidebar)
	case a2.prefs.gifAutoplay, a2.prefs.myColor != 3, a2.prefs.peopleSub != 1, !a2.prefs.gifFiles:
		t.Fatalf("prefs = %+v", a2.prefs)
	case a2.settings != "Privacy", !a2.expanded["c1"]:
		t.Fatalf("settings=%q expanded=%v", a2.settings, a2.expanded)
	case a2.room != rooms[1].ID:
		t.Fatalf("room = %q, want %q", a2.room, rooms[1].ID)
	case a2.draft(rooms[1].ID).Text != "half-typed":
		t.Fatalf("draft = %q", a2.draft(rooms[1].ID).Text)
	}
}

// A state file that is not there, or damaged, leaves the defaults.
func TestStateMissingOrBroken(t *testing.T) {
	dir := t.TempDir()
	a := New(mock.New(time.Now()), nil)
	theme := a.theme
	a.SetStatePath(filepath.Join(dir, "nope.json"))
	writeState(filepath.Join(dir, "bad.json"), []byte("{not json"))
	a.SetStatePath(filepath.Join(dir, "bad.json"))
	if a.theme != theme {
		t.Fatal("defaults changed")
	}
}

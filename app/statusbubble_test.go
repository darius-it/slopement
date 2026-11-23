package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// In the Rail layout, with the setting on, the user's status shows as a bubble
// beside their picture with its words, and a press on it edits the status.
func TestStatusBubbleInTheRail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	m.SetStatus(model.Status{Emoji: "🎧", Text: "Heads down until five"})
	settle(tt)
	if tt.HasText("Heads down until five") {
		t.Fatal("the words show in the Rail without the setting")
	}
	if defaultPrefs().statusBubble {
		t.Fatal("the bubble is on by default")
	}

	a.prefs.statusBubble = true
	settle(tt)
	if !tt.HasText("Heads down until five") {
		t.Fatal("no bubble with the status words")
	}
	snap(t, "app-status-bubble", tt)
	if err := tt.Click("Edit your status: 🎧 Heads down until five"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !a.statusDraft.Open || a.statusDraft.Text != "Heads down until five" {
		t.Fatalf("the press did not open the editor on the status: open %v, %q", a.statusDraft.Open, a.statusDraft.Text)
	}
	a.statusDraft.Open = false

	// With no status there is no bubble; the setting also reaches the saved state.
	m.SetStatus(model.Status{})
	settle(tt)
	if tt.HasText("Heads down until five") {
		t.Fatal("a bubble with no status")
	}
	if got := a.prefs.state().prefs(); !got.statusBubble {
		t.Fatal("the setting was not kept")
	}
}

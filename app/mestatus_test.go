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

// In Tabs and One inbox the user's status is under their name, and one click
// on it edits it; with none set it asks for one.
func TestStatusUnderTheNameInTabsAndInbox(t *testing.T) {
	for _, layout := range []struct {
		name string
		nav  kit.NavLayout
	}{{"tabs", kit.NavTabs}, {"inbox", kit.NavInbox}} {
		t.Run(layout.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			m := mock.New(time.Now())
			m.Delay = 0
			a := New(m, picker.New(nil))
			a.nav = layout.nav
			tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
			settle(tt)
			if !tt.HasText("Set a status") {
				t.Fatal("with no status, the foot does not ask for one")
			}
			if err := tt.Click("Edit your status"); err != nil {
				t.Fatal(err)
			}
			settle(tt)
			if !a.statusDraft.Open {
				t.Fatal("one click did not open the status editor")
			}
			a.statusDraft.Open = false

			m.SetStatus(model.Status{Emoji: "🎧", Text: "Heads down"})
			settle(tt)
			if !tt.HasText("Heads down") || tt.HasText("Set a status") {
				t.Fatal("the current status is not shown under the name")
			}
			if err := tt.Click("Edit your status"); err != nil {
				t.Fatal(err)
			}
			settle(tt)
			if !a.statusDraft.Open || a.statusDraft.Text != "Heads down" {
				t.Fatalf("the editor did not open on the current status: open %v, text %q", a.statusDraft.Open, a.statusDraft.Text)
			}
		})
	}
}

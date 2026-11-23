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

// A nickname replaces the display name wherever the person is named — the chat
// list, the chat, People — and goes with "Remove"; the note shows in their
// card. Both are kept by the source.
func TestPrivateNicknameAndNote(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!dm-alex")
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	if !tt.HasText("Alex Rivera") {
		t.Fatal("the display name is not shown to begin with")
	}

	m.SetPersonNote("@alex:example.com", model.PersonNote{Nick: "Lex", Note: "met at the conference"})
	settle(tt)
	if !tt.HasText("Lex") {
		t.Fatal("the nickname is not shown")
	}
	if tt.HasText("Alex Rivera") {
		t.Fatal("the display name still shows beside a nickname")
	}
	spec := a.profile("@alex:example.com")
	if spec.Nickname != "Lex" || spec.Note != "met at the conference" || spec.Name != "Alex Rivera" || !spec.Noteable {
		t.Fatalf("the card: %+v", spec)
	}

	// A nickname wins over "show usernames" too.
	a.prefs.usernames = true
	settle(tt)
	if !tt.HasText("Lex") || tt.HasText("alex") && !tt.HasText("Lex") {
		t.Fatal("the nickname lost to the username setting")
	}
	a.prefs.usernames = false

	// Removed: the display name is back.
	m.SetPersonNote("@alex:example.com", model.PersonNote{})
	settle(tt)
	if !tt.HasText("Alex Rivera") || tt.HasText("Lex") {
		t.Fatal("removing the nickname did not bring the name back")
	}
}

// The editor saves both, and the card button opens it.
func TestNoteEditorSaves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	a.openNoteEditor("@mira:matrix.org")
	settle(tt)
	if !tt.HasText("Nickname and note") {
		t.Fatal("the editor did not open")
	}
	a.noteEd.Nick, a.noteEd.Note = "  Mimi ", " likes tea "
	settle(tt)
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	n := m.PersonNotes()["@mira:matrix.org"]
	if n.Nick != "Mimi" || n.Note != "likes tea" {
		t.Fatalf("saved %+v", n)
	}
	if a.noteEd.Sheet.Open {
		t.Fatal("the editor stayed open")
	}
}

package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// A room's private name is what it is shown by in the list and in its header,
// over its real name (and over a chat's partner name); the dialog saves it and
// "Remove" brings the real one back. Nobody else's view changes: it is the
// source's, per account.
func TestPrivateRoomName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!crit")
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	if !tt.HasText("design-crit") {
		t.Fatal("the room's own name is not shown to begin with")
	}

	a.openRoomNick("!crit")
	settle(tt)
	if !a.roomNickEd.Sheet.Open || !tt.HasText("Private name") {
		t.Fatal("the dialog did not open")
	}
	a.roomNickEd.Name = "crit club"
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if m.RoomNicknames()["!crit"] != "crit club" {
		t.Fatalf("not kept by the source: %v", m.RoomNicknames())
	}
	if !tt.HasText("crit club") {
		t.Fatal("the private name is not shown")
	}
	if tt.HasText("design-crit") {
		t.Fatal("the real name still shows beside the private one")
	}
	if r, _ := a.roomByID("!crit"); r.Name != "crit club" {
		t.Fatalf("roomByID: %q", r.Name)
	}

	// A chat with a person: the private name beats the partner's name.
	m.SetRoomNickname("!dm-alex", "my lunch buddy")
	settle(tt)
	if r, _ := a.roomByID("!dm-alex"); r.Name != "my lunch buddy" {
		t.Fatalf("a chat's private name: %q", r.Name)
	}

	// Removing brings the real name back.
	a.openRoomNick("!crit")
	settle(tt)
	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if _, ok := m.RoomNicknames()["!crit"]; ok || !tt.HasText("design-crit") {
		t.Fatalf("not removed: %v", m.RoomNicknames())
	}
}

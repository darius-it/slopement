package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// Forward is in a message's menu; it opens the chat list, a press on a chat
// sends a copy there and closes it.
func TestForwardAMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!crit")
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)

	var ev model.Event
	for _, e := range m.Timeline("!crit") {
		if e.Kind == model.EventText && strings.TrimSpace(e.Body) != "" {
			ev = e
		}
	}
	if ev.ID == "" {
		t.Skip("no text message in the mock room")
	}
	if !a.msgMenu(ev).Forward {
		t.Fatal("the menu does not offer Forward")
	}
	a.messageChoice("!crit", ev, kit.MsgForward)
	settle(tt)
	if !tt.HasText("Forward to…") {
		t.Fatal("the dialog did not open")
	}
	var to model.Room
	for _, r := range a.forwardTargets("") {
		if r.ID != "!crit" {
			to = r
			break
		}
	}
	before := len(m.Timeline(to.ID))
	if err := tt.Click("Forward to " + to.Name); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	after := m.Timeline(to.ID)
	if len(after) != before+1 || after[len(after)-1].Body != ev.Body || after[len(after)-1].Sender != m.Me().ID {
		t.Fatalf("the copy is not in %s: %d -> %d", to.Name, before, len(after))
	}
	if a.fwd.Sheet.Open {
		t.Fatal("the dialog stayed open after forwarding")
	}
}

// listReadOnly marks a room read-only in the room list.
type listReadOnly struct {
	model.Source
	room string
}

func (s *listReadOnly) Rooms() []model.Room {
	rooms := s.Source.Rooms()
	for i := range rooms {
		if rooms[i].ID == s.room {
			rooms[i].ReadOnly = true
		}
	}
	return rooms
}

// Rooms the user cannot write in are not offered, and the search narrows.
func TestForwardTargetsFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(&listReadOnly{Source: m, room: "!crit"}, picker.New(nil))
	for _, r := range a.forwardTargets("") {
		if r.ID == "!crit" {
			t.Fatal("a read-only room is offered")
		}
	}
	all := a.forwardTargets("")
	if len(all) < 2 {
		t.Skip("too few mock rooms")
	}
	one := a.forwardTargets(strings.ToUpper(all[0].Name[:3]))
	if len(one) == 0 || len(one) > len(all) {
		t.Fatalf("search gave %d of %d", len(one), len(all))
	}
}

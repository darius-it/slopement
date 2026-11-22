package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

type editSource struct {
	model.Source
	perms  model.RoomPerms
	names  []string
	topics []string
	fail   string
}

func (s *editSource) RoomPermissions(room string, done func(model.RoomPerms)) { done(s.perms) }
func (s *editSource) SetRoomName(room, n string, done func(string)) {
	s.names = append(s.names, n)
	done(s.fail)
}
func (s *editSource) SetRoomTopic(room, t string, done func(string)) {
	s.topics = append(s.topics, t)
	done(s.fail)
}
func (s *editSource) SetRoomAvatar(string, model.File, func(string)) {}
func (s *editSource) RemoveRoomAvatar(string, func(string))          {}

func editApp(t *testing.T, perms model.RoomPerms) (*App, *editSource, *ui.Tester, model.Room) {
	t.Helper()
	a := testApp()
	var room model.Room
	for _, r := range a.src.Rooms() {
		if r.Kind != model.KindDM {
			room = r
			break
		}
	}
	es := &editSource{Source: a.src, perms: perms}
	a.src = es
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	return a, es, tt, room
}

// The edit dialog offers what the user may change and sends only what
// changed; with the power to change nothing it says so.
func TestEditRoomDialog(t *testing.T) {
	a, es, tt, room := editApp(t, model.RoomPerms{Name: true, Topic: true, Avatar: true})
	a.openRoomEdit(room.ID)
	settle(tt)
	if !tt.HasText("Room settings") || !tt.HasText("Name") {
		t.Fatal("the dialog is not open")
	}
	a.re.Name, a.re.Topic = "A better name", room.Topic
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(es.names) != 1 || es.names[0] != "A better name" || len(es.topics) != 0 {
		t.Fatalf("sent names %v topics %v (only the name changed)", es.names, es.topics)
	}
	if a.re.Sheet.Open {
		t.Fatal("the dialog stayed open after saving")
	}

	// A refusal stays in the dialog.
	a.openRoomEdit(room.ID)
	settle(tt)
	es.fail = "You don't have permission to change the topic."
	a.re.Topic = "a different topic"
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !a.re.Sheet.Open || !tt.HasText("You don't have permission to change the topic.") {
		t.Fatal("the refusal is not shown")
	}
}

func TestEditRoomWithoutPower(t *testing.T) {
	a, es, tt, room := editApp(t, model.RoomPerms{})
	a.openRoomEdit(room.ID)
	settle(tt)
	settle(tt)
	if !tt.HasText("You don't have permission to change this.") {
		t.Fatal("no word that nothing can be changed")
	}
	if len(es.names)+len(es.topics) != 0 {
		t.Fatal("something was sent")
	}
	// Only name: the topic is said to be out of reach.
	es.perms = model.RoomPerms{Name: true}
	a.openRoomEdit(room.ID)
	settle(tt)
	settle(tt)
	if !tt.HasText("You can't change its topic.") || tt.HasText("You can't change its name.") {
		t.Fatal("the fields' permissions are not shown separately")
	}
}

// "Community settings" in a community's menu opens the same dialog.
func TestCommunitySettingsOpensTheDialog(t *testing.T) {
	a, _, tt, _ := editApp(t, model.RoomPerms{Name: true})
	cms := a.src.Communities()
	if len(cms) == 0 {
		t.Skip("no community in the mock")
	}
	a.openRoomEdit(cms[0].ID)
	settle(tt)
	if !a.re.Space || !tt.HasText("Community settings") {
		t.Fatalf("space %v", a.re.Space)
	}
}

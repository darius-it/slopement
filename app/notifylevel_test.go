package app

import (
	"testing"

	"slopement/kit"
	"slopement/model"
)

type levelSource struct {
	model.Source
	set []model.NotifyLevel
}

func (s *levelSource) SetNotifyLevel(room string, l model.NotifyLevel) { s.set = append(s.set, l) }

// The room menu's Notifications submenu chooses a level, which goes to the
// source; a source that cannot says so and nothing is lost.
func TestRoomNotificationLevelFromTheMenu(t *testing.T) {
	a := testApp()
	room := a.src.Rooms()[0]
	ls := &levelSource{Source: a.src}
	a.src = ls

	a.roomAction(room.ID, kit.RoomAction{SetLevel: true, Level: int(model.NotifyMentions)})
	a.roomAction(room.ID, kit.RoomAction{SetLevel: true, Level: int(model.NotifyAll)})
	if len(ls.set) != 2 || ls.set[0] != model.NotifyMentions || ls.set[1] != model.NotifyAll {
		t.Fatalf("levels set: %v", ls.set)
	}

	a.src = ls.Source // no NotifyLevels: the mock
	a.roomAction(room.ID, kit.RoomAction{SetLevel: true, Level: 1})
	if a.toast == "" {
		t.Fatal("no word on a source that cannot change it")
	}
	// The menu state carries the room's level for the check mark.
	r := room
	r.Notify = model.NotifyMentions
	if a.menuState(r).Level != int(model.NotifyMentions) {
		t.Fatal("the menu does not show the room's level")
	}
}

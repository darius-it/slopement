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

// readOnlySource says one room is read-only for the user.
type readOnlySource struct {
	model.Source
	room string
}

func (s *readOnlySource) Room(id string) (model.Room, bool) {
	r, ok := s.Source.Room(id)
	r.ReadOnly = r.ReadOnly || id == s.room
	return r, ok
}

// In a room the user may not write in, a bar stands where the composer
// would, in both layouts; other rooms keep their composer.
func TestReadOnlyRoomShowsABarNotAComposer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(&readOnlySource{Source: m, room: "!crit"}, picker.New(nil))
	a.nav = kit.NavRail

	for _, size := range [][2]int{{1440, 900}, {390, 844}} {
		field := "Message " // the composer's placeholder: "Message Name" wide, "Message" on a phone
		if size[0] < 720 {
			field = "Message"
		}
		a.open("!crit")
		tt := ui.NewTesterWith(a.View, size[0], size[1], ui.TesterOptions{Dark: true})
		settle(tt)
		if !tt.HasText("You don't have permission to post in this room") {
			t.Fatalf("%v: no bar in the read-only room", size)
		}
		if tt.HasText(field) {
			t.Fatalf("%v: the composer is there too", size)
		}
		snap(t, "app-readonly", tt)

		a.open("!dm-alex")
		settle(tt)
		if tt.HasText("You don't have permission to post in this room") || !tt.HasText(field) {
			t.Fatalf("%v: a writable room lacks its composer or shows the bar", size)
		}
	}
}

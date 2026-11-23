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

// threeMembers lists a room's three members; everyone else the app knows of is
// not in it.
type threeMembers struct{ model.Source }

func (s *threeMembers) RoomMembers(room string) []model.Person {
	return []model.Person{
		{ID: "@ada:example.org", Name: "Ada"},
		{ID: "@me:matrix.org", Name: "Robin Ellis"},
		{ID: "@zed:example.org", Name: "Zed"},
	}
}

// The Members panel lists the room's own people, as many as the room has, in
// one order frame after frame.
func TestMembersPanelListsTheRoomsPeople(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(&threeMembers{Source: m}, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!crit")
	a.panel = panelMembers
	tt := ui.NewTesterWith(a.View, 1400, 900, ui.TesterOptions{Dark: true})
	settle(tt)

	// The panel's rows are buttons named by the person (the sidebar has its own
	// for chats, so only names that no chat has are looked for among the others).
	order := func() []string {
		var out []string
		for _, acc := range tt.AccessAll() {
			if acc.Role != ui.RoleButton {
				continue
			}
			switch acc.Name {
			case "Ada", "Zed", "Lena", "Theo", "Barbara Klein", "Jun", "Ola":
				out = append(out, acc.Name)
			}
		}
		return out
	}
	first := order()
	if strings.Join(first, ",") != "Ada,Zed" {
		t.Fatalf("members shown: %v (others the app knows of must not be listed)", first)
	}
	for i := 0; i < 5; i++ {
		settle(tt)
		if got := strings.Join(order(), ","); got != strings.Join(first, ",") {
			t.Fatalf("the order changed: %s -> %s", strings.Join(first, ","), got)
		}
	}
}

// The mock lists the same order every time too (it used to come from a map).
func TestMockMembersAreInAStableOrder(t *testing.T) {
	m := mock.New(time.Now())
	first := m.RoomMembers("!crit")
	for i := 0; i < 20; i++ {
		got := m.RoomMembers("!crit")
		for j := range first {
			if got[j].ID != first[j].ID {
				t.Fatalf("the order changed at %d: %s != %s", j, got[j].ID, first[j].ID)
			}
		}
	}
}

package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

type spaceSource struct {
	model.Source
	order  []string
	rooms  []model.PublicRoom
	joined []string
}

func (s *spaceSource) SetCommunityOrder(ids []string) { s.order = ids }
func (s *spaceSource) SpaceRooms(space string, done func([]model.PublicRoom, string)) {
	done(append([]model.PublicRoom(nil), s.rooms...), "")
}
func (s *spaceSource) CreateRoom(model.RoomSpec, func(string, string))     {}
func (s *spaceSource) JoinRoom(t string, done func(string, string))        { s.joined = append(s.joined, t); done("!x", "") }
func (s *spaceSource) Invite(string, string, func(string))                 {}
func (s *spaceSource) SearchPeople(string, func([]model.Person, string))   {}
func (s *spaceSource) PublicRooms(string, string, func([]model.PublicRoom, string)) {}
func (s *spaceSource) Invites() []model.Invite                             { return nil }
func (s *spaceSource) AcceptInvite(string, func(string, string))           {}
func (s *spaceSource) DeclineInvite(string)                                {}

// Dragging a community onto another puts it in that place: down goes after,
// up goes before; the new order goes to the source.
func TestMoveCommunityOnTheRail(t *testing.T) {
	a := testApp()
	ss := &spaceSource{Source: a.src}
	a.src = ss
	ids := a.communityIDs()
	if len(ids) < 3 {
		t.Skip("needs three communities")
	}
	a.moveCommunity(ids[0], ids[2]) // the first dragged to the third's place
	want := []string{ids[1], ids[2], ids[0]}
	want = append(want, ids[3:]...)
	if !reflect.DeepEqual(ss.order, want) {
		t.Fatalf("dragged down: %v, want %v", ss.order, want)
	}
	a.moveCommunity(ids[2], ids[0]) // the third dragged up to the first's place
	want = []string{ids[2], ids[0], ids[1]}
	want = append(want, ids[3:]...)
	if !reflect.DeepEqual(ss.order, want) {
		t.Fatalf("dragged up: %v, want %v", ss.order, want)
	}
	n := len(ss.order)
	a.moveCommunity(ids[0], ids[0])
	a.moveCommunity("nope", ids[1])
	if len(ss.order) != n || !reflect.DeepEqual(ss.order, want) {
		t.Fatal("a drop on itself or an unknown community changed the order")
	}
	// Move down by one (the menu).
	a.moveCommunityBy(ids[0], 1)
	if ss.order[0] != ids[1] || ss.order[1] != ids[0] {
		t.Fatalf("move down: %v", ss.order)
	}
}

// A community's home lists the rooms it offers that the user is not in, each
// with a Join that joins it and takes it from the list.
func TestCommunityListsRoomsToJoin(t *testing.T) {
	a := testApp()
	ss := &spaceSource{Source: a.src, rooms: []model.PublicRoom{
		{ID: "!in:x", Name: "Already in", Joined: true},
		{ID: "!out:x", Alias: "#out:x", Name: "Open to join", Members: 12, Topic: "come along"},
	}}
	a.src = ss
	cms := a.src.Communities()
	a.community = cms[0].ID
	a.tab = 1
	tt := ui.NewTester(a.View, 1100, 900)
	settle(tt)
	settle(tt)
	if !tt.HasText("More to join") || !tt.HasText("Open to join") || tt.HasText("Already in") {
		t.Fatal("the joinable list is wrong")
	}
	if err := tt.Click("Join Open to join"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(ss.joined) != 1 || ss.joined[0] != "#out:x" {
		t.Fatalf("joined %v (by its address)", ss.joined)
	}
	settle(tt)
	if tt.HasText("Open to join") {
		t.Fatal("a joined room is still offered")
	}
}

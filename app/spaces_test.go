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
	comms  []model.Community             // when set, the communities to list
	hier   map[string][]model.PublicRoom // when set, each space's hierarchy
	own    map[string]model.Room         // when set, rooms the fake owns
}

func (s *spaceSource) SetCommunityOrder(ids []string) { s.order = ids }
func (s *spaceSource) SpaceRooms(space string, done func([]model.PublicRoom, string)) {
	if s.hier != nil {
		done(append([]model.PublicRoom(nil), s.hier[space]...), "")
		return
	}
	done(append([]model.PublicRoom(nil), s.rooms...), "")
}
func (s *spaceSource) Communities() []model.Community {
	if s.comms != nil {
		return s.comms
	}
	return s.Source.Communities()
}
func (s *spaceSource) Room(id string) (model.Room, bool) {
	if r, ok := s.own[id]; ok {
		return r, true
	}
	return s.Source.Room(id)
}
func (s *spaceSource) Leave(room string) {
	delete(s.own, room)
	s.Source.Leave(room)
}
func (s *spaceSource) CreateRoom(model.RoomSpec, func(string, string))     {}
func (s *spaceSource) JoinRoom(t string, done func(string, string)) {
	s.joined = append(s.joined, t)
	for _, list := range [][]model.PublicRoom{s.rooms} {
		for _, r := range list {
			if r.ID == t || r.Alias == t {
				done(r.ID, "")
				return
			}
		}
	}
	for _, list := range s.hier {
		for _, r := range list {
			if r.ID == t || r.Alias == t {
				done(r.ID, "")
				return
			}
		}
	}
	done(t, "")
}
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
	ss := &spaceSource{Source: a.src,
		own: map[string]model.Room{
			"!in:x": {ID: "!in:x", Kind: model.KindRoom, Name: "Already in", Community: "!design", Members: 10, LastActivity: a.now},
		},
		rooms: []model.PublicRoom{
			{ID: "!in:x", Name: "Already in", Joined: true},
			{ID: "!out:x", Alias: "#out:x", Name: "Open to join", Members: 12, Topic: "come along"},
		},
	}
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

// novaSource is a community over the mock with sub-spaces, its hierarchy
// answered in the server's raw order (a joined room, two channels to join
// and two sub-spaces, one of each joined; the unjoined sub-space offers
// rooms of its own).
func novaSource(a *App) *spaceSource {
	ss := &spaceSource{Source: a.src,
		own: map[string]model.Room{
			"!support": {ID: "!support", Kind: model.KindRoom, Name: "support", Community: "!nova", Members: 2, LastActivity: a.now},
			"!speed":   {ID: "!speed", Kind: model.KindRoom, Name: "speedruns", Community: "!games", Members: 5, LastActivity: a.now},
			"!hello":   {ID: "!hello", Kind: model.KindRoom, Name: "hello", Community: "!games", Members: 2, LastActivity: a.now},
			"!games":   {ID: "!games", Kind: model.KindRoom, Name: "Games", Community: "", Members: 40, LastActivity: a.now},
		},
		comms: []model.Community{
			{ID: "!nova", Name: "Nova", Rooms: []string{"!support"}, Spaces: []string{"!games"}},
			{ID: "!games", Name: "Games", Parent: "!nova", Rooms: []string{"!speed", "!hello"}},
		},
		hier: map[string][]model.PublicRoom{
			"!nova": {
				{ID: "!art", Name: "Art Corner", Space: true, Members: 30},
				{ID: "!support", Name: "support", Joined: true, Members: 2},
				{ID: "!random", Name: "random", Members: 4, Topic: "off-topic"},
				{ID: "!games", Name: "Games", Space: true, Joined: true, Members: 40},
				{ID: "!intro", Name: "introductions", Members: 9},
			},
			"!art": {
				{ID: "!gallery", Name: "gallery", Members: 2},
				{ID: "!critique", Name: "critique", Members: 1},
			},
			"!games": {
				{ID: "!speed", Name: "speedruns", Joined: true, Members: 5},
				{ID: "!lfg", Name: "looking-for-group", Members: 3},
			},
		},
	}
	a.src = ss
	return ss
}

// A community's "More to join" mirrors the space's tree: its own rooms and
// its sub-spaces in the space's order, each sub-space a titled group with
// its own rooms beneath — the Join buttons are on the rooms, titles are
// only labels. A room joined from it is no longer offered and opens inside
// the community; one left later is offered again.
func TestMoreToJoinSpaces(t *testing.T) {
	a := testApp()
	ss := novaSource(a)
	a.community = "!nova"
	tt := ui.NewTester(a.View, 1440, 900)
	settle(tt)
	settle(tt)
	snap(t, "more-to-join", tt)

	// The sub-spaces are only labels — the Join buttons are on the rooms —
	// and everything sits in the space's order.
	if !tt.HasText("More to join") || !tt.HasText("Art Corner") || !tt.HasText("gallery") {
		t.Fatal("the tree is not offered")
	}
	if tt.HasText("Join Art Corner") || tt.HasText("Join Games") {
		t.Fatal("a sub-space's title carries a Join")
	}
	art, ok := tt.Find("gallery")
	if !ok {
		t.Fatal("no gallery row under the unjoined sub-space")
	}
	games, ok := tt.Find("looking-for-group")
	if !ok {
		t.Fatal("no looking-for-group row under the joined sub-space")
	}
	intro, ok := tt.Find("introductions")
	if !ok {
		t.Fatal("no introductions row")
	}
	if !(art.Y < games.Y && games.Y < intro.Y) {
		t.Fatal("the sub-spaces are not in the space's order")
	}

	// Joining a channel takes it from the list and queues it to open (the
	// sync has to bring it first).
	if err := tt.Click("Join random"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if tt.HasText("Join random") {
		t.Fatal("a joined room is still offered")
	}
	if len(ss.joined) != 1 || ss.joined[0] != "!random" {
		t.Fatalf("joined %v (it has no address: by its ID)", ss.joined)
	}
	if a.pendingOpen != "!random" {
		t.Fatalf("the joined room is not queued to open: %q", a.pendingOpen)
	}

	// Leaving a room brings it back, in place.
	r, ok := a.src.Room("!support")
	if !ok {
		t.Fatal("no support room")
	}
	a.leaveRoom(r)
	settle(tt)
	if !tt.HasText("Join support") {
		t.Fatal("a left room is not offered again")
	}
	snap(t, "more-to-join-left", tt)

	// The same for a sub-space's room: it returns under the sub-space.
	r, ok = a.src.Room("!speed")
	if !ok {
		t.Fatal("no speedruns room")
	}
	a.leaveRoom(r)
	settle(tt)
	if !tt.HasText("Join speedruns") {
		t.Fatal("a sub-space's left room is not offered again")
	}

	// A channel joined from the list opens inside the community — a
	// sub-space's too; a room of no community opens in Chats.
	a.enter("!hello", false) // a join that just landed
	settle(tt)
	if a.room != "!hello" || a.community != "!nova" {
		t.Fatalf("opened %q in %q: not inside the community", a.room, a.community)
	}
	a.community = "!nova"
	a.enter("!dm-mira", false)
	settle(tt)
	if a.room != "!dm-mira" || a.community != "" {
		t.Fatalf("opened %q in %q: not in Chats", a.room, a.community)
	}
}

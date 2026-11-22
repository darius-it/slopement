package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// rmSource is a source that manages rooms, recording what the app asked and
// answering at once (as the real one does on the UI thread, a moment later).
type rmSource struct {
	model.Source
	joined, created, invited, accepted, declined []string
	specs                                        []model.RoomSpec
	invites                                      []model.Invite
	dir                                          []model.PublicRoom
	people                                       []model.Person
	searches                                     []string
	failJoin                                     string
}

func (s *rmSource) JoinRoom(t string, done func(string, string)) {
	s.joined = append(s.joined, t)
	if s.failJoin != "" {
		done("", s.failJoin)
		return
	}
	done("!joined:example.org", "")
}
func (s *rmSource) CreateRoom(sp model.RoomSpec, done func(string, string)) {
	s.specs = append(s.specs, sp)
	done("!made:example.org", "")
}
func (s *rmSource) Invite(room, who string, done func(string)) {
	s.invited = append(s.invited, room+" "+who)
	done("")
}
func (s *rmSource) SearchPeople(q string, done func([]model.Person, string)) {
	s.searches = append(s.searches, q)
	done(s.people, "")
}
func (s *rmSource) PublicRooms(q, _ string, done func([]model.PublicRoom, string)) { done(s.dir, "") }
func (s *rmSource) Invites() []model.Invite                                        { return s.invites }
func (s *rmSource) AcceptInvite(room string, done func(string, string)) {
	s.accepted = append(s.accepted, room)
	s.invites = nil
	done(room, "")
}
func (s *rmSource) DeclineInvite(room string) {
	s.declined = append(s.declined, room)
	s.invites = nil
}

func rmApp(t *testing.T) (*App, *rmSource) {
	t.Helper()
	a := testApp()
	rs := &rmSource{Source: a.src}
	a.src = rs
	return a, rs
}

// The menu bar's Join a room opens the dialog; an address typed there is
// joined, and the dialog closes.
func TestJoinDialog(t *testing.T) {
	a, rs := rmApp(t)
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	a.Actions().JoinRoom()
	settle(tt)
	if !tt.HasText("Room address or link") {
		t.Fatal("the join dialog did not open")
	}
	a.rd.Target = "#lounge:example.org"
	if err := tt.Click("Join room"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(rs.joined) != 1 || rs.joined[0] != "#lounge:example.org" {
		t.Fatalf("joined: %v", rs.joined)
	}
	if a.rd.Sheet.Open {
		t.Fatal("the dialog stayed open after joining")
	}
	if a.pendingOpen != "!joined:example.org" {
		t.Fatalf("the joined room is not queued to open: %q", a.pendingOpen)
	}

	// A refusal stays in the dialog, in a line.
	rs.failJoin = "No room with that address."
	a.openRooms(0, 0)
	settle(tt)
	a.rd.Target = "#nothere:example.org"
	if err := tt.Click("Join room"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !a.rd.Sheet.Open || !tt.HasText("No room with that address.") {
		t.Fatal("the refusal is not shown in the dialog")
	}
}

// Create: the three kinds make a private room, a public one, a community; a
// community opens as the current one at once.
func TestCreateDialog(t *testing.T) {
	a, rs := rmApp(t)
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	a.openRooms(1, 2) // the rail's + → create, a community
	settle(tt)
	if !tt.HasText("Community") || !tt.HasText("Private room") {
		t.Fatal("no kinds to choose from")
	}
	a.rd.Name = "Study group"
	if err := tt.Click("Create community"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(rs.specs) != 1 || rs.specs[0].Name != "Study group" || !rs.specs[0].Space || rs.specs[0].Public {
		t.Fatalf("specs: %+v", rs.specs)
	}
	if a.community != "!made:example.org" {
		t.Fatalf("a new community should be open: %q", a.community)
	}

	a.openRooms(1, 1)
	settle(tt)
	a.rd.Name = "Open mic"
	if err := tt.Click("Create room"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if sp := rs.specs[len(rs.specs)-1]; sp.Space || !sp.Public {
		t.Fatalf("a public room: %+v", sp)
	}
}

// Explore lists the directory and joins what is chosen (by its address).
func TestExploreDialog(t *testing.T) {
	a, rs := rmApp(t)
	rs.dir = []model.PublicRoom{
		{ID: "!big:x", Alias: "#big:x", Name: "Big room", Members: 900, Topic: "loud"},
		{ID: "!mine:x", Name: "Already in", Members: 3, Joined: true},
	}
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	a.openRooms(2, 0)
	settle(tt)
	settle(tt)
	if !tt.HasText("Big room") || !tt.HasText("900 members") || !tt.HasText("Joined") {
		t.Fatal("the directory is not listed")
	}
	if err := tt.Click("Join Big room"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(rs.joined) != 1 || rs.joined[0] != "#big:x" {
		t.Fatalf("joined: %v", rs.joined)
	}
}

// Invitations show above the chats; Accept joins and opens; Decline turns
// down.
func TestInvitationsSection(t *testing.T) {
	a, rs := rmApp(t)
	rs.invites = []model.Invite{{ID: "!inv:x", Name: "Lunch club", Inviter: "@kai:example.org"}}
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	if !tt.HasText("Invitations") || !tt.HasText("Lunch club") {
		t.Fatal("the invitation is not shown")
	}
	if err := tt.Click("Accept"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(rs.accepted) != 1 || rs.accepted[0] != "!inv:x" || a.pendingOpen != "!inv:x" {
		t.Fatalf("accepted %v, opening %q", rs.accepted, a.pendingOpen)
	}
	settle(tt)
	if tt.HasText("Lunch club") {
		t.Fatal("an accepted invitation is still shown")
	}

	rs.invites = []model.Invite{{ID: "!no:x", Name: "Spam", Inviter: "@x:y"}}
	settle(tt)
	if err := tt.Click("Decline"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(rs.declined) != 1 || rs.declined[0] != "!no:x" {
		t.Fatalf("declined: %v", rs.declined)
	}
}

// Inviting: the room's Invite searches people as one types and asks the one
// chosen; Add person in People starts a chat with the one chosen.
func TestInviteAndAddPerson(t *testing.T) {
	a, rs := rmApp(t)
	rs.people = []model.Person{{ID: "@kai:example.org", Name: "Kai Fire"}}
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)

	a.openInvite("!room:x")
	settle(tt)
	if !tt.HasText("Invite people") {
		t.Fatal("no invite dialog")
	}
	a.inv.Query = "ka"
	settle(tt)
	settle(tt)
	if len(rs.searches) == 0 || rs.searches[len(rs.searches)-1] != "ka" {
		t.Fatalf("searches: %v", rs.searches)
	}
	if !tt.HasText("Kai Fire") {
		t.Fatal("the person found is not listed")
	}
	if err := tt.Click("Invite"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(rs.invited) != 1 || !strings.Contains(rs.invited[0], "!room:x @kai:example.org") {
		t.Fatalf("invited: %v", rs.invited)
	}
	if !tt.HasText("Invited Kai Fire") {
		t.Fatal("no word that the invitation went")
	}

	a.openAddPerson()
	settle(tt)
	if !tt.HasText("Start a chat") {
		t.Fatal("Add person is not a chat starter")
	}
}

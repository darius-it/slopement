package matrix

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func TestParseTarget(t *testing.T) {
	for _, tc := range []struct {
		in, want, via string
		bad           bool
	}{
		{in: "#room:example.org", want: "#room:example.org"},
		{in: "  !abc:example.org ", want: "!abc:example.org"},
		{in: "https://matrix.to/#/#room:example.org", want: "#room:example.org"},
		{in: "https://matrix.to/#/%23room%3Aexample.org", want: "#room:example.org"},
		{in: "https://matrix.to/#/!abc:example.org?via=one.org&via=two.org", want: "!abc:example.org", via: "one.org,two.org"},
		{in: "matrix:r/room:example.org", want: "#room:example.org"},
		{in: "matrix:roomid/abc:example.org?via=x.org", want: "!abc:example.org", via: "x.org"},
		{in: "", bad: true},
		{in: "just words", bad: true},
		{in: "#nocolon", bad: true},
		{in: "matrix:u/user:example.org", bad: true},
	} {
		got, via, err := parseTarget(tc.in)
		if tc.bad {
			if err == nil {
				t.Errorf("%q should not parse, got %q", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want || strings.Join(via, ",") != tc.via {
			t.Errorf("%q → %q %v %v, want %q via %q", tc.in, got, via, err, tc.want, tc.via)
		}
	}
}

func stripped(typ event.Type, key, sender, raw string) *event.Event {
	e := &event.Event{Type: typ, Sender: id.UserID(sender), Timestamp: 5000, Content: event.Content{VeryRaw: json.RawMessage(raw)}}
	if key != "" {
		e.StateKey = &key
	}
	return e
}

// A sync's invitations are read from their stripped state — kept apart,
// not made rooms of — and go when the room is joined or left.
func TestInvitationsAreNotRooms(t *testing.T) {
	s, _ := testSynced()
	resp := &mautrix.RespSync{}
	resp.Rooms.Invite = map[id.RoomID]*mautrix.SyncInvitedRoom{
		"!inv:example.org": {State: mautrix.SyncEventsList{Events: []*event.Event{
			stripped(event.StateRoomName, "", "@kai:example.org", `{"name":"Lunch club"}`),
			stripped(event.StateMember, "@me:example.org", "@kai:example.org", `{"membership":"invite"}`),
		}}},
		"!dm:example.org": {State: mautrix.SyncEventsList{Events: []*event.Event{
			stripped(event.StateMember, "@me:example.org", "@ada:example.org", `{"membership":"invite","is_direct":true}`),
		}}},
	}
	s.takeInvites(resp)
	if resp.Rooms.Invite != nil {
		t.Fatal("the invitations were left for the room handlers")
	}
	if len(s.rooms) != 0 {
		t.Fatalf("an invitation became a room: %v", s.rooms)
	}
	inv := s.Invites()
	if len(inv) != 2 {
		t.Fatalf("invites = %+v", inv)
	}
	byID := map[string]model.Invite{}
	for _, i := range inv {
		byID[i.ID] = i
	}
	if i := byID["!inv:example.org"]; i.Name != "Lunch club" || i.Inviter != "@kai:example.org" || i.DM {
		t.Fatalf("room invite: %+v", i)
	}
	if i := byID["!dm:example.org"]; !i.DM || i.Name != "ada" || i.Inviter != "@ada:example.org" {
		t.Fatalf("dm invite: %+v", i)
	}

	// Joined elsewhere: the invitation is gone.
	resp2 := &mautrix.RespSync{}
	resp2.Rooms.Join = map[id.RoomID]*mautrix.SyncJoinedRoom{"!inv:example.org": {}}
	s.takeInvites(resp2)
	if got := s.Invites(); len(got) != 1 || got[0].ID != "!dm:example.org" {
		t.Fatalf("after joining: %+v", got)
	}

	// They survive the cache.
	b := s.invitesRecord()
	s2, _ := testSynced()
	s2.loadInvites(b)
	if got := s2.Invites(); len(got) != 1 || got[0].Inviter != "@ada:example.org" {
		t.Fatalf("after the cache: %+v", got)
	}
}

type roomsServer struct {
	mu     sync.Mutex
	reqs   []string
	bodies []string
}

func (f *roomsServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Method+" "+r.URL.RequestURI())
	f.bodies = append(f.bodies, string(body))
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	p := r.URL.Path
	switch {
	case strings.HasSuffix(p, "/createRoom"):
		if strings.Contains(string(body), `"room_alias_name":"taken"`) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"errcode":"M_ROOM_IN_USE","error":"in use"}`))
			return
		}
		_, _ = w.Write([]byte(`{"room_id":"!new:example.org"}`))
	case strings.Contains(p, "/join/"):
		if strings.Contains(p, "nothere") {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"no"}`))
			return
		}
		_, _ = w.Write([]byte(`{"room_id":"!joined:example.org"}`))
	case strings.HasSuffix(p, "/publicRooms"):
		_, _ = w.Write([]byte(`{"chunk":[` +
			`{"room_id":"!small:x","name":"Small","num_joined_members":3},` +
			`{"room_id":"!big:x","canonical_alias":"#big:x","name":"Big","topic":"loud","num_joined_members":900,"room_type":"m.space"}]}`))
	case strings.HasSuffix(p, "/user_directory/search"):
		_, _ = w.Write([]byte(`{"limited":false,"results":[{"user_id":"@kai:example.org","display_name":"Kai"}]}`))
	case strings.Contains(p, "/profile/"):
		_, _ = w.Write([]byte(`{"displayname":"Typed Person"}`))
	default:
		_, _ = w.Write([]byte(`{}`))
	}
}

func (f *roomsServer) find(parts ...string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
outer:
	for i, r := range f.reqs {
		for _, p := range parts {
			if !strings.Contains(r, p) {
				continue outer
			}
		}
		return f.bodies[i], true
	}
	return "", false
}

func roomsSetup(t *testing.T) (*Source, *roomsServer) {
	t.Helper()
	f := &roomsServer{}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	return s, f
}

type result struct{ room, err string }

func wait(t *testing.T, ch chan result) result {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("no answer")
		return result{}
	}
}

// A private room is encrypted and invite-only; a public one is open with an
// address from its name; a community is a space only admins write in. A name
// is needed, and an address in use is said so.
func TestCreateRooms(t *testing.T) {
	s, f := roomsSetup(t)
	ch := make(chan result, 1)
	done := func(room, err string) { ch <- result{room, err} }

	s.CreateRoom(model.RoomSpec{Name: "  "}, done)
	if r := wait(t, ch); r.err == "" {
		t.Fatal("a room with no name was made")
	}

	s.CreateRoom(model.RoomSpec{Name: "Book club", Topic: "reading", Invite: []string{"@kai:example.org"}}, done)
	if r := wait(t, ch); r.room != "!new:example.org" || r.err != "" {
		t.Fatalf("private room: %+v", r)
	}
	body, _ := f.find("/createRoom")
	for _, want := range []string{`"preset":"private_chat"`, `"name":"Book club"`, `"topic":"reading"`, "@kai:example.org", "m.room.encryption"} {
		if !strings.Contains(body, want) {
			t.Fatalf("private room body lacks %q: %s", want, body)
		}
	}
	if r := s.rooms["!new:example.org"]; r == nil || !r.encrypted || r.name != "Book club" {
		t.Fatalf("the new room: %+v", r)
	}

	s.CreateRoom(model.RoomSpec{Name: "Open House!", Public: true}, done)
	wait(t, ch)
	f.mu.Lock()
	pub := f.bodies[len(f.bodies)-1]
	f.mu.Unlock()
	for _, want := range []string{`"preset":"public_chat"`, `"visibility":"public"`, `"room_alias_name":"open-house"`} {
		if !strings.Contains(pub, want) {
			t.Fatalf("public room body lacks %q: %s", want, pub)
		}
	}
	if strings.Contains(pub, "m.room.encryption") {
		t.Fatal("a public room was made encrypted")
	}

	s.CreateRoom(model.RoomSpec{Name: "My community", Space: true}, done)
	wait(t, ch)
	f.mu.Lock()
	sp := f.bodies[len(f.bodies)-1]
	f.mu.Unlock()
	if !strings.Contains(sp, `"type":"m.space"`) || !strings.Contains(sp, `"events_default":100`) || strings.Contains(sp, "m.room.encryption") {
		t.Fatalf("community body: %s", sp)
	}
	if r := s.rooms["!new:example.org"]; !r.space {
		t.Fatal("the community is not a space here")
	}

	s.CreateRoom(model.RoomSpec{Name: "taken", Public: true}, done)
	if r := wait(t, ch); r.room != "" || !strings.Contains(r.err, "taken") {
		t.Fatalf("address in use: %+v", r)
	}
}

func TestAliasFor(t *testing.T) {
	for in, want := range map[string]string{"Book club": "book-club", "  Hi!! There ": "hi-there", "ÄÖ": "", "a_b": "a-b", "x1": "x1"} {
		if got := aliasFor(in); got != want {
			t.Errorf("aliasFor(%q) = %q, want %q", in, got, want)
		}
	}
}

// Joining takes an address or a link and says in a line why it did not
// work; accepting an invitation is joining it, and it goes from the list.
func TestJoinAndAccept(t *testing.T) {
	s, f := roomsSetup(t)
	ch := make(chan result, 1)
	done := func(room, err string) { ch <- result{room, err} }

	s.JoinRoom("garbage", done)
	if r := wait(t, ch); r.err == "" {
		t.Fatal("garbage was joined")
	}
	s.JoinRoom("https://matrix.to/#/#lounge:example.org?via=one.org", done)
	if r := wait(t, ch); r.room != "!joined:example.org" || r.err != "" {
		t.Fatalf("join: %+v", r)
	}
	if _, ok := f.find("POST", "/join/", "lounge", "via=one.org"); !ok {
		t.Fatalf("join request: %v", f.reqs)
	}
	s.JoinRoom("#nothere:example.org", done)
	if r := wait(t, ch); !strings.Contains(r.err, "No room") {
		t.Fatalf("a missing room: %+v", r)
	}

	s.invites = map[string]*model.Invite{"!inv:example.org": {ID: "!inv:example.org", Name: "Club", Inviter: "@kai:example.org"}}
	s.AcceptInvite("!inv:example.org", done)
	wait(t, ch)
	if len(s.Invites()) != 0 {
		t.Fatal("an accepted invitation is still listed")
	}

	s.invites = map[string]*model.Invite{"!no:example.org": {ID: "!no:example.org"}}
	s.DeclineInvite("!no:example.org")
	if len(s.Invites()) != 0 {
		t.Fatal("a declined invitation is still listed")
	}
	waitFor(t, "the leave", func() bool { _, ok := f.find("POST", "no:example.org/leave"); return ok })
}

// The directory and the people search give what the server has; a typed
// address is offered as typed.
func TestDirectoryAndPeople(t *testing.T) {
	s, f := roomsSetup(t)
	s.rooms["!small:x"] = &room{id: "!small:x"}
	rooms := make(chan []model.PublicRoom, 1)
	s.PublicRooms("club", "", func(r []model.PublicRoom, err string) { rooms <- r })
	got := <-rooms
	if len(got) != 2 || got[0].Name != "Big" || got[0].Members != 900 || !got[0].Space || got[0].Alias != "#big:x" {
		t.Fatalf("directory (biggest first): %+v", got)
	}
	if !got[1].Joined || got[0].Joined {
		t.Fatalf("which are joined: %+v", got)
	}
	if b, ok := f.find("POST", "/publicRooms"); !ok || !strings.Contains(b, `"generic_search_term":"club"`) {
		t.Fatalf("directory request: %q", b)
	}

	people := make(chan []model.Person, 1)
	s.SearchPeople("ka", func(p []model.Person, err string) { people <- p })
	if p := <-people; len(p) != 1 || p[0].ID != "@kai:example.org" || p[0].Name != "Kai" {
		t.Fatalf("people: %+v", p)
	}
	s.SearchPeople("@someone:else.org", func(p []model.Person, err string) { people <- p })
	if p := <-people; len(p) < 1 || p[0].ID != "@someone:else.org" || p[0].Name != "Typed Person" {
		t.Fatalf("a typed address: %+v", p)
	}
}

func TestInvitePeople(t *testing.T) {
	s, f := roomsSetup(t)
	got := make(chan string, 1)
	s.Invite("!a:example.org", "not an address", func(err string) { got <- err })
	if <-got == "" {
		t.Fatal("a non-address was invited")
	}
	s.Invite("!a:example.org", "@kai:example.org", func(err string) { got <- err })
	if err := <-got; err != "" {
		t.Fatal(err)
	}
	if b, ok := f.find("POST", "/invite"); !ok || !strings.Contains(b, "@kai:example.org") {
		t.Fatalf("invite request: %q %v", b, ok)
	}
}

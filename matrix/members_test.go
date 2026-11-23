package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// A room's members are the ones the server lists as joined — not everyone the
// app has seen — by name, and in the same order every time; the list is asked
// for once, and again after someone joins or leaves.
func TestRoomMembersAreTheRoomsOwn(t *testing.T) {
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/members") {
			http.NotFound(w, r)
			return
		}
		asked.Add(1)
		if r.URL.Query().Get("membership") != "join" {
			t.Errorf("asked for %q members", r.URL.Query().Get("membership"))
		}
		_, _ = w.Write([]byte(`{"chunk":[
		 {"type":"m.room.member","state_key":"@zed:example.org","sender":"@zed:example.org","event_id":"$1","origin_server_ts":1,"content":{"membership":"join","displayname":"Zed"}},
		 {"type":"m.room.member","state_key":"@me:example.org","sender":"@me:example.org","event_id":"$2","origin_server_ts":2,"content":{"membership":"join","displayname":"Me"}},
		 {"type":"m.room.member","state_key":"@ada:example.org","sender":"@ada:example.org","event_id":"$3","origin_server_ts":3,"content":{"membership":"join","displayname":"ada"}}]}`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	// Others the app has seen elsewhere are not this room's members.
	for _, id := range []string{"@kai:example.org", "@noor:example.org", "@lena:example.org", "@theo:example.org"} {
		s.people[id] = &model.Person{ID: id, Name: id[1:5]}
	}

	if got := s.RoomMembers("!a"); len(got) != 0 {
		t.Fatalf("before the answer: %v", got)
	}
	waitFor(t, "the members", func() bool { return len(r.memberList) == 3 })
	for round := 0; round < 3; round++ { // the same order every time
		got := s.RoomMembers("!a")
		if len(got) != 3 || got[0].Name != "ada" || got[1].Name != "Me" || got[2].Name != "Zed" {
			t.Fatalf("members = %+v", got)
		}
	}
	if r.members != 3 {
		t.Fatalf("the count is %d, want 3", r.members)
	}
	if asked.Load() != 1 {
		t.Fatalf("asked %d times, want once", asked.Load())
	}

	// Someone joins: asked again, once.
	s.onMember(nil, &event.Event{
		ID: "$j", RoomID: id.RoomID("!a"), Sender: "@noor:example.org", StateKey: ptr("@noor:example.org"), Type: event.StateMember,
		Content: event.Content{Parsed: &event.MemberEventContent{Membership: event.MembershipJoin, Displayname: "Noor"}},
	})
	s.RoomMembers("!a")
	s.RoomMembers("!a")
	waitFor(t, "the second ask", func() bool { return asked.Load() == 2 })
}

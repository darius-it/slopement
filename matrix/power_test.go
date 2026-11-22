package matrix

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func levels(eventsDefault int, users map[id.UserID]int, events map[string]int) *event.PowerLevelsEventContent {
	pl := &event.PowerLevelsEventContent{EventsDefault: eventsDefault, Users: users}
	if len(events) > 0 {
		pl.Events = map[string]int{}
		for k, v := range events {
			pl.Events[k] = v
		}
	}
	return pl
}

// A message needs the room's level for m.room.message, or events_default
// without one, from a user whose level is at least that (0 when not listed).
func TestCanPost(t *testing.T) {
	me := "@me:example.org"
	for name, tc := range map[string]struct {
		pl   *event.PowerLevelsEventContent
		want bool
	}{
		"unknown levels: the server decides": {nil, true},
		"an ordinary room":                   {levels(0, nil, nil), true},
		"announcements, a plain member":      {levels(50, map[id.UserID]int{"@mod:example.org": 50}, nil), false},
		"announcements, a moderator":         {levels(50, map[id.UserID]int{id.UserID(me): 50}, nil), true},
		"messages set to 100 specifically":   {levels(0, nil, map[string]int{"m.room.message": 100}), false},
		"messages open though others not":    {levels(100, nil, map[string]int{"m.room.message": 0}), true},
		"the user's own level is lower":      {levels(10, map[id.UserID]int{id.UserID(me): 5}, nil), false},
	} {
		if got := canPost(tc.pl, me); got != tc.want {
			t.Errorf("%s: canPost = %v, want %v", name, got, tc.want)
		}
	}
}

// The levels the sync brings make the room read-only, and a change back
// makes it writable again.
func TestPowerLevelsMakeARoomReadOnly(t *testing.T) {
	s, _ := testSynced()
	s.roomOf("!a")
	if r, _ := s.Room("!a"); r.ReadOnly {
		t.Fatal("a room with no levels known is read-only")
	}
	s.setPower("!a", levels(50, map[id.UserID]int{"@admin:example.org": 100}, nil))
	if r, _ := s.Room("!a"); !r.ReadOnly {
		t.Fatal("not read-only with events_default 50 and the user at 0")
	}
	if rooms := s.Rooms(); len(rooms) != 1 || !rooms[0].ReadOnly {
		t.Fatalf("the list does not say it: %+v", rooms)
	}
	s.setPower("!a", levels(0, nil, nil))
	if r, _ := s.Room("!a"); r.ReadOnly {
		t.Fatal("still read-only after the levels opened")
	}
}

// The first time a room is shown its levels are asked of the server (a sync
// resumed from the cache does not repeat state), once.
func TestPowerLevelsAskedOnce(t *testing.T) {
	asked := make(chan struct{}, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		asked <- struct{}{}
		_, _ = w.Write([]byte(`{"events_default":50,"users":{"@admin:example.org":100}}`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.roomOf("!a")
	s.Room("!a")
	s.Room("!a")
	waitFor(t, "the room to become read-only", func() bool { r, _ := s.Room("!a"); return r.ReadOnly })
	if len(asked) != 1 {
		t.Fatalf("asked %d times", len(asked))
	}
}

// A refusal to fetch leaves the room writable (the server decides) and asks
// again next time.
func TestPowerLevelsFailureLeavesRoomWritable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errcode":"M_UNKNOWN","error":"x"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.roomOf("!a")
	s.Room("!a")
	waitFor(t, "the ask to be forgotten", func() bool { return !s.rooms["!a"].powerAsked })
	if r, _ := s.Room("!a"); r.ReadOnly {
		t.Fatal("a failed ask made the room read-only")
	}
}

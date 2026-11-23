package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func memberEvent(typ, key, sender, content string) *event.Event {
	return &event.Event{
		ID: id.EventID("$" + key), RoomID: "!r", Sender: id.UserID(sender), StateKey: &key,
		Type: event.Type{Type: typ, Class: event.StateEventType}, Content: event.Content{VeryRaw: []byte(content)},
	}
}

// A room shows a call going on while someone's membership has content, counted
// by person (two devices of one person are one), and not once emptied.
func TestRoomShowsWhoIsInItsCall(t *testing.T) {
	s, _ := testSynced()
	s.roomOf("!r")
	if got, _ := s.Room("!r"); got.Call || got.CallCount != 0 {
		t.Fatal("a call with nobody in it")
	}
	s.onCallEvent(nil, memberEvent("m.rtc.member", "_@a:x_1", "@a:x", `{"slot_id":"s"}`))
	s.onCallEvent(nil, memberEvent("m.rtc.member", "_@a:x_2", "@a:x", `{"slot_id":"s"}`)) // the same person, another device
	s.onCallEvent(nil, memberEvent("org.matrix.msc3401.call.member", "@b:x", "@b:x", `{"m.calls":[{}]}`))
	if got, _ := s.Room("!r"); !got.Call || got.CallCount != 2 {
		t.Fatalf("call %v with %d people, want 2", got.Call, got.CallCount)
	}
	s.onCallEvent(nil, memberEvent("m.rtc.member", "_@a:x_1", "@a:x", `{}`)) // left
	s.onCallEvent(nil, memberEvent("m.rtc.member", "_@a:x_2", "@a:x", `{}`))
	if got, _ := s.Room("!r"); got.CallCount != 1 {
		t.Fatalf("after one left: %d", got.CallCount)
	}
	s.onCallEvent(nil, memberEvent("org.matrix.msc3401.call.member", "@b:x", "@b:x", `{}`))
	if got, _ := s.Room("!r"); got.Call {
		t.Fatal("still a call after everyone left")
	}
	// Other state events are not calls.
	s.onCallEvent(nil, memberEvent("m.room.topic", "", "@a:x", `{"topic":"t"}`))
	if got, _ := s.Room("!r"); got.Call {
		t.Fatal("a topic made a call")
	}
}

// A call that began before the app started is read from the room's state once.
func TestRefreshCallReadsTheRoomStateOnce(t *testing.T) {
	asked := 0
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/state") { // the room's whole state (other requests are the app's own)
			asked++
		}
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
		 {"type":"m.rtc.member","state_key":"_@a:x_1","sender":"@a:x","content":{"slot_id":"s"},"event_id":"$1","origin_server_ts":1},
		 {"type":"m.rtc.member","state_key":"_@b:x_1","sender":"@b:x","content":{},"event_id":"$2","origin_server_ts":2}]`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:x", "t")
	s.cli = cli
	s.roomOf("!r")
	s.RefreshCall("!r")
	s.RefreshCall("!r")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := s.Room("!r"); got.CallCount == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got, _ := s.Room("!r"); got.CallCount != 1 {
		t.Fatalf("count %d, want 1", got.CallCount)
	}
	if asked != 1 {
		t.Fatalf("asked the server %d times: %v", asked, paths)
	}
}

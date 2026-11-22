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

	"slopement/model"
)

// A fake homeserver that records requests, with canned answers for what the
// room actions read back (account data, room creation).
type actionServer struct {
	mu      sync.Mutex
	reqs    []string
	bodies  []string
	accData map[string]string // account data name → JSON it answers a GET with
	newRoom string
}

func (f *actionServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Method+" "+r.URL.Path)
	f.bodies = append(f.bodies, string(body))
	acc := f.accData
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/account_data/"):
		name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if v, ok := acc[name]; ok {
			_, _ = w.Write([]byte(v))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"no"}`))
	case strings.HasSuffix(r.URL.Path, "/createRoom"):
		_, _ = w.Write([]byte(`{"room_id":"` + f.newRoom + `"}`))
	default:
		_, _ = w.Write([]byte(`{"event_id":"$ok"}`))
	}
}

// has: a request whose "METHOD path" holds all of the parts.
func (f *actionServer) has(parts ...string) (string, bool) {
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

func actionSetup(t *testing.T, fake *actionServer) (*Source, *room) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(srv.Close)
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	r.name = "Alpha"
	r.appendEvent(model.Event{ID: "$1", Sender: "@kai:example.org", Kind: model.EventText, Body: "hi", Time: at(1)}, "kai", false)
	return s, r
}

// Favourite and low priority are the room's m.favourite and m.lowpriority
// tags; one takes the other's away.
func TestFavouriteAndLowPriorityTags(t *testing.T) {
	fake := &actionServer{}
	s, r := actionSetup(t, fake)

	s.SetLowPriority("!a", true)
	if !r.lowPriority || r.favourite {
		t.Fatal("low priority not shown at once")
	}
	waitFor(t, "the low priority tag", func() bool { _, ok := fake.has("PUT", "/tags/m.lowpriority"); return ok })

	s.SetFavourite("!a", true)
	if !r.favourite || r.lowPriority {
		t.Fatalf("favourite should replace low priority: fav %v low %v", r.favourite, r.lowPriority)
	}
	waitFor(t, "the favourite tag", func() bool { _, ok := fake.has("PUT", "/tags/m.favourite"); return ok })
	waitFor(t, "the low priority tag taken away", func() bool { _, ok := fake.has("DELETE", "/tags/m.lowpriority"); return ok })

	s.SetFavourite("!a", false)
	if r.favourite {
		t.Fatal("still a favourite")
	}
	waitFor(t, "the tag removed", func() bool { _, ok := fake.has("DELETE", "/tags/m.favourite"); return ok })
}

// Marking unread shows at once as an unread room and writes
// m.marked_unread; reading the room clears it everywhere.
func TestMarkUnread(t *testing.T) {
	fake := &actionServer{}
	s, r := actionSetup(t, fake)
	s.MarkUnread("!a")
	if m := r.modelRoom(); m.Unread != 1 {
		t.Fatalf("a room marked unread shows %d unread", m.Unread)
	}
	waitFor(t, "m.marked_unread", func() bool {
		b, ok := fake.has("PUT", "/rooms/!a/account_data/m.marked_unread")
		return ok && strings.Contains(b, `"unread":true`)
	})
	s.MarkRead("!a")
	if r.markedUnread || r.modelRoom().Unread != 0 {
		t.Fatal("reading the room did not clear the mark")
	}
	waitFor(t, "the mark cleared", func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		for i, r := range fake.reqs {
			if strings.HasPrefix(r, "PUT ") && strings.Contains(r, "m.marked_unread") && strings.Contains(fake.bodies[i], `"unread":false`) {
				return true
			}
		}
		return false
	})
}

// A muted room is a room push rule with no actions; its end, for a timed
// mute, is in account data; unmuting deletes the rule and the entry.
func TestMuteIsAPushRule(t *testing.T) {
	fake := &actionServer{accData: map[string]string{}}
	s, r := actionSetup(t, fake)
	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)

	s.Mute("!a", until)
	if !r.muted || !r.mutedUntil.Equal(until) {
		t.Fatalf("not muted at once: %v %v", r.muted, r.mutedUntil)
	}
	if m := r.modelRoom(); !m.Muted || !m.MutedUntil.Equal(until) {
		t.Fatalf("the list does not show the mute: %+v", m)
	}
	waitFor(t, "the push rule", func() bool { _, ok := fake.has("PUT", "/pushrules/global/room/!a"); return ok })
	body, _ := fake.has("PUT", "/pushrules/global/room/!a")
	var rule struct {
		Actions []any `json:"actions"`
	}
	_ = json.Unmarshal([]byte(body), &rule)
	if rule.Actions == nil || len(rule.Actions) != 0 {
		t.Fatalf("a mute is a rule with empty actions, got %s", body)
	}
	waitFor(t, "the mute's end in account data", func() bool {
		b, ok := fake.has("PUT", "/account_data/com.slopement.mutes")
		return ok && strings.Contains(b, "!a")
	})

	s.Unmute("!a")
	if r.muted || !r.mutedUntil.IsZero() {
		t.Fatal("still muted")
	}
	waitFor(t, "the rule deleted", func() bool { _, ok := fake.has("DELETE", "/pushrules/global/room/!a"); return ok })
}

// What the server says of push rules decides which rooms show as muted:
// from another client's mute too.
func TestPushRulesMarkRoomsMuted(t *testing.T) {
	s, _ := testSynced()
	a := s.roomOf("!a")
	b := s.roomOf("!b")
	b.muted = true
	raw := `{"global":{"room":[{"rule_id":"!a","enabled":true,"actions":[]},{"rule_id":"!b","enabled":true,"actions":["notify"]}]}}`
	evt := &event.Event{Content: event.Content{VeryRaw: json.RawMessage(raw)}}
	s.onPushRules(nil, evt)
	if !a.muted || b.muted {
		t.Fatalf("muted: a %v, b %v", a.muted, b.muted)
	}
}

// Leaving takes the room from the lists at once and tells the server (and
// asks it to forget).
func TestLeaveRoom(t *testing.T) {
	fake := &actionServer{}
	s, _ := actionSetup(t, fake)
	s.Leave("!a")
	if _, ok := s.rooms["!a"]; ok {
		t.Fatal("the room is still listed")
	}
	waitFor(t, "leave", func() bool { _, ok := fake.has("POST", "/rooms/!a/leave"); return ok })
	waitFor(t, "forget", func() bool { _, ok := fake.has("POST", "/rooms/!a/forget"); return ok })
}

// A DM that exists is found; one that does not is made — encrypted, with the
// person invited — and listed under them in m.direct beside the others.
func TestOpenDM(t *testing.T) {
	fake := &actionServer{newRoom: "!dm:example.org",
		accData: map[string]string{"m.direct": `{"@old:example.org":["!old:example.org"]}`}}
	s, _ := actionSetup(t, fake)
	dm := s.roomOf("!kai")
	dm.dm, dm.partner = true, "@kai:example.org"

	if got := s.OpenDM("@kai:example.org"); got != "!kai" {
		t.Fatalf("existing DM = %q", got)
	}
	var found string
	s.OpenDMAsync("@kai:example.org", func(room string) { found = room })
	if found != "!kai" {
		t.Fatalf("OpenDMAsync did not take the existing one: %q", found)
	}

	done := make(chan string, 1)
	s.OpenDMAsync("@new:example.org", func(room string) { done <- room })
	select {
	case room := <-done:
		if room != "!dm:example.org" {
			t.Fatalf("new DM = %q", room)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the DM was not made")
	}
	body, _ := fake.has("POST", "/createRoom")
	for _, want := range []string{`"is_direct":true`, "@new:example.org", "m.room.encryption"} {
		if !strings.Contains(body, want) {
			t.Fatalf("createRoom body lacks %q: %s", want, body)
		}
	}
	waitFor(t, "m.direct", func() bool { _, ok := fake.has("PUT", "/account_data/m.direct"); return ok })
	direct, _ := fake.has("PUT", "/account_data/m.direct")
	if !strings.Contains(direct, "@old:example.org") || !strings.Contains(direct, "!dm:example.org") {
		t.Fatalf("m.direct lost an entry or missed the new: %s", direct)
	}
	r := s.rooms["!dm:example.org"]
	if r == nil || !r.dm || r.partner != "@new:example.org" || !r.encrypted {
		t.Fatalf("the new DM room = %+v", r)
	}
}

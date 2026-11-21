package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// cached is a Source over an account's cache, writing at once.
func cached(t *testing.T, a *Account, device id.DeviceID) *Source {
	t.Helper()
	s := NewSource(&Session{UserID: testUser, DeviceID: device})
	s.cli = &mautrix.Client{UserID: testUser, DeviceID: device}
	s.cache = a.Cache
	a.Cache.setKey([]byte("0123456789abcdef0123456789abcdef"))
	s.update = func(fn func()) { fn() }
	s.persistPending = true // no timer: the test persists by hand
	return s
}

// What a run learned is what the next run starts with: rooms, timelines,
// the encrypted events still waiting, the people, the DMs — and the token.
func TestCacheRoundTrip(t *testing.T) {
	openTest(t)
	ctx := context.Background()
	a, err := Open(ctx, testUser, testDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	s := cached(t, a, testDevice)
	s.rooms["!dm"] = &room{id: "!dm", name: "kai", dm: true, partner: "@kai:example.org", members: 2, hasSummary: true}
	s.markDirty("!dm")
	r := s.roomOf("!a")
	r.name, r.space, r.favourite = "Balls", true, true
	r.children = []spaceChild{{room: "!b", order: "x"}}
	r.appendEvent(model.Event{ID: "$1", Sender: "@kai:example.org", Body: "hi", Kind: model.EventText}, "kai", false)
	r.appendEvent(model.Event{ID: localPrefix + "9", Pending: true, Body: "never sent"}, "You", true)
	r.react("$1", "@me:example.org", "👍")
	enc := &event.Event{ID: "$enc", RoomID: "!a", Sender: "@kai:example.org", Type: event.EventEncrypted,
		Content: event.Content{Parsed: &event.EncryptedEventContent{SessionID: "sess", Algorithm: id.AlgorithmMegolmV1}}}
	r.raw = map[string]*rawEvent{"$enc": {evt: enc, tile: undecryptable(enc)}}
	s.people["@kai:example.org"] = &model.Person{ID: "@kai:example.org", Name: "kai", Presence: model.Online, Contact: true}
	s.direct["!dm"] = true
	s.token, s.tokenDirty = "s123", true
	s.persistNow()
	a.Cache.flush()

	// Decrypted messages must not sit on disk in the clear.
	var stored []byte
	if err := a.Cache.db.QueryRow(`SELECT data FROM rooms WHERE id='!a'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("Balls")) || bytes.Contains(stored, []byte(`"hi"`)) {
		t.Fatal("the cache holds a room in the clear")
	}

	s2 := cached(t, a, testDevice)
	if !s2.loadCache(ctx) {
		t.Fatal("nothing to resume from")
	}
	if s2.token != "s123" {
		t.Fatalf("token = %q", s2.token)
	}
	got := s2.rooms["!a"]
	if got == nil || got.name != "Balls" || !got.space || !got.favourite || len(got.children) != 1 || got.children[0].order != "x" {
		t.Fatalf("room = %+v", got)
	}
	if len(got.events) != 1 || got.events[0].ID != "$1" {
		t.Fatalf("events = %+v (the unsent one must not persist)", got.events)
	}
	if len(got.reactions) == 0 {
		t.Fatalf("reactions lost: %+v", got.reactions)
	}
	if raw := got.raw["$enc"]; raw == nil || raw.evt.Content.AsEncrypted() == nil || raw.evt.Content.AsEncrypted().SessionID != "sess" {
		t.Fatalf("raw = %+v", got.raw)
	}
	if s2.encrypted != 1 {
		t.Fatalf("encrypted = %d", s2.encrypted)
	}
	if dm := s2.rooms["!dm"]; dm == nil || !dm.dm || !dm.hasSummary || dm.members != 2 {
		t.Fatalf("dm = %+v", dm)
	}
	if p := s2.people["@kai:example.org"]; p == nil || p.Name != "kai" || p.Presence != model.Offline {
		t.Fatalf("person = %+v", p)
	}
	if !s2.direct["!dm"] {
		t.Fatal("direct lost")
	}
	if _, err := json.Marshal(got.record()); err != nil {
		t.Fatal(err)
	}
}

// A cache of another device, or of another build, is not trusted: the
// sync starts from nothing.
func TestCacheNotTrusted(t *testing.T) {
	openTest(t)
	ctx := context.Background()
	a, err := Open(ctx, testUser, testDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s := cached(t, a, testDevice)
	s.roomOf("!a")
	s.token, s.tokenDirty = "s1", true
	s.persistNow()
	a.Cache.flush()

	if other := cached(t, a, "OTHERDEV"); other.loadCache(ctx) {
		t.Fatal("another device's cache was resumed")
	}
	// Dropped for good, not just ignored.
	if again := cached(t, a, testDevice); again.loadCache(ctx) {
		t.Fatal("the cache survived being refused")
	}
}

// Leaving a room removes it from the cache; joining it again brings it
// back; the token moves with the state.
func TestCacheLeaveAndRejoin(t *testing.T) {
	openTest(t)
	ctx := context.Background()
	a, err := Open(ctx, testUser, testDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s := cached(t, a, testDevice)
	s.roomOf("!a")
	s.roomOf("!b")
	s.endBatch("s1", nil)
	s.persistNow()
	a.Cache.flush()

	s.endBatch("s2", []string{"!a"})
	s.persistNow()
	a.Cache.flush()
	s2 := cached(t, a, testDevice)
	if !s2.loadCache(ctx) || s2.token != "s2" || s2.rooms["!a"] != nil || s2.rooms["!b"] == nil {
		t.Fatalf("after leaving: token=%q rooms=%v", s2.token, s2.rooms)
	}

	s.roomOf("!a") // joined again
	s.endBatch("s3", []string{"!a"})
	s.roomOf("!a")
	s.persistNow()
	a.Cache.flush()
	s3 := cached(t, a, testDevice)
	if !s3.loadCache(ctx) || s3.rooms["!a"] == nil {
		t.Fatalf("rejoined room lost: %v", s3.rooms)
	}
}

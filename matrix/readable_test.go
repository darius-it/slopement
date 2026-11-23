package matrix

import (
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// A source with a syncer to dispatch to, a room that holds one event waiting for
// its key (as a lock tile, and the encrypted copy held for retries).
func lockedRoom(t *testing.T, evtID string) (*Source, *room, *rawSnapshot) {
	t.Helper()
	s, _ := testSynced()
	s.me.ID = "@me:x"
	cli, _ := mautrix.NewClient("http://127.0.0.1:1", "@me:x", "t")
	syncer := mautrix.NewDefaultSyncer()
	cli.Syncer = syncer
	s.cli = cli
	for _, typ := range rtcNotificationTypes {
		syncer.OnEventType(typ, s.onCallNotification)
	}
	r := s.roomOf("!r")
	tile := model.Event{ID: evtID, Room: "!r", Sender: "@kai:x", Time: time.Now(), Kind: model.EventUndecryptable}
	r.events = []model.Event{tile}
	r.raw = map[string]*rawEvent{evtID: {evt: &event.Event{ID: id.EventID(evtID)}, tile: tile}}
	s.encrypted = 1
	return s, r, &rawSnapshot{id: "!r", raw: r.raw}
}

// Something that decrypted and that nothing shows — the call's own reactions —
// leaves no "waiting for this message" behind.
func TestReadableEventNobodyShowsLeavesNoLockTile(t *testing.T) {
	s, r, snap := lockedRoom(t, "$react")
	decrypted := &event.Event{ID: "$react", RoomID: "!r", Sender: "@kai:x", Timestamp: time.Now().UnixMilli(),
		Type: event.Type{Type: "io.element.call.reaction", Class: event.MessageEventType}}
	s.applyDecrypted(snap, "$react", decrypted)
	if len(r.events) != 0 || len(r.raw) != 0 || s.encrypted != 0 {
		t.Fatalf("a waiting tile is left: %d events, %d held, %d encrypted", len(r.events), len(r.raw), s.encrypted)
	}
}

// An event back from the cache has no class; it must still reach the handler
// of its type, which turns it into its line and replaces the tile.
func TestDecryptedEventFromTheCacheReachesItsHandler(t *testing.T) {
	s, r, snap := lockedRoom(t, "$ring")
	s.people["@kai:x"] = &model.Person{ID: "@kai:x", Name: "Kai"}
	decrypted := &event.Event{ID: "$ring", RoomID: "!r", Sender: "@kai:x", Timestamp: time.Now().UnixMilli(),
		Type:    event.Type{Type: "org.matrix.msc4075.rtc.notification"}, // no class, as loaded from the cache
		Content: event.Content{Raw: map[string]any{"notification_type": "ring"}}}
	if decrypted.Type.Class != event.UnknownEventType {
		t.Fatal("the test event has a class")
	}
	s.applyDecrypted(snap, "$ring", decrypted)
	if len(r.events) != 1 || r.events[0].Kind != model.EventSystem || r.events[0].Body != "Kai started a call" {
		t.Fatalf("events: %+v", r.events)
	}
	if len(r.raw) != 0 || s.encrypted != 0 {
		t.Fatalf("held: %d, encrypted: %d", len(r.raw), s.encrypted)
	}
}

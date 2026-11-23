package matrix

import (
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// A call notification that waited for its key shows as "waiting for this
// message"; when it decrypts it becomes "<who> started a call", and the lock
// tile and its raw copy are gone.
func TestCallNotificationReplacesItsLockTile(t *testing.T) {
	s, _ := testSynced()
	s.me.ID = "@me:x"
	s.people["@kai:x"] = &model.Person{ID: "@kai:x", Name: "Kai"}
	r := s.roomOf("!r")
	locked := model.Event{ID: "$n", Room: "!r", Sender: "@kai:x", Time: time.Now(), Kind: model.EventUndecryptable}
	r.events = []model.Event{locked}
	r.raw = map[string]*rawEvent{"$n": {evt: &event.Event{ID: "$n"}, tile: locked}}
	s.encrypted = 1

	evt := &event.Event{ID: id.EventID("$n"), RoomID: "!r", Sender: "@kai:x", Type: rtcNotificationTypes[0],
		Timestamp: time.Now().UnixMilli(), Content: event.Content{Raw: map[string]any{"notification_type": "ring"}}}
	s.onCallNotification(nil, evt)

	if len(r.events) != 1 || r.events[0].Kind != model.EventSystem || r.events[0].Body != "Kai started a call" {
		t.Fatalf("events: %+v", r.events)
	}
	if len(r.raw) != 0 || s.encrypted != 0 {
		t.Fatalf("the waiting copy stayed: raw %d, encrypted %d", len(r.raw), s.encrypted)
	}
	// One's own is "You".
	mine := &event.Event{ID: "$m", RoomID: "!r", Sender: "@me:x", Type: rtcNotificationTypes[0], Timestamp: time.Now().UnixMilli()}
	s.onCallNotification(nil, mine)
	if got := r.events[len(r.events)-1].Body; got != "You started a call" {
		t.Fatalf("own line: %q", got)
	}
}

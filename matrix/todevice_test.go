package matrix

import (
	"encoding/json"
	"testing"
)

// A call's media keys, decrypted from a to-device message, go to the call page
// of the room they name, in the widget API's shape, and to no other room's.
func TestCallKeysGoToTheirRoomsPage(t *testing.T) {
	s, _ := testSynced()
	var got, other []json.RawMessage
	var toDevice, otherToDevice bool
	s.WatchCall("!a", func(td bool, raw json.RawMessage) { toDevice = td; got = append(got, raw) })
	s.WatchCall("!b", func(td bool, raw json.RawMessage) { otherToDevice = td; other = append(other, raw) })

	s.deliverToDevice("io.element.call.encryption_keys", "@kai:x", json.RawMessage(`{"room_id":"!a","keys":{"index":0,"key":"k"},"member":{"id":"m"}}`))
	if len(got) != 1 || !toDevice || len(other) != 0 || otherToDevice {
		t.Fatalf("routing: %d for the room, %d for the other", len(got), len(other))
	}
	var m struct {
		Type      string
		Sender    string
		Encrypted bool
		Content   struct{ Room_id string }
	}
	if err := json.Unmarshal(got[0], &m); err != nil || m.Type != "io.element.call.encryption_keys" || m.Sender != "@kai:x" || !m.Encrypted || m.Content.Room_id != "!a" {
		t.Fatalf("the message is %s (%v)", got[0], err)
	}
	// No room named: every call page hears it.
	s.deliverToDevice("io.element.call.encryption_keys", "@kai:x", json.RawMessage(`{"keys":{}}`))
	if len(got) != 2 || len(other) != 1 {
		t.Fatalf("without a room: %d and %d", len(got), len(other))
	}
}

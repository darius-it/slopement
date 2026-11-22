package mock

import (
	"encoding/json"

	"slopement/model"
)

// EventSource shows a mock event as the JSON an m.room.message would be.
func (s *Source) EventSource(room, id string, done func(original, decrypted, err string)) {
	for _, e := range s.timeline[room] {
		if e.ID != id {
			continue
		}
		b, _ := json.MarshalIndent(map[string]any{
			"event_id": e.ID, "room_id": e.Room, "sender": e.Sender,
			"origin_server_ts": e.Time.UnixMilli(), "type": "m.room.message",
			"content": map[string]any{"msgtype": "m.text", "body": e.Body},
		}, "", "  ")
		done(string(b), "", "")
		return
	}
	done("", "", "That event is not held.")
}

var _ model.EventSourcer = (*Source)(nil)

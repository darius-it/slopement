package matrix

import (
	"context"
	"encoding/json"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// EventSource fetches one event from the server and shows it as JSON: as
// stored (encrypted, if it is) and, when this device has the key, as it
// reads decrypted. It is what a bug report about one message needs.
func (s *Source) EventSource(room, eventID string, done func(original, decrypted, err string)) {
	if s.cli == nil {
		done("", "", "Not signed in.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		evt, err := s.cli.GetEvent(ctx, id.RoomID(room), id.EventID(eventID))
		if err != nil {
			s.post(func() { done("", "", "Could not fetch the event: "+plain(err)) })
			return
		}
		evt.RoomID = id.RoomID(room)
		original := prettyJSON(evt)
		decrypted, why := "", ""
		if evt.Type == event.EventEncrypted {
			switch {
			case s.helper == nil:
				why = "This session cannot decrypt."
			default:
				_ = evt.Content.ParseRaw(evt.Type)
				if dec := s.decryptFetched(ctx, evt); dec != nil {
					dec.RoomID = id.RoomID(room)
					decrypted = prettyJSON(dec)
				} else {
					why = "Not decrypted: this device has no key for it yet."
				}
			}
		}
		s.post(func() { done(original, decrypted, why) })
	}()
}

// prettyJSON is an event as indented JSON.
func prettyJSON(evt *event.Event) string {
	b, err := json.MarshalIndent(evt, "", "  ")
	if err != nil {
		return "(could not be shown: " + err.Error() + ")"
	}
	return string(b)
}

var _ model.EventSourcer = (*Source)(nil)

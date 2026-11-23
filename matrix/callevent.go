package matrix

import (
	"context"
	"encoding/json"
	"time"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// A call starting is announced in the room by an m.rtc.notification event (the
// call page sends it). It is shown as one quiet line, "<who> started a call".
// In an encrypted room it arrives encrypted, and when it was waiting for its
// key and decrypts later it must take the place of the "waiting for this
// message" tile it was shown as; with no handler for its type nothing did, and
// the tile stayed for good.

var rtcNotificationTypes = []event.Type{
	{Type: "m.rtc.notification", Class: event.MessageEventType},
	{Type: "org.matrix.msc4075.rtc.notification", Class: event.MessageEventType},
}

func (s *Source) onCallNotification(_ context.Context, evt *event.Event) {
	sender := evt.Sender.String()
	you := sender == s.me.ID
	e := model.Event{
		ID:     evt.ID.String(),
		Room:   evt.RoomID.String(),
		Sender: sender,
		Time:   eventTime(evt),
		Kind:   model.EventSystem,
		Call:   &model.CallLine{},
	}
	s.post(func() {
		name := s.nameOf(sender)
		e.Body = name + " started a call"
		if you {
			e.Body = "You started a call"
		}
		r := s.roomOf(evt.RoomID.String())
		if evt.Mautrix.EventSource&sourceHistory != 0 {
			r.insertOlder(e)
		} else {
			r.appendEvent(e, name, you)
		}
		if _, raw := r.raw[e.ID]; raw { // it had waited for its key: the tile is this line now
			delete(r.raw, e.ID)
			s.encrypted--
		}
		// Somebody calling: a notification, once, while it is fresh. A ring
		// always tells; a plain notification (a call started in a room) only
		// where messages would.
		if you || evt.Mautrix.EventSource&sourceHistory != 0 || s.notify == nil || r.muted || s.ignored[sender] ||
			time.Since(e.Time) > notifyWindow || s.notified[e.ID] {
			return
		}
		if kind := notificationType(evt); kind != "ring" && !r.dm && r.notify != model.NotifyAll {
			return
		}
		if s.notified == nil {
			s.notified = map[string]bool{}
		}
		s.notified[e.ID] = true
		title, body := r.name, name+" is calling"
		if r.dm {
			title, body = name, "Incoming call"
		}
		s.notify(model.Notice{Room: r.id, Event: e.ID, Title: title, Body: body, Sender: sender, DM: r.dm})
	})
}

// notificationType is what a call notification says it is: "ring" (somebody is
// being called) or "notification" (a call started in a room).
func notificationType(evt *event.Event) string {
	if t, ok := evt.Content.Raw["notification_type"].(string); ok {
		return t
	}
	var c struct {
		T string `json:"notification_type"`
	}
	_ = json.Unmarshal(evt.Content.VeryRaw, &c)
	return c.T
}

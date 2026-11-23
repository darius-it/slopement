package matrix

import (
	"context"

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
	})
}

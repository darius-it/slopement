package matrix

import (
	"context"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// Message actions: edit, delete and pin. Each shows its effect at once (the
// message is edited, gone, pinned) and goes to the server after; if the
// server says no, a quiet line in the room says why. The sync's echo then
// confirms what was already shown.

// notice puts a quiet line in a room's timeline: something that did not
// work, said where it happened. It is not an event: it is never stored.
func (s *Source) notice(room, text string) {
	s.post(func() {
		r := s.roomOf(room)
		r.appendEvent(model.Event{ID: localPrefix + "notice-" + s.cli.TxnID(), Room: room, Kind: model.EventSystem,
			Body: text, Time: time.Now()}, "", true)
	})
}

// Edit replaces the text of one of the user's messages (Markdown in, as
// Send takes it).
func (s *Source) Edit(room, eventID, text string) {
	if s.cli == nil || eventID == "" {
		return
	}
	body := contentOf(text)
	edit := *body
	edit.Body = "* " + body.Body
	if body.FormattedBody != "" {
		edit.FormattedBody = "* " + body.FormattedBody
	}
	edit.NewContent = body
	edit.RelatesTo = (&event.RelatesTo{}).SetReplace(id.EventID(eventID))
	newText, newHTML := body.Body, body.FormattedBody
	s.post(func() { s.roomOf(room).editEvent(eventID, newText, newHTML, time.Now().UnixMilli()) })
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.cli.SendMessageEvent(ctx, id.RoomID(room), event.EventMessage, &edit); err != nil {
			s.notice(room, "Could not edit the message: "+plain(err))
		}
	}()
}

// Redact deletes a message: it goes from the timeline at once.
func (s *Source) Redact(room, eventID string) {
	if s.cli == nil || eventID == "" {
		return
	}
	s.post(func() {
		r := s.roomOf(room)
		if i := r.indexOf(eventID); i >= 0 {
			r.removeAt(i)
		}
	})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.cli.RedactEvent(ctx, id.RoomID(room), id.EventID(eventID)); err != nil {
			s.notice(room, "Could not delete the message: "+plain(err))
		}
	}()
}

// Pin pins a message to the room, or takes its pin away.
func (s *Source) Pin(room, eventID string, on bool) {
	if s.cli == nil || eventID == "" {
		return
	}
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	var pinned []string
	for _, p := range r.pinned {
		if p != eventID {
			pinned = append(pinned, p)
		}
	}
	if on {
		pinned = append(pinned, eventID)
	}
	r.pinned = pinned
	s.markDirty(room)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Built on what the server has now, not on what this app happens to
		// know: a pin list this device never learned must not be replaced by
		// a list of one.
		current, err := s.serverPins(ctx, room)
		if err != nil {
			s.notice(room, "Could not change the pins: "+plain(err))
			return
		}
		var next []id.EventID
		for _, p := range current {
			if p.String() != eventID {
				next = append(next, p)
			}
		}
		if on {
			next = append(next, id.EventID(eventID))
		}
		if _, err = s.cli.SendStateEvent(ctx, id.RoomID(room), event.StatePinnedEvents, "", &event.PinnedEventsEventContent{Pinned: next}); err != nil {
			s.notice(room, "Could not change the pins: "+plain(err))
			return
		}
		list := make([]string, len(next))
		for i, p := range next {
			list[i] = p.String()
		}
		s.post(func() {
			if r, ok := s.rooms[room]; ok {
				r.pinned = list
				s.markDirty(room)
			}
		})
	}()
}

// IsPinned says whether a message is pinned in its room. UI thread.
func (s *Source) IsPinned(room, eventID string) bool {
	if r, ok := s.rooms[room]; ok {
		for _, p := range r.pinned {
			if p == eventID {
				return true
			}
		}
	}
	return false
}

// Pinned lists a room's pinned events, in the order they were pinned. UI
// thread.
func (s *Source) Pinned(room string) []string {
	if r, ok := s.rooms[room]; ok {
		return append([]string(nil), r.pinned...)
	}
	return nil
}

// onPinned is the room's pinned events, as the server has them.
func (s *Source) onPinned(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsPinnedEvents()
	if c == nil {
		return
	}
	pinned := make([]string, len(c.Pinned))
	for i, p := range c.Pinned {
		pinned[i] = p.String()
	}
	room := evt.RoomID.String()
	s.post(func() {
		r := s.roomOf(room)
		r.pinned = pinned
		s.markDirty(room)
		// A pin that is new may be a message this app does not hold.
		r.pinsAsked = true
		go s.fetchPins(room)
	})
}

var _ model.Actions = (*Source)(nil)

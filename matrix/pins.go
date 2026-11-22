package matrix

import (
	"context"
	"errors"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// serverPins is the room's pinned events as the server has them now. A room
// that never had any has none (M_NOT_FOUND is an empty list, not an error).
func (s *Source) serverPins(ctx context.Context, room string) ([]id.EventID, error) {
	var content event.PinnedEventsEventContent
	err := s.cli.StateEvent(ctx, id.RoomID(room), event.StatePinnedEvents, "", &content)
	if err != nil {
		if errors.Is(err, mautrix.MNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return content.Pinned, nil
}

// RefreshPins asks the server for a room's pins and for the pinned messages
// this app does not hold (older than what it loaded, or from before a cache
// that never learned of them): the sync only tells of a pin when it changes,
// so a resumed sync can never say what was pinned before. Once per room per
// run, and again when the room's pins change. UI thread; the work is off it.
func (s *Source) RefreshPins(room string) {
	if s.cli == nil {
		return
	}
	r, ok := s.rooms[room]
	if !ok || r.pinsAsked {
		return
	}
	r.pinsAsked = true
	go s.fetchPins(room)
}

// decryptFetched decrypts an event this app fetched on its own: with the
// keys it has, else with the session from the server's key backup (as the
// timeline's waiting messages are looked for), else asking the user's other
// devices for it. Nil when it is still locked.
func (s *Source) decryptFetched(ctx context.Context, evt *event.Event) *event.Event {
	if dec, err := s.helper.Decrypt(ctx, evt); err == nil {
		return dec
	}
	enc := evt.Content.AsEncrypted()
	if enc == nil {
		return nil
	}
	if s.verify != nil {
		if key := s.verify.savedBackupKey(); key != nil {
			if info, err := s.cli.GetKeyBackupLatestVersion(ctx); err == nil {
				s.importSession(ctx, info.Version, key, evt.RoomID, enc.SessionID)
				if dec, err := s.helper.Decrypt(ctx, evt); err == nil {
					return dec
				}
			}
		}
	}
	s.helper.RequestSession(ctx, evt.RoomID, enc.SenderKey, enc.SessionID, evt.Sender, enc.DeviceID)
	return nil
}

// pinRetries is how often a pin that is still locked is looked at again,
// seconds apart, before the next time the room opens does.
const pinRetries = 4

func (s *Source) fetchPins(room string) { s.fetchPinsTry(room, 0) }

func (s *Source) fetchPinsTry(room string, try int) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	stillLocked := false
	defer func() {
		if stillLocked && try < pinRetries {
			time.AfterFunc(8*time.Second, func() { s.fetchPinsTry(room, try+1) })
		}
	}()
	pins, err := s.serverPins(ctx, room)
	if err != nil {
		s.post(func() {
			if r, ok := s.rooms[room]; ok {
				r.pinsAsked = false // the next time the room opens tries again
			}
		})
		return
	}
	list := make([]string, len(pins))
	for i, p := range pins {
		list[i] = p.String()
	}
	s.post(func() {
		if r, ok := s.rooms[room]; ok {
			r.pinned = list
			s.markDirty(room)
		}
	})
	for _, pid := range pins {
		held := make(chan bool, 1)
		s.post(func() {
			r, ok := s.rooms[room]
			if !ok {
				held <- true // the room is gone: nothing to fetch for
				return
			}
			// Held in the timeline, or fetched and readable; one fetched but
			// still locked is looked for again.
			e, have := r.pinEvents[pid.String()]
			held <- r.indexOf(pid.String()) >= 0 || (have && e.Kind != model.EventUndecryptable)
		})
		if <-held {
			continue
		}
		locked := false
		evt, err := s.cli.GetEvent(ctx, id.RoomID(room), pid)
		if err != nil {
			continue // gone, or not readable by this account
		}
		evt.RoomID = id.RoomID(room)
		if err := evt.Content.ParseRaw(evt.Type); err != nil && evt.Type != event.EventEncrypted {
			continue
		}
		if evt.Type == event.EventEncrypted && s.helper != nil {
			if dec := s.decryptFetched(ctx, evt); dec != nil {
				evt = dec
				evt.RoomID = id.RoomID(room)
			} else {
				locked = true
			}
		}
		var ev model.Event
		switch evt.Type {
		case event.EventMessage, event.EventSticker:
			c := evt.Content.AsMessage()
			if c == nil || (c.MsgType == "" && c.Body == "") {
				continue
			}
			ev = textEvent(s.me.ID, evt)
		case event.EventEncrypted:
			ev = undecryptable(evt) // shows as waiting for its keys
			locked = true
		default:
			continue
		}
		if locked {
			stillLocked = true
		}
		s.post(func() {
			if r, ok := s.rooms[room]; ok {
				if r.pinEvents == nil {
					r.pinEvents = map[string]model.Event{}
				}
				r.pinEvents[ev.ID] = ev
				s.markDirty(room)
			}
		})
	}
}

// PinnedEvents are a room's pinned messages in the order they were pinned:
// as the timeline holds them (with their edits and reactions), or as they
// were fetched. Pins not fetched yet are left out until they arrive. UI
// thread.
func (s *Source) PinnedEvents(room string) []model.Event {
	r, ok := s.rooms[room]
	if !ok || len(r.pinned) == 0 {
		return nil
	}
	var out []model.Event
	for _, pid := range r.pinned {
		if i := r.indexOf(pid); i >= 0 {
			out = append(out, r.events[i])
		} else if e, ok := r.pinEvents[pid]; ok {
			out = append(out, e)
		}
	}
	return out
}

var _ model.Pins = (*Source)(nil)

package matrix

import (
	"context"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// Whether the user may write in a room is the room's power levels
// (m.room.power_levels): a message is an event of type m.room.message, sent at
// the level the room sets for it (events["m.room.message"], else events_default,
// 0 unless the room says), by a user whose level is at least that. An
// announcements room sets events_default high. The levels come with the sync
// as they change, and from the server once when the room is first shown (a
// cache resume does not repeat state); the server has the last word, and a
// refusal on sending is said as the message failing.

// canPost reports whether pl lets user send a message. Pure, for the tests.
func canPost(pl *event.PowerLevelsEventContent, user string) bool {
	if pl == nil {
		return true // not known: the server decides
	}
	return pl.GetUserLevel(id.UserID(user)) >= pl.GetEventLevel(event.EventMessage)
}

// onPowerLevels is a room's power levels changing.
func (s *Source) onPowerLevels(ctx context.Context, evt *event.Event) {
	pl := evt.Content.AsPowerLevels()
	room := evt.RoomID.String()
	s.post(func() { s.setPower(room, pl) })
}

func (s *Source) setPower(room string, pl *event.PowerLevelsEventContent) {
	r := s.roomOf(room)
	r.powerAsked = true
	if cannot := !canPost(pl, s.me.ID); cannot != r.cannotPost {
		r.cannotPost = cannot
		s.changed()
	}
}

// askPower fetches a room's power levels the first time the room is shown.
func (s *Source) askPower(room string) {
	r, ok := s.rooms[room]
	if !ok || r.powerAsked || s.cli == nil {
		return
	}
	r.powerAsked = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var pl event.PowerLevelsEventContent
		if err := s.cli.StateEvent(ctx, id.RoomID(room), event.StatePowerLevels, "", &pl); err != nil {
			s.post(func() { r.powerAsked = false }) // asked again next time
			return
		}
		s.post(func() { s.setPower(room, &pl) })
	}()
}

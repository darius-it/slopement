package mock

import (
	"strconv"

	"slopement/model"
)

// Forward puts a copy of an event in another room's timeline, from the user.
func (s *Source) Forward(room, id, to string, done func(err string)) {
	if _, ok := s.rooms[to]; !ok {
		done("That chat is not available.")
		return
	}
	for _, e := range s.timeline[room] {
		if e.ID != id {
			continue
		}
		s.seq++
		c := e
		c.ID, c.Room, c.Sender, c.Time = "$fwd"+strconv.Itoa(s.seq), to, s.me.ID, s.now()
		c.ReplyTo, c.ThreadRoot, c.Thread, c.Reactions, c.Edited, c.ReadBy = "", "", nil, nil, false, nil
		s.timeline[to] = append(s.timeline[to], c)
		for _, fn := range s.changed {
			fn()
		}
		done("")
		return
	}
	done("That message is not held.")
}

var _ model.Forwarder = (*Source)(nil)

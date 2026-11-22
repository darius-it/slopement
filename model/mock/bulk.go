package mock

import (
	"strconv"
	"time"

	"slopement/model"
)

// AddBulk adds a big account to the mock, for measuring: chats DMs and groups,
// spaces communities of perSpace rooms each, and events messages in the room
// "!crit" — about what a heavy user's list looks like. It tells the listeners
// that the account changed.
func (s *Source) AddBulk(chats, spaces, perSpace, events int) {
	now := s.now()
	for i := 0; i < chats; i++ {
		n := strconv.Itoa(i)
		partner := "@bulk" + n + ":matrix.org"
		s.people[partner] = &model.Person{ID: partner, Name: "Person " + n, Contact: true, Presence: model.Online}
		s.rooms["!bulk-dm"+n] = &model.Room{ID: "!bulk-dm" + n, Kind: model.KindDM, Name: "Person " + n, Partner: partner,
			Members: 2, Encrypted: i%2 == 0, Unread: i % 5, LastActivity: now.Add(-time.Duration(i) * time.Minute),
			Preview: "message number " + n}
	}
	for sp := 0; sp < spaces; sp++ {
		sn := strconv.Itoa(sp)
		cm := model.Community{ID: "!bulk-space" + sn, Name: "Community " + sn, Members: 50 + sp, Online: 5}
		for j := 0; j < perSpace; j++ {
			id := "!bulk-r" + sn + "-" + strconv.Itoa(j)
			s.rooms[id] = &model.Room{ID: id, Kind: model.KindRoom, Name: "room-" + sn + "-" + strconv.Itoa(j), Members: 20,
				Community: cm.ID, Unread: j % 3, LastActivity: now.Add(-time.Duration(sp*perSpace+j) * time.Minute),
				Preview: "hello from " + id}
			cm.Rooms = append(cm.Rooms, id)
		}
		s.comms = append(s.comms, cm)
	}
	for i := 0; i < events; i++ {
		who := "@bulk" + strconv.Itoa(i%7) + ":matrix.org"
		if i%7 == 0 {
			who = s.me.ID
		}
		s.timeline["!crit"] = append(s.timeline["!crit"], model.Event{
			ID: "$bulk" + strconv.Itoa(i), Room: "!crit", Sender: who, Time: now.Add(-time.Duration(events-i) * time.Minute),
			Body: "message " + strconv.Itoa(i) + " with **some** words in it and a link https://example.org/" + strconv.Itoa(i),
		})
	}
	for _, fn := range s.changed {
		fn()
	}
}

package matrix

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// RoomMembers are the people joined to a room, by name. The first ask starts
// fetching them from the server (the sync only carries the members who spoke:
// it is lazy-loaded), and answers with what is known meanwhile; the list is
// fetched again after anyone joins or leaves. UI thread.
func (s *Source) RoomMembers(room string) []model.Person {
	r, ok := s.rooms[room]
	if !ok {
		return nil
	}
	if !r.membersAsked && s.cli != nil {
		r.membersAsked = true
		go s.fetchMembers(room)
	}
	return r.memberList
}

func (s *Source) fetchMembers(room string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := s.cli.Members(ctx, id.RoomID(room), mautrix.ReqMembers{Membership: event.MembershipJoin})
	if err != nil {
		debugf("members %s: %v", room, err)
		s.post(func() {
			if r, ok := s.rooms[room]; ok {
				r.membersAsked = false // asked again the next time the panel is open
			}
		})
		return
	}
	type member struct{ id, name, avatar string }
	var got []member
	for _, evt := range resp.Chunk {
		if evt.StateKey == nil {
			continue
		}
		evt.Type.Class = event.StateEventType // these are state events, which ParseRaw reads by class
		if err := evt.Content.ParseRaw(evt.Type); err != nil && !errors.Is(err, event.ErrContentAlreadyParsed) {
			debugf("members %s: %s: %v", room, *evt.StateKey, err)
			continue
		}
		c := evt.Content.AsMember()
		if c == nil || c.Membership != event.MembershipJoin {
			continue
		}
		got = append(got, member{*evt.StateKey, nameFor(c.Displayname, *evt.StateKey), string(c.AvatarURL)})
	}
	debugf("members %s: %d of %d events are joined members", room, len(got), len(resp.Chunk))
	s.post(func() {
		r, ok := s.rooms[room]
		if !ok {
			return
		}
		list := make([]model.Person, 0, len(got))
		for _, m := range got {
			p := model.Person{ID: m.id}
			if known, ok := s.people[m.id]; ok { // what the app knows of them: presence, status, trust
				p = *known
			} else if m.id == s.me.ID {
				p = s.me
			}
			p.Name, p.Avatar = m.name, m.avatar // as this room's member event has them
			list = append(list, p)
		}
		sortMembers(list)
		r.memberList = list
		r.members = len(list) // joined, as the server lists them
	})
}

// sortMembers puts people in an order that does not change between frames:
// by name, ignoring case, then by ID.
func sortMembers(list []model.Person) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if a != b {
			return a < b
		}
		return list[i].ID < list[j].ID
	})
}

var _ model.RoomMembers = (*Source)(nil)

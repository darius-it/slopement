package mock

import (
	"sort"
	"strings"

	"slopement/model"
)

// RoomMembers are the mock's members of a room: the user and, in a chat with
// one person, that person; elsewhere everyone known — by name, so the order
// does not change between frames.
func (s *Source) RoomMembers(room string) []model.Person {
	out := []model.Person{s.me}
	if r, ok := s.rooms[room]; ok && r.Kind == model.KindDM {
		if p, ok := s.people[r.Partner]; ok {
			out = append(out, *p)
		}
	} else {
		for _, p := range s.people {
			out = append(out, *p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

var _ model.RoomMembers = (*Source)(nil)

package mock

import "slopement/model"

// PersonNotes is the nicknames and notes given in this run.
func (s *Source) PersonNotes() map[string]model.PersonNote { return s.notes }

// SetPersonNote gives someone a nickname and a note.
func (s *Source) SetPersonNote(mxid string, n model.PersonNote) {
	next := make(map[string]model.PersonNote, len(s.notes)+1)
	for k, v := range s.notes {
		next[k] = v
	}
	if n.Nick == "" && n.Note == "" {
		delete(next, mxid)
	} else {
		next[mxid] = n
	}
	s.notes = next
	for _, fn := range s.changed {
		fn()
	}
}

var _ model.PrivateNotes = (*Source)(nil)

// RoomNicknames is the private room names given in this run.
func (s *Source) RoomNicknames() map[string]string { return s.roomNick }

// SetRoomNickname gives a room a private name ("" takes it away).
func (s *Source) SetRoomNickname(room, name string) {
	next := make(map[string]string, len(s.roomNick)+1)
	for k, v := range s.roomNick {
		next[k] = v
	}
	if name == "" {
		delete(next, room)
	} else {
		next[room] = name
	}
	s.roomNick = next
	for _, fn := range s.changed {
		fn()
	}
}

var _ model.PrivateRoomNames = (*Source)(nil)

// ReactCustom reacts with a custom emoji: its URL is the key.
func (s *Source) ReactCustom(room, event, url, shortcode string) { s.React(room, event, url) }

var _ model.CustomReactor = (*Source)(nil)

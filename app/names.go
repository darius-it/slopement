package app

import (
	"strings"

	"slopement/model"
)

// Names. People are shown by their display names, as Element and other
// clients do. Settings → Appearance can switch to usernames (the part of the
// Matrix ID before the colon, "ada" of "@ada:example.org"): in the chat list,
// the chats, quotes, People and read receipts.

// usernameOf is a Matrix ID's username: "@ada:example.org" is "ada".
func usernameOf(mxid string) string {
	local, _, _ := strings.Cut(strings.TrimPrefix(mxid, "@"), ":")
	return local
}

// person is who an ID is, named as the user chose to see names. (The profile
// card keeps the display name: it shows the ID beside it.)
func (a *App) person(id string) (model.Person, bool) {
	p, ok := a.src.Person(id)
	if a.prefs.usernames && id != "" {
		p.Name = usernameOf(id)
	}
	return p, ok
}

// shownName is the name a person is shown by.
func (a *App) shownName(p model.Person) string {
	if a.prefs.usernames && p.ID != "" {
		return usernameOf(p.ID)
	}
	return p.Name
}

// namedRoom is a room as the lists show it: a chat with one person is named
// after them, so it follows the choice of names too.
func (a *App) namedRoom(r model.Room) model.Room {
	if a.prefs.usernames && r.Kind == model.KindDM && r.Partner != "" {
		r.Name = usernameOf(r.Partner)
	}
	return r
}

// roomByID is a room by its ID, named as the user chose to see names.
func (a *App) roomByID(id string) (model.Room, bool) {
	r, ok := a.src.Room(id)
	return a.namedRoom(r), ok
}

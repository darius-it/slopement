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
	p.Name = a.nameFor(id, p.Name)
	return p, ok
}

// nameFor is the name a person is shown by: the nickname the user privately
// gave them, else their username or display name, as chosen in Settings.
func (a *App) nameFor(id, display string) string {
	if n := a.noteOf(id); n.Nick != "" {
		return n.Nick
	}
	if a.prefs.usernames && id != "" {
		return usernameOf(id)
	}
	return display
}

// shownName is the name a person is shown by.
func (a *App) shownName(p model.Person) string { return a.nameFor(p.ID, p.Name) }

// noteOf is the nickname and note the user gave someone.
func (a *App) noteOf(id string) model.PersonNote {
	if pn, ok := a.src.(model.PrivateNotes); ok {
		return pn.PersonNotes()[id]
	}
	return model.PersonNote{}
}

// namesMatter says names are not shown as the source gives them (usernames, or
// some nickname), so a DM must be renamed.
func (a *App) namesMatter() bool {
	if a.prefs.usernames {
		return true
	}
	if pn, ok := a.src.(model.PrivateNotes); ok {
		for _, n := range pn.PersonNotes() {
			if n.Nick != "" {
				return true
			}
		}
	}
	return false
}

// namedRoom is a room as the lists show it: a chat with one person is named
// after them, so it follows the nickname and the choice of names too.
func (a *App) namedRoom(r model.Room) model.Room {
	if r.Kind == model.KindDM && r.Partner != "" {
		r.Name = a.nameFor(r.Partner, r.Name)
	}
	return r
}

// roomByID is a room by its ID, named as the user chose to see names.
func (a *App) roomByID(id string) (model.Room, bool) {
	r, ok := a.src.Room(id)
	return a.namedRoom(r), ok
}

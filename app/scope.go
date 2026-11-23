package app

import "slices"

// Each list — Chats, and every community — remembers the room last open in it.
// Going back to a list brings that room back (and the keyboard to its
// composer); with none, the desktop shows an empty "Select a chat" pane and
// the phone shows the list itself.

// inScope says whether a room belongs to a list: Chats holds the chats that
// are in no community, a community holds its rooms (sub-spaces' too).
func (a *App) inScope(room, community string) bool {
	if room == "" {
		return false
	}
	if community == "" {
		for _, r := range a.chats() {
			if r.ID == room {
				return true
			}
		}
		return false
	}
	cm, ok := a.communityOf(community)
	return ok && slices.Contains(a.communityRoomIDs(cm, 0), room)
}

// scopeSync keeps the memory and applies it when the list changes. Each frame,
// before navTrack.
func (a *App) scopeSync() {
	if !a.scopeInit {
		a.scope, a.scopeInit = a.community, true
	}
	if a.scope == a.community {
		if a.view == viewRoom && a.room != "" && a.scopeRooms[a.scope] != a.room && a.inScope(a.room, a.scope) {
			if a.scopeRooms == nil {
				a.scopeRooms = map[string]string{}
			}
			a.scopeRooms[a.scope] = a.room
		}
		return
	}
	a.scope = a.community
	if a.restoreRoom != "" || a.restoreCommunity != "" { // the last run's place is still being put back
		return
	}
	if a.room != "" && a.inScope(a.room, a.scope) { // e.g. a room chosen in the switcher
		return
	}
	if a.narrow { // phone: the list itself, not a chat
		a.pushed = false
		return
	}
	if last := a.scopeRooms[a.scope]; a.prefs.jumpLast && a.inScope(last, a.scope) {
		a.open(last) // focus included
		return
	}
	a.stopTyping(a.room)
	a.room = ""
}

package app

import (
	"slopement/kit"
	"slopement/model"
)

// Two small caches keep a frame from redoing the work of the source's lists.
//
// srcCache is what the source's lists came to, kept while the source has not
// changed (its OnChange counts the changes) and for at most half a minute (the
// lists show times). A frame asks for the rooms, the communities and who is
// online many times over — every list, count and lookup — and each is a copy
// and a sort of all of them: in an account of hundreds of chats that was most
// of every frame, typing included.
//
// frameCache is what is built from them with pictures and statuses in it,
// which change without the source changing (a picture arrives): kept for one
// frame only.
type srcCache struct {
	set      bool
	gen      uint64
	bucket   int64
	rooms    []model.Room
	roomsOK  bool
	comms    []model.Community
	commsOK  bool
	inbox    *inboxData
	names    bool   // the lists were made with usernames shown (names.go)
	online   [2]int // by "all"
	onlineOK [2]bool
}

type frameCache struct {
	on         bool
	contacts   [2][]kit.Person // by "all"
	contactsOK [2]bool
	hidden     [2]int
}

// cacheBucket is how long a cached list may be kept: half a minute.
func (a *App) cacheBucket() int64 { return a.now.Unix() / 30 }

// frameStart begins a frame: its own cache is empty, and the source's is kept
// if the source has not changed since.
func (a *App) frameStart() {
	a.fc = frameCache{on: true}
	if b := a.cacheBucket(); !a.scValid() || a.sc.bucket != b {
		a.sc = srcCache{set: true, gen: a.srcGen, bucket: b, names: a.prefs.usernames}
	}
}

func (a *App) frameEnd() { a.fc = frameCache{} }

// scValid says the source's cache is for the source as it is now.
func (a *App) scValid() bool {
	return a.sc.set && a.sc.gen == a.srcGen && a.sc.names == a.prefs.usernames
}

func (a *App) scEnsure() {
	if !a.scValid() {
		a.sc = srcCache{set: true, gen: a.srcGen, bucket: a.cacheBucket(), names: a.prefs.usernames}
	}
}

// roomList is the source's rooms (most recent first), kept while it does not
// change. Callers must not change the slice.
func (a *App) roomList() []model.Room {
	a.scEnsure()
	if !a.sc.roomsOK {
		rooms := a.src.Rooms()
		if a.prefs.usernames { // chats with a person are named after them, by username
			named := make([]model.Room, len(rooms))
			for i, r := range rooms {
				named[i] = a.namedRoom(r)
			}
			rooms = named
		}
		a.sc.rooms, a.sc.roomsOK = rooms, true
	}
	return a.sc.rooms
}

// communityList is the source's communities, kept while it does not change.
func (a *App) communityList() []model.Community {
	a.scEnsure()
	if !a.sc.commsOK {
		a.sc.comms, a.sc.commsOK = a.src.Communities(), true
	}
	return a.sc.comms
}

// onlineContacts counts the contacts People lists that are online, without
// building a card for each (a big account has thousands of people).
func (a *App) onlineContacts() int {
	a.scEnsure()
	i := 0
	if a.peopleAll {
		i = 1
	}
	if a.sc.onlineOK[i] {
		return a.sc.online[i]
	}
	n := 0
	for _, p := range a.src.People() {
		if p.Contact && (p.Presence == model.Online || p.Presence == model.Away) && (a.peopleAll || !a.outsideHome(p.ID)) {
			n++
		}
	}
	a.sc.online[i], a.sc.onlineOK[i] = n, true
	return n
}

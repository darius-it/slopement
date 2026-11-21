package matrix

import (
	"sort"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"

	"slopement/model"
)

// Source is one signed-in account's data for the app. Before Start it
// knows who is signed in and nothing else; the sync loop (sync.go)
// fills in the rooms, the timeline and the people, changing everything
// through SetUpdate's function, on the UI thread alone.
type Source struct {
	me       model.Person
	rooms    map[string]*room
	direct   map[string]bool // the DM rooms (m.direct)
	people   map[string]*model.Person
	update   func(func())
	onChange []func()

	// The wiring Start needs; nil on a Source built by NewSource alone.
	cli     *mautrix.Client
	acc     *Account
	helper  *cryptohelper.CryptoHelper
	started bool
}

// NewSource is the app's view of a signed-in account, before any sync.
func NewSource(s *Session) *Source {
	return &Source{
		me:     model.Person{ID: string(s.UserID), Name: mxidName(string(s.UserID))},
		rooms:  map[string]*room{},
		direct: map[string]bool{},
		people: map[string]*model.Person{},
	}
}

// mxidName is the local part of an MXID, until the profile (display
// name, avatar) arrives with the sync.
func mxidName(mxid string) string {
	local, _, _ := strings.Cut(strings.TrimPrefix(mxid, "@"), ":")
	return local
}

// SetUpdate gives the source the window's update function: work done
// off the UI thread comes back through it.
func (s *Source) SetUpdate(update func(func())) { s.update = update }

// OnChange is called (on the UI thread) after anything changed.
func (s *Source) OnChange(fn func()) { s.onChange = append(s.onChange, fn) }

// changed runs what listens for changes on the UI thread, and redraws.
// Call it inside a SetUpdate function, never from a goroutine.
func (s *Source) changed() {
	for _, fn := range s.onChange {
		fn()
	}
}

// post runs fn on the UI thread if a window is there, and redraws.
func (s *Source) post(fn func()) {
	if s.update == nil {
		return
	}
	s.update(func() {
		fn()
		s.changed()
	})
}

// person returns who an MXID is, as the timeline shows them.
func (s *Source) person(mxid string) *model.Person {
	if p, ok := s.people[mxid]; ok {
		return p
	}
	return &model.Person{ID: mxid, Name: mxidName(mxid)}
}

func (s *Source) Me() model.Person { return s.me }

// modelRoom is the room as the lists show it.
func (r *room) modelRoom() model.Room {
	m := model.Room{
		ID:           r.id,
		Kind:         model.KindGroup,
		Name:         r.name,
		Topic:        r.topic,
		Members:      r.members,
		Encrypted:    r.encrypted,
		Unread:       r.unread,
		Mentions:     r.mentions,
		Favourite:    r.favourite,
		LowPriority:  r.lowPriority,
		Muted:        r.muted,
		LastActivity: r.last,
		Preview:      r.preview,
		Typing:       r.typing,
	}
	if r.dm {
		m.Kind, m.Partner = model.KindDM, r.partner
	} else if r.members > 20 {
		m.Kind = model.KindRoom
	}
	return m
}

// Rooms lists the joined rooms, the most recently active first, as the
// lists show them.
func (s *Source) Rooms() []model.Room {
	out := make([]model.Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		out = append(out, r.modelRoom())
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActivity.After(out[j].LastActivity) })
	return out
}

func (s *Source) Room(id string) (model.Room, bool) {
	r, ok := s.rooms[id]
	if !ok {
		return model.Room{}, false
	}
	return r.modelRoom(), true
}

// Communities is nothing yet: spaces come with a later milestone.
func (s *Source) Communities() []model.Community { return nil }

func (s *Source) People() []model.Person {
	out := make([]model.Person, 0, len(s.people))
	for _, p := range s.people {
		out = append(out, *p)
	}
	return out
}

func (s *Source) Person(id string) (model.Person, bool) {
	if id == s.me.ID {
		return s.me, true
	}
	if p, ok := s.people[id]; ok {
		return *p, true
	}
	return model.Person{ID: id, Name: mxidName(id)}, id == ""
}

func (s *Source) Timeline(room string) []model.Event {
	if r, ok := s.rooms[room]; ok {
		return r.timeline(s.me.ID)
	}
	return nil
}

func (s *Source) Sessions() []model.Session { return nil }

// The writes that mean something before the later milestones: sending,
// reacting, reading. The rest are quiet no-ops, marked for their own
// milestones.

func (s *Source) SetStatus(model.Status)              {}
func (s *Source) SetFavourite(string, bool)           {}
func (s *Source) SetCommunityFavourite(string, bool)  {}
func (s *Source) SetLowPriority(string, bool)         {}
func (s *Source) Mute(string, time.Time)              {}
func (s *Source) Unmute(string)                       {}
func (s *Source) MarkUnread(string)                   {} // m.fully_read account data, a later milestone
func (s *Source) CreatePoll(string, model.Poll)       {}
func (s *Source) Vote(room, event string, answer int) {}
func (s *Source) EndPoll(room, event string)          {}
func (s *Source) Leave(string)                        {}
func (s *Source) OpenDM(person string) string         { return "" }

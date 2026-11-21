package matrix

import (
	"sort"
	"strings"
	"sync"
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
	me     model.Person
	rooms  map[string]*room
	direct map[string]bool // the DM rooms (m.direct)
	people map[string]*model.Person
	// encrypted counts the messages still waiting for keys: the verify
	// prompt shows while any are there and the session is unverified.
	encrypted int
	update    func(func())
	onChange  []func()

	// The wiring Start needs; nil on a Source built by NewSource alone.
	cli     *mautrix.Client
	acc     *Account
	helper  *cryptohelper.CryptoHelper
	verify  *VerifyDriver
	started bool

	// mu guards the batch: posts made while a sync response is processed
	// are queued and applied together.
	mu       sync.Mutex
	batching  int
	queue     []func()
	pendToken string   // the sync token of a batch that ended inside another
	pendLeft  []string // and the rooms it left

	// keyKick wakes the key watcher when a message arrives locked.
	keyKick chan struct{}

	// The room cache (persist.go): the rooms changed since the last
	// write, the rooms gone, the sync token the state is at, and whether
	// a write is already scheduled. UI thread alone.
	cache          *Cache
	token          string
	dirty          map[string]bool
	gone           []string
	dirtyMeta      bool
	tokenDirty     bool
	persistPending bool
	// resumed: this run started from the cache's token (a token the
	// server refuses then means the cache is dropped and the sync starts
	// over).
	resumed bool

	// publicReceipts: the read receipts sent are public (SetPublicReceipts).
	publicReceipts bool
}

// NewSource is the app's view of a signed-in account, before any sync.
func NewSource(s *Session) *Source {
	return &Source{
		me:     model.Person{ID: string(s.UserID), Name: mxidName(string(s.UserID))},
		rooms:  map[string]*room{},
		direct: map[string]bool{},
		people: map[string]*model.Person{},

		keyKick: make(chan struct{}, 1),
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
	s.mu.Lock()
	if s.batching > 0 {
		s.queue = append(s.queue, fn)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.update(func() {
		fn()
		s.changed()
	})
}

// beginBatch starts collecting posts: one sync response is dozens of
// events, and the UI must see it whole — or a room shows in Chats for a
// frame before its space and parent claims arrive, then moves.
//
// Batches nest (a page of history loaded while a sync is processed): what
// is collected runs when the last one ends.
func (s *Source) beginBatch() {
	s.mu.Lock()
	s.batching++
	s.mu.Unlock()
}

// endBatch runs everything collected as one UI update, one redraw.
//
// token is the sync token this response continues to: the state is at it
// once the queue has run, so the cache writes them together. left are the
// rooms the user is no longer in: they go once the queue has run (their
// last events would otherwise bring them back).
func (s *Source) endBatch(token string, left []string) {
	s.mu.Lock()
	if token != "" {
		s.pendToken = token
	}
	s.pendLeft = append(s.pendLeft, left...)
	s.batching--
	if s.batching > 0 {
		s.mu.Unlock()
		return // an outer batch is still collecting
	}
	q, token, left := s.queue, s.pendToken, s.pendLeft
	s.queue, s.pendToken, s.pendLeft = nil, "", nil
	s.mu.Unlock()
	if s.update == nil {
		return
	}
	s.update(func() {
		for _, fn := range q {
			fn()
		}
		for _, id := range left {
			if _, ok := s.rooms[id]; ok {
				delete(s.rooms, id)
				s.gone = append(s.gone, id)
				delete(s.dirty, id)
				s.dirtyMeta = true
			}
		}
		if token != "" {
			s.token, s.tokenDirty = token, true
		}
		s.schedulePersist()
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
		Avatar:       r.avatar,
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
// lists show them, each with the space it belongs to.
func (s *Source) Rooms() []model.Room {
	spaceOf := s.spaceOf()
	out := make([]model.Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		if r.space {
			continue // a space is a community on the rail, never a chat
		}
		m := r.modelRoom()
		m.Community = spaceOf[r.id]
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActivity.After(out[j].LastActivity) })
	return out
}

func (s *Source) Room(id string) (model.Room, bool) {
	r, ok := s.rooms[id]
	if !ok {
		return model.Room{}, false
	}
	m := r.modelRoom()
	m.Community = s.spaceOf()[id]
	return m, true
}

// Communities lists the spaces (the joined rooms of type m.space) by
// name, with the rooms each one lists.
func (s *Source) Communities() []model.Community {
	var out []model.Community
	spaceOf := s.spaceOf()
	for _, r := range s.rooms {
		if !r.space {
			continue
		}
		cm := model.Community{
			ID: r.id, Name: r.name, About: r.topic, Avatar: r.avatar,
			Members: r.members, Favourite: r.favourite,
			Rooms: s.childIDs(r),
		}
		if p := spaceOf[r.id]; p != "" && p != r.id {
			cm.Parent = p
		}
		for _, ch := range r.children {
			if cr, ok := s.rooms[ch.room]; ok && cr.space && spaceOf[ch.room] == r.id {
				cm.Spaces = append(cm.Spaces, ch.room)
			}
		}
		out = append(out, cm)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// childIDs is a space's rooms, in its own order — the sub-spaces among
// them stay out: they are communities themselves, on the rail, not rooms
// in a list.
func (s *Source) childIDs(r *room) []string {
	var out []string
	for _, ch := range r.children {
		if cr, ok := s.rooms[ch.room]; ok && cr.space {
			continue
		}
		out = append(out, ch.room)
	}
	return out
}

// spaceOf maps each room to the space it belongs to, deterministically —
// the same answer every call, or the lists would flicker. A room's own
// m.space.parent claim wins; otherwise the first space by ID that lists
// it, and only a joined space's claim counts (an unjoined space must not
// take a room out of Chats and give it nowhere to go).
func (s *Source) spaceOf() map[string]string {
	out := map[string]string{}
	var spaceIDs []string
	byID := map[string]*room{}
	for id, r := range s.rooms {
		if r.space {
			spaceIDs = append(spaceIDs, id)
			byID[id] = r
		}
	}
	sort.Strings(spaceIDs)
	for _, r := range s.rooms {
		if r.parent != "" {
			if _, ok := s.rooms[r.parent]; ok {
				out[r.id] = r.parent
			}
		}
	}
	for _, spaceID := range spaceIDs {
		for _, ch := range byID[spaceID].children {
			if _, ok := out[ch.room]; !ok {
				out[ch.room] = spaceID
			}
		}
	}
	return out
}

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

// Verify is this device's verification: the emoji with another device,
// or the Security Key. Nil before sync.
func (s *Source) Verify() model.Verify {
	if s.verify == nil {
		return nil
	}
	return s.verify
}

// The writes that mean something before the later milestones: sending,
// reacting, reading. The rest are quiet no-ops, marked for their own
// milestones.

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

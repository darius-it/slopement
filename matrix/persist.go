package matrix

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// roomRecord is a room as the cache stores it.
type roomRecord struct {
	ID         string
	Name       string
	Topic      string
	Alias      string
	Avatar     string
	Partner    string
	DM         bool
	Encrypted  bool
	Space      bool
	Members    int
	HasSummary bool

	Unread, Mentions              int
	Favourite, LowPriority, Muted bool
	Last                          time.Time
	Preview                       string

	Events    []model.Event
	Reactions map[string]map[string]string
	Raw       map[string]rawRecord
	Children  []childRecord
	Parent    string
	Reads     map[string]readMark
	Edits     map[string]editMark
	Back      string
	BackDone  bool
	Pinned    []string
}

type rawRecord struct {
	Evt  json.RawMessage // the encrypted event as the server sent it
	Tile model.Event
}

type childRecord struct{ Room, Order string }

func (r *room) record() roomRecord {
	rec := roomRecord{
		ID: r.id, Name: r.name, Topic: r.topic, Alias: r.alias, Avatar: r.avatar, Partner: r.partner,
		DM: r.dm, Encrypted: r.encrypted, Space: r.space, Members: r.members, HasSummary: r.hasSummary,
		Unread: r.unread, Mentions: r.mentions,
		Favourite: r.favourite, LowPriority: r.lowPriority, Muted: r.muted,
		Last: r.last, Preview: r.preview, Parent: r.parent,
		Reactions: r.reactions, Reads: r.reads, Edits: r.edits, Pinned: r.pinned,
	}
	events := r.events
	if len(events) > timelineCap {
		events = events[len(events)-timelineCap:] // the cache holds the newest; history is paged back in
	}
	for _, e := range events {
		if strings.HasPrefix(e.ID, localPrefix) {
			continue // not sent yet: it dies with the process
		}
		rec.Events = append(rec.Events, e)
	}
	// Where history was: only worth keeping while the cache holds the events
	// the token was taken from (it is dropped when they are trimmed).
	if len(r.events) <= timelineCap {
		rec.Back, rec.BackDone = r.back, r.backDone
	}
	if len(r.raw) > 0 {
		rec.Raw = make(map[string]rawRecord, len(r.raw))
		for id, raw := range r.raw {
			if b, err := json.Marshal(raw.evt); err == nil {
				rec.Raw[id] = rawRecord{Evt: b, Tile: raw.tile}
			}
		}
	}
	for _, c := range r.children {
		rec.Children = append(rec.Children, childRecord{c.room, c.order})
	}
	return rec
}

func (rec roomRecord) room() *room {
	r := &room{
		id: rec.ID, name: rec.Name, topic: rec.Topic, alias: rec.Alias, avatar: rec.Avatar, partner: rec.Partner,
		dm: rec.DM, encrypted: rec.Encrypted, space: rec.Space, members: rec.Members, hasSummary: rec.HasSummary,
		unread: rec.Unread, mentions: rec.Mentions,
		favourite: rec.Favourite, lowPriority: rec.LowPriority, muted: rec.Muted,
		last: rec.Last, preview: rec.Preview, parent: rec.Parent,
		events: rec.Events, reactions: rec.Reactions, reads: rec.Reads, edits: rec.Edits, back: rec.Back, backDone: rec.BackDone, pinned: rec.Pinned,
	}
	for id, raw := range rec.Raw {
		var evt event.Event
		if json.Unmarshal(raw.Evt, &evt) != nil || evt.Content.ParseRaw(evt.Type) != nil {
			continue
		}
		if r.raw == nil {
			r.raw = map[string]*rawEvent{}
		}
		r.raw[id] = &rawEvent{evt: &evt, tile: raw.Tile}
	}
	for _, c := range rec.Children {
		r.children = append(r.children, spaceChild{room: c.Room, order: c.Order})
	}
	return r
}

// loadCache fills the Source from the cache and says whether the sync can
// resume from its token.
func (s *Source) loadCache(ctx context.Context) bool {
	if s.cache == nil || s.cli == nil {
		return false
	}
	snap, ok := s.cache.load(ctx, s.cli.DeviceID)
	if !ok {
		return false
	}
	rooms := make(map[string]*room, len(snap.rooms))
	for id, data := range snap.rooms {
		var rec roomRecord
		if json.Unmarshal(data, &rec) != nil {
			s.cache.reset(ctx) // unreadable: trust none of it
			return false
		}
		rooms[id] = rec.room()
	}
	var people map[string]model.Person
	_ = json.Unmarshal(snap.people, &people)
	var direct []string
	_ = json.Unmarshal(snap.direct, &direct)

	s.rooms = rooms
	for id, p := range people {
		p := p
		p.Presence = model.Offline // what it was is not what it is
		s.people[id] = &p
	}
	for _, id := range direct {
		s.direct[id] = true
	}
	if me, ok := s.people[s.me.ID]; ok { // the user's own name and picture, as last known
		s.setMe(me.Name, me.Avatar)
	}
	for _, r := range s.rooms {
		s.encrypted += len(r.raw)
	}
	s.token = snap.token
	return true
}

// markDirty says a room changed since it was last written. UI thread.
func (s *Source) markDirty(id string) {
	if s.cache == nil {
		return
	}
	if s.dirty == nil {
		s.dirty = map[string]bool{}
	}
	s.dirty[id] = true
	s.dirtyMeta = true
	s.schedulePersist()
}

// schedulePersist writes the changes a moment from now, once for all that
// happens meanwhile. UI thread.
func (s *Source) schedulePersist() {
	if s.cache == nil || s.update == nil || s.persistPending {
		return
	}
	s.persistPending = true
	time.AfterFunc(time.Second, func() { s.update(s.persistNow) })
}

// persistNow queues the changed rooms, with the token they continue from,
// for the writer. UI thread.
func (s *Source) persistNow() {
	s.persistPending = false
	if s.cache == nil || (len(s.dirty) == 0 && len(s.gone) == 0 && !s.dirtyMeta && !s.tokenDirty) {
		return
	}
	w := cacheWrite{token: s.token, rooms: map[string][]byte{}, meta: map[string][]byte{}}
	for _, id := range s.gone {
		if _, back := s.rooms[id]; !back { // a room joined again stays
			w.drop = append(w.drop, id)
		}
	}
	for id := range s.dirty {
		r, ok := s.rooms[id]
		if !ok {
			w.drop = append(w.drop, id)
			continue
		}
		if b, err := json.Marshal(r.record()); err == nil {
			w.rooms[id] = b
		}
	}
	if s.dirtyMeta || s.tokenDirty {
		people := make(map[string]model.Person, len(s.people))
		for id, p := range s.people {
			people[id] = *p
		}
		direct := make([]string, 0, len(s.direct))
		for id := range s.direct {
			direct = append(direct, id)
		}
		w.meta["people"], _ = json.Marshal(people)
		w.meta["direct"], _ = json.Marshal(direct)
		w.meta["device"] = []byte(s.cli.DeviceID)
	}
	s.dirty, s.gone, s.dirtyMeta, s.tokenDirty = nil, nil, false, false
	s.cache.write(w)
}

// Flush writes what is pending and waits for it to be on disk: call it
// on the UI thread when the window closes.
func (s *Source) Flush() {
	if s.cache == nil {
		return
	}
	s.persistNow()
	s.cache.flush()
}

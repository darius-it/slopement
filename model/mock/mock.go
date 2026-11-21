// Package mock is a model.Source of fake data, shaped like Matrix's, for
// building the app before the Matrix layer exists (and for tests). It
// answers like a small, lively account: people reply to DMs after typing
// for a moment.
package mock

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"slopement/model"
)

// Source is the mock. Build it with New.
type Source struct {
	now      func() time.Time
	update   func(func())
	changed  []func()
	me       model.Person
	people   map[string]*model.Person
	rooms    map[string]*model.Room
	comms    []model.Community
	timeline map[string][]model.Event
	sessions []model.Session
	seq      int
	// Delay scales the simulated replies (0 in tests: none).
	Delay time.Duration
}

var _ model.Source = (*Source)(nil)

// New returns the mock account at now.
func New(now time.Time) *Source {
	s := &Source{
		now:      time.Now,
		people:   map[string]*model.Person{},
		rooms:    map[string]*model.Room{},
		timeline: map[string][]model.Event{},
		Delay:    time.Second,
	}
	s.seed(now)
	return s
}

func (s *Source) SetUpdate(update func(func())) { s.update = update }
func (s *Source) OnChange(fn func())            { s.changed = append(s.changed, fn) }

func (s *Source) emit() {
	for _, fn := range s.changed {
		fn()
	}
}

// later runs fn on the UI thread after d (scaled by Delay); without an
// update function (tests), never.
func (s *Source) later(d time.Duration, fn func()) {
	if s.update == nil || s.Delay == 0 {
		return
	}
	time.AfterFunc(d*s.Delay/time.Second, func() {
		s.update(func() {
			fn()
			s.emit()
		})
	})
}

func (s *Source) id(prefix string) string {
	s.seq++
	return prefix + strconv.Itoa(s.seq)
}

// ---- Reading ---------------------------------------------------------------

func (s *Source) Me() model.Person { return s.me }

func (s *Source) Rooms() []model.Room {
	out := make([]model.Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastActivity.After(out[j].LastActivity) })
	return out
}

func (s *Source) Room(id string) (model.Room, bool) {
	r, ok := s.rooms[id]
	if !ok {
		return model.Room{}, false
	}
	return *r, true
}

func (s *Source) Communities() []model.Community { return s.comms }

func (s *Source) People() []model.Person {
	out := make([]model.Person, 0, len(s.people))
	for _, p := range s.people {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Source) Person(id string) (model.Person, bool) {
	if id == s.me.ID {
		return s.me, true
	}
	p, ok := s.people[id]
	if !ok {
		return model.Person{ID: id, Name: strings.TrimPrefix(strings.SplitN(id, ":", 2)[0], "@")}, false
	}
	return *p, true
}

func (s *Source) Timeline(room string) []model.Event { return s.timeline[room] }
func (s *Source) Sessions() []model.Session          { return s.sessions }

// ---- Acting ----------------------------------------------------------------

func (s *Source) Send(room, markdown, replyTo string) {
	r, ok := s.rooms[room]
	if !ok || strings.TrimSpace(markdown) == "" {
		return
	}
	// Pending until "sent" (at once without an update function: tests).
	ev := model.Event{ID: s.id("$e"), Room: room, Sender: s.me.ID, Time: s.now(), Body: markdown, ReplyTo: replyTo,
		Pending: s.update != nil && s.Delay > 0}
	s.timeline[room] = append(s.timeline[room], ev)
	r.LastActivity, r.Preview = ev.Time, "You: "+firstLine(markdown)
	s.emit()
	// Sent: the pending mark goes.
	s.later(400*time.Millisecond, func() { s.setPending(room, ev.ID, false) })
	// In a DM, they reply after typing a moment.
	if r.Kind == model.KindDM {
		who := s.people[r.Partner]
		s.later(1200*time.Millisecond, func() { r.Typing = []string{who.Name} })
		s.later(3200*time.Millisecond, func() {
			r.Typing = nil
			s.receive(room, who.ID, replyTo1(markdown))
		})
	}
}

func (s *Source) setPending(room, id string, on bool) {
	tl := s.timeline[room]
	for i := range tl {
		if tl[i].ID == id {
			tl[i].Pending = on
		}
	}
}

// receive adds a message from someone, as a sync would.
func (s *Source) receive(room, sender, body string) {
	r := s.rooms[room]
	ev := model.Event{ID: s.id("$e"), Room: room, Sender: sender, Time: s.now(), Body: body}
	s.timeline[room] = append(s.timeline[room], ev)
	name := s.people[sender].Name
	r.LastActivity, r.Preview = ev.Time, firstLine(body)
	if r.Kind != model.KindDM {
		r.Preview = name + ": " + r.Preview
	}
	r.Unread++
}

// replyTo1 is a canned answer.
func replyTo1(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "?"):
		return "good question, let me think 🤔"
	case strings.Contains(m, "lunch") || strings.Contains(m, "coffee"):
		return "yes please ☕"
	case strings.Contains(m, "thanks") || strings.Contains(m, "thank"):
		return "anytime!"
	}
	return []string{"haha", "sounds good 👍", "ok!", "on it", "nice"}[len(msg)%5]
}

func (s *Source) React(room, event, key string) {
	tl := s.timeline[room]
	for i := range tl {
		if tl[i].ID != event {
			continue
		}
		rs := tl[i].Reactions
		for j := range rs {
			if rs[j].Key == key {
				if rs[j].Mine {
					rs[j].Count--
				} else {
					rs[j].Count++
				}
				rs[j].Mine = !rs[j].Mine
				if rs[j].Count == 0 {
					rs = append(rs[:j], rs[j+1:]...)
				}
				tl[i].Reactions = rs
				s.emit()
				return
			}
		}
		tl[i].Reactions = append(rs, model.Reaction{Key: key, Count: 1, Mine: true})
	}
	s.emit()
}

func (s *Source) poll(room, event string) *model.Poll {
	for i, ev := range s.timeline[room] {
		if ev.ID == event {
			return s.timeline[room][i].Poll
		}
	}
	return nil
}

func (s *Source) Vote(room, event string, answer int) {
	p := s.poll(room, event)
	if p == nil || p.Ended || answer < 0 || answer >= len(p.Answers) {
		return
	}
	mine := func(i int) int {
		for j, v := range p.Answers[i].Voters {
			if v == s.me.ID {
				return j
			}
		}
		return -1
	}
	if j := mine(answer); j >= 0 { // take it back
		p.Answers[answer].Voters = append(p.Answers[answer].Voters[:j], p.Answers[answer].Voters[j+1:]...)
		s.emit()
		return
	}
	if !p.Multiple {
		for i := range p.Answers {
			if j := mine(i); j >= 0 {
				p.Answers[i].Voters = append(p.Answers[i].Voters[:j], p.Answers[i].Voters[j+1:]...)
			}
		}
	}
	p.Answers[answer].Voters = append(p.Answers[answer].Voters, s.me.ID)
	s.emit()
}

func (s *Source) EndPoll(room, event string) {
	if p := s.poll(room, event); p != nil {
		p.Ended = true
		s.emit()
	}
}

func (s *Source) CreatePoll(room string, p model.Poll) {
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	ev := model.Event{ID: s.id("$p"), Room: room, Sender: s.me.ID, Time: s.now(), Kind: model.EventPoll, Poll: &p}
	s.timeline[room] = append(s.timeline[room], ev)
	r.LastActivity, r.Preview = ev.Time, "You: 📊 "+p.Question
	s.emit()
}

func (s *Source) SetStatus(st model.Status) { s.me.Status = st; s.emit() }

func (s *Source) with(room string, fn func(r *model.Room)) {
	if r, ok := s.rooms[room]; ok {
		fn(r)
		s.emit()
	}
}

func (s *Source) SetFavourite(room string, on bool) {
	s.with(room, func(r *model.Room) {
		r.Favourite = on
		if on {
			r.LowPriority = false
		}
	})
}

// SetCommunityFavourite stars a community, or unstars it.
func (s *Source) SetCommunityFavourite(community string, on bool) {
	for i := range s.comms {
		if s.comms[i].ID == community {
			s.comms[i].Favourite = on
			s.emit()
			return
		}
	}
}
func (s *Source) SetLowPriority(room string, on bool) {
	s.with(room, func(r *model.Room) {
		r.LowPriority = on
		if on {
			r.Favourite = false
		}
	})
}
func (s *Source) Mute(room string, until time.Time) {
	s.with(room, func(r *model.Room) { r.Muted, r.MutedUntil = true, until })
}
func (s *Source) Unmute(room string) {
	s.with(room, func(r *model.Room) { r.Muted, r.MutedUntil = false, time.Time{} })
}
func (s *Source) MarkRead(room string) {
	r, ok := s.rooms[room]
	if !ok || (r.Unread == 0 && r.Mentions == 0) {
		return // nothing to do: no change, no frame
	}
	r.Unread, r.Mentions = 0, 0
	s.emit()
}
func (s *Source) MarkUnread(room string) {
	s.with(room, func(r *model.Room) {
		if r.Unread == 0 {
			r.Unread = 1
		}
	})
}
func (s *Source) Leave(room string) {
	if _, ok := s.rooms[room]; !ok {
		return
	}
	delete(s.rooms, room)
	for i := range s.comms {
		rs := s.comms[i].Rooms
		for j, id := range rs {
			if id == room {
				s.comms[i].Rooms = append(rs[:j:j], rs[j+1:]...)
				break
			}
		}
	}
	s.emit()
}

func (s *Source) OpenDM(person string) string {
	for id, r := range s.rooms {
		if r.Kind == model.KindDM && r.Partner == person {
			return id
		}
	}
	p, _ := s.Person(person)
	id := s.id("!dm")
	s.rooms[id] = &model.Room{ID: id, Kind: model.KindDM, Name: p.Name, Partner: person, Members: 2, Encrypted: true,
		LastActivity: s.now()}
	if pp, ok := s.people[person]; ok {
		pp.Contact = true
	}
	s.emit()
	return id
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

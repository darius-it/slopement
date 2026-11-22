package matrix

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// Polls (MSC3381): a poll is a start event; each vote is a response event
// that names it; the poll ends with an end event from whoever started it.
// Both the unstable names (org.matrix.msc3381.*, which Element writes) and
// the stable ones (m.poll.*) are read; polls are written in the unstable
// form, which every client that has polls reads.
//
// The tally is not stored on the poll's event: votes and the end are kept
// apart (pollState, in the cache too, since a resumed sync never repeats
// them) and the event shows what they add up to.

func msgType(t string) event.Type { return event.Type{Type: t, Class: event.MessageEventType} }

var (
	pollStartTypes    = []event.Type{msgType("org.matrix.msc3381.poll.start"), msgType("m.poll.start")}
	pollResponseTypes = []event.Type{msgType("org.matrix.msc3381.poll.response"), msgType("m.poll.response")}
	pollEndTypes      = []event.Type{msgType("org.matrix.msc3381.poll.end"), msgType("m.poll.end")}
	mutesType         = event.Type{Type: mutesData, Class: event.AccountDataEventType}
)

// isPollType says an event type is one of a poll's (to take it through
// history pages as the live ones).
func isPollType(t event.Type) bool {
	for _, set := range [][]event.Type{pollStartTypes, pollResponseTypes, pollEndTypes} {
		for _, p := range set {
			if p.Type == t.Type {
				return true
			}
		}
	}
	return false
}

// pollState is what a poll has gathered: each person's latest vote, how
// many answers it may pick, and the end if it has one.
type pollState struct {
	Votes map[string]pollVote
	Max   int
	// Ends is when each person sent an end, earliest first kept: only the
	// poll's starter's counts, and who that is may not be known until its
	// start event is (history comes newest first).
	Ends map[string]int64
}

type pollVote struct {
	TS      int64
	Answers []string
}

func (r *room) pollOf(eventID string) *pollState {
	if r.polls == nil {
		r.polls = map[string]*pollState{}
	}
	st := r.polls[eventID]
	if st == nil {
		st = &pollState{Votes: map[string]pollVote{}}
		r.polls[eventID] = st
	}
	if st.Votes == nil {
		st.Votes = map[string]pollVote{}
	}
	return st
}

// refreshPoll recounts a poll's event from its state: who voted for what
// (only the latest vote of each person, only valid answers, none after the
// end), and whether it has ended (only its starter can end it).
func (r *room) refreshPoll(eventID string) {
	i := r.indexOf(eventID)
	if i < 0 || r.events[i].Poll == nil {
		return
	}
	ev := r.events[i]
	st := r.pollOf(eventID)
	p := *ev.Poll
	p.Answers = make([]model.PollAnswer, len(ev.Poll.Answers))
	valid := map[string]int{}
	for k, a := range ev.Poll.Answers {
		p.Answers[k] = model.PollAnswer{ID: a.ID, Text: a.Text}
		valid[a.ID] = k
	}
	endTS, ended := st.Ends[ev.Sender] // only the starter's end ends it
	p.Ended = ended
	people := make([]string, 0, len(st.Votes))
	for who := range st.Votes {
		people = append(people, who)
	}
	slices.Sort(people) // a stable order of voters
	for _, who := range people {
		v := st.Votes[who]
		if p.Ended && v.TS > endTS {
			continue // after the end: not counted
		}
		max := max(st.Max, 1)
		if len(v.Answers) > max {
			continue // more than allowed: a spoiled vote
		}
		for _, a := range v.Answers {
			if k, ok := valid[a]; ok {
				p.Answers[k].Voters = append(p.Answers[k].Voters, who)
			}
		}
	}
	r.events[i].Poll = &p
}

// ---- Reading ------------------------------------------------------------

// contentMap is an event's content as a map, whatever way it arrived.
func contentMap(evt *event.Event) map[string]any {
	var m map[string]any
	if len(evt.Content.VeryRaw) > 0 && json.Unmarshal(evt.Content.VeryRaw, &m) == nil && m != nil {
		return m
	}
	if evt.Content.Raw != nil {
		return evt.Content.Raw
	}
	if b, err := json.Marshal(evt.Content.Parsed); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

// textOf is the words of a poll's question or answer, in either shape:
// a plain "org.matrix.msc1767.text" or "body", or the stable "m.text" list
// of representations.
func textOf(v any) string {
	m, _ := v.(map[string]any)
	if m == nil {
		return ""
	}
	if s, ok := m["org.matrix.msc1767.text"].(string); ok && s != "" {
		return s
	}
	if s, ok := m["body"].(string); ok && s != "" {
		return s
	}
	if list, ok := m["m.text"].([]any); ok {
		for _, it := range list {
			if rep, ok := it.(map[string]any); ok {
				if s, ok := rep["body"].(string); ok && s != "" {
					return s
				}
			}
		}
	}
	if list, ok := m["org.matrix.msc1767.message"].([]any); ok {
		for _, it := range list {
			if rep, ok := it.(map[string]any); ok {
				if s, ok := rep["body"].(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// parsePollStart reads a poll start's content.
func parsePollStart(c map[string]any) (p model.Poll, max int, ok bool) {
	body, _ := c["org.matrix.msc3381.poll.start"].(map[string]any)
	if body == nil {
		body, _ = c["m.poll"].(map[string]any)
	}
	if body == nil {
		return p, 0, false
	}
	p.Question = textOf(body["question"])
	if p.Question == "" {
		return p, 0, false
	}
	kind, _ := body["kind"].(string)
	p.Hidden = strings.Contains(kind, "undisclosed")
	max = 1
	if f, ok := body["max_selections"].(float64); ok && f >= 1 {
		max = int(f)
	}
	p.Multiple = max > 1
	answers, _ := body["answers"].([]any)
	for _, a := range answers {
		am, _ := a.(map[string]any)
		if am == nil {
			continue
		}
		aid, _ := am["id"].(string)
		if aid == "" {
			aid, _ = am["m.id"].(string)
		}
		if aid == "" {
			continue
		}
		p.Answers = append(p.Answers, model.PollAnswer{ID: aid, Text: textOf(am)})
	}
	return p, max, len(p.Answers) > 0
}

// relatedTo is the event a poll response or end names.
func relatedTo(c map[string]any) string {
	rel, _ := c["m.relates_to"].(map[string]any)
	if rel == nil {
		return ""
	}
	s, _ := rel["event_id"].(string)
	return s
}

func (s *Source) onPollStart(ctx context.Context, evt *event.Event) {
	p, max, ok := parsePollStart(contentMap(evt))
	if !ok {
		return
	}
	e := model.Event{
		ID: evt.ID.String(), Room: evt.RoomID.String(), Sender: evt.Sender.String(),
		Time: eventTime(evt), Kind: model.EventPoll, Body: p.Question, Poll: &p,
	}
	sender, txn := evt.Sender.String(), evt.Unsigned.TransactionID
	history := evt.Mautrix.EventSource&sourceHistory != 0
	s.post(func() {
		r := s.roomOf(e.Room)
		r.pollOf(e.ID).Max = max
		if txn != "" && sender == s.me.ID {
			r.adoptEcho(txn, e.ID) // the poll the user made here: same place, server's ID
		}
		if history {
			r.insertOlder(e)
		} else {
			r.appendEvent(e, s.nameOf(sender), sender == s.me.ID)
		}
		r.refreshPoll(e.ID)
		s.markDirty(e.Room)
	})
}

func (s *Source) onPollResponse(ctx context.Context, evt *event.Event) {
	c := contentMap(evt)
	target := relatedTo(c)
	if target == "" {
		return
	}
	var answers []string
	if body, ok := c["org.matrix.msc3381.poll.response"].(map[string]any); ok {
		answers = stringList(body["answers"])
	} else {
		answers = stringList(c["m.selections"])
	}
	room, who, ts := evt.RoomID.String(), evt.Sender.String(), evt.Timestamp
	s.post(func() {
		r := s.roomOf(room)
		st := r.pollOf(target)
		if old, ok := st.Votes[who]; ok && old.TS > ts {
			return // an older echo of a vote already replaced
		}
		st.Votes[who] = pollVote{TS: ts, Answers: answers}
		r.refreshPoll(target)
		s.markDirty(room)
	})
}

func (s *Source) onPollEnd(ctx context.Context, evt *event.Event) {
	target := relatedTo(contentMap(evt))
	if target == "" {
		return
	}
	room, who, ts := evt.RoomID.String(), evt.Sender.String(), evt.Timestamp
	s.post(func() {
		r := s.roomOf(room)
		st := r.pollOf(target)
		if st.Ends == nil {
			st.Ends = map[string]int64{}
		}
		if old, ok := st.Ends[who]; !ok || ts < old {
			st.Ends[who] = ts
		}
		r.refreshPoll(target)
		s.markDirty(room)
	})
}

func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, it := range list {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ---- Writing ------------------------------------------------------------

// CreatePoll sends a poll: it shows at once, pending, and takes its ID from
// the server as any message does.
func (s *Source) CreatePoll(room string, p model.Poll) {
	if s.cli == nil || strings.TrimSpace(p.Question) == "" || len(p.Answers) < 2 {
		return
	}
	answers := make([]map[string]any, len(p.Answers))
	local := make([]model.PollAnswer, len(p.Answers))
	for i, a := range p.Answers {
		aid := "a" + strconv.Itoa(i+1)
		answers[i] = map[string]any{"id": aid, "org.matrix.msc1767.text": a.Text}
		local[i] = model.PollAnswer{ID: aid, Text: a.Text}
	}
	max := 1
	if p.Multiple {
		max = len(p.Answers)
	}
	kind := "org.matrix.msc3381.poll.disclosed"
	if p.Hidden {
		kind = "org.matrix.msc3381.poll.undisclosed"
	}
	fallback := p.Question
	for i, a := range p.Answers {
		fallback += "\n" + strconv.Itoa(i+1) + ". " + a.Text
	}
	content := map[string]any{
		"org.matrix.msc3381.poll.start": map[string]any{
			"question":       map[string]any{"org.matrix.msc1767.text": p.Question, "body": p.Question, "msgtype": "m.text"},
			"kind":           kind,
			"max_selections": max,
			"answers":        answers,
		},
		"org.matrix.msc1767.text": fallback,
		"body":                    fallback,
		"msgtype":                 "m.text",
	}
	txn := s.cli.TxnID()
	id0 := localPrefix + txn
	shown := model.Poll{Question: p.Question, Answers: local, Multiple: p.Multiple, Hidden: p.Hidden}
	pending := model.Event{ID: id0, Room: room, Sender: s.me.ID, Time: time.Now(), Kind: model.EventPoll,
		Body: p.Question, Poll: &shown, Pending: true}
	r := s.roomOf(room)
	r.pollOf(id0).Max = max
	r.appendEvent(pending, "You", true)
	go func() {
		resp, err := s.cli.SendMessageEvent(context.Background(), id.RoomID(room), pollStartTypes[0], content,
			mautrix.ReqSendEvent{TransactionID: txn})
		s.post(func() {
			rr := s.roomOf(room)
			if err != nil {
				rr.failLocal(id0, "Not sent: "+plain(err))
				return
			}
			rr.settleLocal(id0, resp.EventID.String())
			if st, ok := rr.polls[id0]; ok {
				rr.polls[resp.EventID.String()] = st
				delete(rr.polls, id0)
			}
			rr.refreshPoll(resp.EventID.String())
		})
	}()
}

// Vote casts the user's vote: for a poll of one answer, the answer replaces
// the last (and a second press on it takes the vote back); for one that
// allows several, the answer is switched on or off among the user's. It
// shows at once; the response goes after.
func (s *Source) Vote(room, eventID string, answer int) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		return
	}
	i := r.indexOf(eventID)
	if i < 0 || r.events[i].Poll == nil || answer < 0 || answer >= len(r.events[i].Poll.Answers) {
		return
	}
	ev := r.events[i]
	if ev.Poll.Ended || ev.Pending {
		return
	}
	pick := ev.Poll.Answers[answer].ID
	st := r.pollOf(eventID)
	cur := st.Votes[s.me.ID].Answers
	var next []string
	switch {
	case slices.Contains(cur, pick):
		next = slices.DeleteFunc(slices.Clone(cur), func(a string) bool { return a == pick })
	case ev.Poll.Multiple:
		next = append(slices.Clone(cur), pick)
		if len(next) > max(st.Max, 1) {
			return // as many as the poll allows already
		}
	default:
		next = []string{pick}
	}
	now := time.Now().UnixMilli()
	st.Votes[s.me.ID] = pollVote{TS: now, Answers: next}
	r.refreshPoll(eventID)
	s.markDirty(room)
	content := map[string]any{
		"org.matrix.msc3381.poll.response": map[string]any{"answers": next},
		"m.relates_to":                     map[string]any{"rel_type": "m.reference", "event_id": eventID},
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.cli.SendMessageEvent(ctx, id.RoomID(room), pollResponseTypes[0], content); err != nil {
			s.post(func() { // not sent: the vote goes back to what it was
				if rr, ok := s.rooms[room]; ok {
					rr.pollOf(eventID).Votes[s.me.ID] = pollVote{TS: now - 1, Answers: cur}
					rr.refreshPoll(eventID)
				}
			})
			s.notice(room, "Could not send your vote: "+plain(err))
		}
	}()
}

// EndPoll ends one of the user's polls: it shows as ended at once and the
// results are shown to everyone.
func (s *Source) EndPoll(room, eventID string) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		return
	}
	i := r.indexOf(eventID)
	if i < 0 || r.events[i].Poll == nil || r.events[i].Sender != s.me.ID || r.events[i].Poll.Ended {
		return
	}
	st := r.pollOf(eventID)
	if st.Ends == nil {
		st.Ends = map[string]int64{}
	}
	st.Ends[s.me.ID] = time.Now().UnixMilli()
	r.refreshPoll(eventID)
	s.markDirty(room)
	content := map[string]any{
		"org.matrix.msc3381.poll.end": map[string]any{},
		"org.matrix.msc1767.text":     "The poll has ended.",
		"body":                        "The poll has ended.",
		"m.relates_to":                map[string]any{"rel_type": "m.reference", "event_id": eventID},
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.cli.SendMessageEvent(ctx, id.RoomID(room), pollEndTypes[0], content); err != nil {
			s.post(func() {
				if rr, ok := s.rooms[room]; ok {
					delete(rr.pollOf(eventID).Ends, s.me.ID)
					rr.refreshPoll(eventID)
				}
			})
			s.notice(room, "Could not end the poll: "+plain(err))
		}
	}()
}

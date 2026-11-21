package matrix

import (
	"strings"
	"time"

	"slopement/model"
)

// timelineCap is how many events a room keeps in memory, the newest of
// them; older ones page back from SQLite with a later milestone.
const timelineCap = 300

// room is one room's live state, touched only on the UI thread: what the
// lists show, and the newest events with their reactions.
type room struct {
	id        string
	name      string // m.room.name, else a DM partner's name, else the alias
	topic     string
	alias     string
	partner   string // a DM's other person (MXID)
	dm        bool
	encrypted bool
	members   int

	unread, mentions              int
	favourite, lowPriority, muted bool
	last                          time.Time
	preview                       string
	typing                        []string

	// events holds the timeline, newest last, capped. reactions indexes
	// the reaction events by the event they decorate.
	events    []model.Event
	reactions map[string]map[string]string // event ID → sender → key
}

// appendEvent adds e to the timeline, capped, and refreshes what the
// lists show of the room. fromMe says whether the sender is the user:
// their own messages don't count unread.
func (r *room) appendEvent(e model.Event, senderName string, fromMe bool) {
	r.events = append(r.events, e)
	if len(r.events) > timelineCap {
		r.events = r.events[len(r.events)-timelineCap:]
	}
	if e.Kind != model.EventSystem {
		r.last = e.Time
		who := senderName
		if fromMe {
			who = "You"
		}
		r.preview = who + ": " + strings.SplitN(e.Body, "\n", 2)[0]
		if !fromMe {
			r.unread++
			if e.Mentions {
				r.mentions++
			}
		}
	}
}

// editEvent replaces what e (a message's edit) says about its target.
func (r *room) editEvent(target, body string) {
	for i := range r.events {
		if r.events[i].ID == target {
			r.events[i].Body = body
			r.events[i].Edited = true
			return
		}
	}
}

// react records sender's reaction key on an event, or takes it back with
// an empty key.
func (r *room) react(target, sender, key string) {
	if r.reactions == nil {
		r.reactions = map[string]map[string]string{}
	}
	bySender := r.reactions[target]
	if bySender == nil {
		bySender = map[string]string{}
		r.reactions[target] = bySender
	}
	if key == "" {
		delete(bySender, sender)
	} else {
		bySender[sender] = key
	}
}

// reactionList is the counted reactions of one event, me's included.
func (r *room) reactionList(eventID, me string) []model.Reaction {
	bySender := r.reactions[eventID]
	if len(bySender) == 0 {
		return nil
	}
	counts := map[string]*model.Reaction{}
	var order []string
	for sender, key := range bySender {
		if it := counts[key]; it == nil {
			counts[key] = &model.Reaction{Key: key}
			order = append(order, key)
		}
		counts[key].Count++
		if sender == me {
			counts[key].Mine = true
		}
	}
	out := make([]model.Reaction, 0, len(order))
	for _, key := range order {
		out = append(out, *counts[key])
	}
	return out
}

// timeline is the room's events with their reactions, oldest first.
func (r *room) timeline(me string) []model.Event {
	out := make([]model.Event, len(r.events))
	for i, e := range r.events {
		e.Reactions = r.reactionList(e.ID, me)
		out[i] = e
	}
	return out
}

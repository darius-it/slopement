package matrix

import (
	"strings"
	"time"

	"maunium.net/go/mautrix/event"

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
	avatar    string
	partner   string // a DM's other person (MXID)
	dm        bool
	encrypted bool
	space     bool // a room of type m.space: a community
	members   int
	// hasSummary: the server's own joined-member count is what members
	// holds (lazy loading means the member events are only some of them).
	hasSummary bool

	unread, mentions              int
	favourite, lowPriority, muted bool
	last                          time.Time
	preview                       string
	typing                        []string

	// events holds the timeline, newest last, capped. reactions indexes
	// the reaction events by the event they decorate; raw keeps the
	// encrypted events that did not decrypt, for when the keys arrive.
	// A space also carries its children, by their order field when one is
	// set; a room may say which space it belongs to itself (m.space.parent,
	// canonical), which wins over any space's claim.
	events    []model.Event
	reactions map[string]map[string]string // event ID → sender → key
	raw       map[string]*rawEvent         // event ID → still encrypted
	children  []spaceChild
	parent    string // a m.space.parent claim, canonical ones alone
}

// spaceChild is one room of a space, in the space's own order.
type spaceChild struct {
	room  string
	order string
}

// rawEvent is an event that did not decrypt yet, kept for the retry when
// the keys arrive (verification or the key backup).
type rawEvent struct {
	evt  *event.Event
	tile model.Event // what the timeline shows until then
}

// appendEvent adds e to the timeline, capped, and refreshes what the
// lists show of the room. An event the room already has (a late decrypt
// re-dispatched) replaces the one it has, keeping the timeline's order.
// fromMe says whether the sender is the user: their own messages don't
// count unread.
func (r *room) appendEvent(e model.Event, senderName string, fromMe bool) {
	for i := range r.events {
		if r.events[i].ID == e.ID {
			e.Reactions = r.events[i].Reactions // what the room collected stays
			r.events[i] = e
			if i == len(r.events)-1 && e.Kind != model.EventSystem {
				// The newest event just decrypted: the list shows it.
				who := senderName
				if fromMe {
					who = "You"
				}
				r.preview = who + ": " + previewText(e)
			}
			return
		}
	}
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
		r.preview = who + ": " + previewText(e)
		if !fromMe {
			r.unread++
			if e.Mentions {
				r.mentions++
			}
		}
	}
}

// localPrefix marks the ID of a message that is shown before the server
// has given it one.
const localPrefix = "~local-"

// settleLocal gives a sent message the server's ID. If the sync's echo
// got here first, the echo is the event and the local copy goes.
func (r *room) settleLocal(local, real string) {
	li := r.indexOf(local)
	if li < 0 {
		return
	}
	if r.indexOf(real) >= 0 {
		r.removeAt(li) // the echo is already here
		return
	}
	r.events[li].ID, r.events[li].Pending = real, false
}

// failLocal turns a message that did not send into a quiet line saying so.
func (r *room) failLocal(local, why string) {
	if i := r.indexOf(local); i >= 0 {
		r.events[i].Kind, r.events[i].Body, r.events[i].Pending = model.EventSystem, why, false
	}
}

func (r *room) indexOf(id string) int {
	for i := range r.events {
		if r.events[i].ID == id {
			return i
		}
	}
	return -1
}

func (r *room) removeAt(i int) {
	if i >= 0 && i < len(r.events) {
		r.events = append(r.events[:i], r.events[i+1:]...)
	}
}

// previewText is what the room list shows of an event: its first line,
// or a word for what has no text.
func previewText(e model.Event) string {
	switch {
	case e.Kind == model.EventUndecryptable:
		return "Encrypted message"
	case e.Kind == model.EventMedia && len(e.Media) > 0:
		switch e.Media[0].Kind {
		case model.MediaVideo:
			return "Video"
		case model.MediaImage:
			return "Photo"
		}
		return "Attachment"
	}
	if line := strings.TrimSpace(strings.SplitN(e.Body, "\n", 2)[0]); line != "" {
		return line
	}
	return "Message"
}

// editEvent replaces what e (a message's edit) says about its target.
func (r *room) editEvent(target, body string) {
	for i := range r.events {
		if r.events[i].ID == target {
			r.events[i].Body = body
			r.events[i].Edited = true
			if i == len(r.events)-1 {
				if who, _, ok := strings.Cut(r.preview, ": "); ok {
					r.preview = who + ": " + previewText(r.events[i])
				}
			}
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

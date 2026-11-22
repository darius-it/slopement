package matrix

import (
	"sort"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"

	"slopement/format"
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
	reactions map[string]map[string]reactionRec // event ID → reaction event ID → who and which key
	reactSeq  int                               // orders the reactions as they were made
	reactTxn  map[string]string                 // transaction ID → local ID, of reactions the user is sending
	raw       map[string]*rawEvent         // event ID → still encrypted
	children  []spaceChild
	parent    string // a m.space.parent claim, canonical ones alone
	// reads is where each other person's public read receipt is: the
	// event and when (the receipt's time, for an event this room does not
	// hold).
	reads map[string]readMark
	// edits is each edited event's latest edit, kept until the event is
	// there to take it.
	edits map[string]editMark

	// pinned are the room's pinned events (m.room.pinned_events).
	pinned []string
	// pinEvents are pinned messages the room's timeline does not hold,
	// fetched (not kept in the cache: asked for again each run), and
	// pinsAsked says this run has asked.
	pinEvents map[string]model.Event
	pinsAsked bool

	// History: back is the token to page older events from ("" until the
	// first page is asked for), backDone says there are no older ones,
	// backBusy that a page is on its way. keep is how many events the room
	// holds after pages were loaded, so live events do not trim them.
	back     string
	backDone bool
	backBusy bool
	keep     int
}

// readMark is one person's place in a room.
type readMark struct {
	Event string
	TS    int64
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
	// since is when this app began waiting for its key: an old message
	// paged in a moment ago has had no time to be looked for in the key
	// backup, so it is not yet "stuck".
	since time.Time
}

// appendEvent adds e to the timeline, capped, and refreshes what the
// lists show of the room. An event the room already has (a late decrypt
// re-dispatched) replaces the one it has, keeping the timeline's order.
// fromMe says whether the sender is the user: their own messages don't
// count unread.
func (r *room) appendEvent(e model.Event, senderName string, fromMe bool) {
	if mark, ok := r.edits[e.ID]; ok && e.Kind != model.EventUndecryptable && e.Kind != model.EventSystem {
		mark.apply(&e) // an edit that came before the event could take it
	}
	for i := range r.events {
		if r.events[i].ID == e.ID {
			if e.Kind == model.EventUndecryptable && r.events[i].Kind != model.EventUndecryptable {
				return // a lock tile never replaces what is already readable
			}
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
	if limit := max(timelineCap, r.keep); len(r.events) > limit {
		r.events = r.events[len(r.events)-limit:] // history the user loaded stays, while it fits
	}
	if e.Kind != model.EventSystem {
		r.last = e.Time
		who := senderName
		if fromMe {
			who = "You"
		}
		if !lockedForNow(e) { // a lock that may open in a second is not a preview
			r.preview = who + ": " + previewText(e)
		}
		if !fromMe {
			r.unread++
			if e.Mentions {
				r.mentions++
			}
		}
	}
}

// insertOlder puts an event loaded from the room's history in its place by
// time. It is not news: the room's last activity, preview and unread count
// stay as they are. An event the room already holds is replaced, as
// appendEvent does.
func (r *room) insertOlder(e model.Event) {
	if r.indexOf(e.ID) >= 0 {
		r.appendEvent(e, "", false)
		return
	}
	if mark, ok := r.edits[e.ID]; ok && e.Kind != model.EventUndecryptable && e.Kind != model.EventSystem {
		mark.apply(&e)
	}
	i := len(r.events)
	for i > 0 && r.events[i-1].Time.After(e.Time) {
		i--
	}
	r.events = append(r.events, model.Event{})
	copy(r.events[i+1:], r.events[i:])
	r.events[i] = e
	r.keep = max(r.keep, len(r.events)) // what was loaded is kept, up to here
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

// failLocal marks a message that did not send: it stays where it is, with
// the reason; nothing else about it changes.
func (r *room) failLocal(local, why string) {
	if i := r.indexOf(local); i >= 0 {
		r.events[i].Failed, r.events[i].Error, r.events[i].Pending = true, why, false
	}
}

// adoptEcho is the sync's echo of a message the user sent from here,
// found by its transaction ID: the local copy takes the server's ID and
// stays as it is. Reports whether there was a local copy — the echo needs
// no tile of its own (least of all an encrypted one that says it is
// waiting for keys, for text the user just typed).
func (r *room) adoptEcho(txn, real string) bool {
	local := localPrefix + txn
	li := r.indexOf(local)
	if li < 0 {
		return false
	}
	if r.indexOf(real) >= 0 {
		r.removeAt(li)
		return true
	}
	r.events[li].ID, r.events[li].Pending = real, false
	return true
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
	body := e.Body
	if e.Formatted != "" {
		// What it says, not the Markdown it was typed as: "- a" is "a" in
		// the list, and "**x**" is x.
		body = format.Plain(format.ParseHTML(e.Formatted))
	}
	if line := strings.TrimSpace(strings.SplitN(body, "\n", 2)[0]); line != "" {
		return line
	}
	return "Message"
}

// editEvent replaces what e (a message's edit) says about its target.
//
// The edit is kept by its target, newest by time: the target may not be
// readable yet (still waiting for its keys, or not synced) and takes the
// edit when it is — an edit that finds nothing and is forgotten leaves the
// message as it first was.
func (r *room) editEvent(target, body, formatted string, ts int64) {
	if old, ok := r.edits[target]; ok && old.TS > ts {
		return // an older edit arriving late
	}
	if r.edits == nil {
		r.edits = map[string]editMark{}
	}
	mark := editMark{Body: body, Formatted: formatted, TS: ts}
	r.edits[target] = mark
	i := r.indexOf(target)
	if i < 0 || r.events[i].Kind == model.EventUndecryptable {
		return
	}
	mark.apply(&r.events[i])
	if i == len(r.events)-1 {
		if who, _, ok := strings.Cut(r.preview, ": "); ok {
			r.preview = who + ": " + previewText(r.events[i])
		}
	}
}

// editMark is the latest edit of an event: its new text (and its HTML, if
// it has one), and when.
type editMark struct {
	Body      string
	Formatted string
	TS        int64
}

// apply puts the edit on an event: its words, as it was formatted.
func (m editMark) apply(e *model.Event) {
	e.Body, e.Formatted, e.Plain, e.Edited = m.Body, m.Formatted, m.Formatted == "", true
}

// reactionRec is one reaction: who put which key on an event, and in what
// order it came (the pills keep the order the reactions were made in).
type reactionRec struct {
	Sender, Key string
	Seq         int
}

// react records a reaction — the event that is the reaction, by its ID (or
// a local ID while it is being sent) — of sender's key on target. The same
// reaction event arriving twice (an echo of one shown at once) is one.
func (r *room) react(target, reactionID, sender, key string) {
	if key == "" || reactionID == "" {
		return
	}
	if r.reactions == nil {
		r.reactions = map[string]map[string]reactionRec{}
	}
	byID := r.reactions[target]
	if byID == nil {
		byID = map[string]reactionRec{}
		r.reactions[target] = byID
	}
	if _, have := byID[reactionID]; have {
		return
	}
	r.reactSeq++
	byID[reactionID] = reactionRec{Sender: sender, Key: key, Seq: r.reactSeq}
}

// unreact takes a reaction away, by the ID of the event that was the
// reaction (it was redacted); reports whether it was there.
func (r *room) unreact(reactionID string) bool {
	found := false
	for target, byID := range r.reactions {
		if _, ok := byID[reactionID]; ok {
			delete(byID, reactionID)
			found = true
			if len(byID) == 0 {
				delete(r.reactions, target)
			}
		}
	}
	return found
}

// hasReaction reports whether a reaction event is recorded.
func (r *room) hasReaction(reactionID string) bool {
	for _, byID := range r.reactions {
		if _, ok := byID[reactionID]; ok {
			return true
		}
	}
	return false
}

// myReaction is the ID of the reaction event by which sender put key on
// target, or "".
func (r *room) myReaction(target, sender, key string) string {
	for id, rec := range r.reactions[target] {
		if rec.Sender == sender && rec.Key == key {
			return id
		}
	}
	return ""
}

// renameReaction gives a reaction shown while it was being sent the ID the
// server gave it.
func (r *room) renameReaction(local, real string) {
	for _, byID := range r.reactions {
		if rec, ok := byID[local]; ok {
			delete(byID, local)
			byID[real] = rec
		}
	}
}

// reactionList is the counted reactions of one event, me's included, in the
// order each key was first used.
func (r *room) reactionList(eventID, me string) []model.Reaction {
	byID := r.reactions[eventID]
	if len(byID) == 0 {
		return nil
	}
	counts := map[string]*model.Reaction{}
	first := map[string]int{}
	for _, rec := range byID {
		it := counts[rec.Key]
		if it == nil {
			it = &model.Reaction{Key: rec.Key}
			counts[rec.Key] = it
			first[rec.Key] = rec.Seq
		}
		it.Count++
		if rec.Sender == me {
			it.Mine = true
		}
		first[rec.Key] = min(first[rec.Key], rec.Seq)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return first[keys[i]] < first[keys[j]] })
	out := make([]model.Reaction, len(keys))
	for i, k := range keys {
		out[i] = *counts[k]
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
	// Each person's marker lands on the event it names — or, when the
	// room does not show that event (a reaction, a state change), on the
	// last shown event before the receipt's time.
	//
	// Sending is reading: whoever wrote a message has read up to it, so a
	// person is at least on their own latest message — whether or not
	// their client sent a receipt for it, and whichever is further on.
	at := map[string]int{}
	for u, mark := range r.reads {
		i := r.indexOf(mark.Event)
		if i < 0 {
			for j := len(r.events) - 1; j >= 0; j-- {
				if r.events[j].Time.UnixMilli() <= mark.TS {
					i = j
					break
				}
			}
		}
		if i >= 0 {
			at[u] = i
		}
	}
	for i, e := range r.events {
		if e.Kind != model.EventSystem && e.Sender != me && e.Sender != "" && !e.Pending {
			if prev, ok := at[e.Sender]; !ok || i > prev {
				at[e.Sender] = i
			}
		}
	}
	users := make([]string, 0, len(at))
	for u := range at {
		users = append(users, u)
	}
	sort.Strings(users)
	for _, u := range users {
		out[at[u]].ReadBy = append(out[at[u]].ReadBy, u)
	}
	// A message that arrived a moment ago and cannot be read yet is not
	// shown as locked: its key is usually on its way, and a lock tile that
	// turns into the text a second later is a flicker. After the grace
	// period it shows as what it is.
	shown := out[:0]
	for _, e := range out {
		if !lockedForNow(e) {
			shown = append(shown, e)
		}
	}
	return shown
}

// unreadableGrace is how long a fresh message that cannot be read yet is
// kept out of sight before it shows as waiting for its keys.
const unreadableGrace = 20 * time.Second

// lockedForNow reports an unreadable message still inside the grace period.
func lockedForNow(e model.Event) bool {
	return e.Kind == model.EventUndecryptable && time.Since(e.Time) < unreadableGrace
}

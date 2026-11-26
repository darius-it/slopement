package app

import (
	"sort"

	"slopement/kit"
	"slopement/model"
)

// Read receipts are one privacy setting, off until the user turns it on:
// off, nothing is shown of who has read what, and the receipts the user
// sends are private (the server and the user's own devices keep the unread
// counts right; nobody else learns anything). On, both ways are open.

// receiptsSync tells the source what the setting is, when it changes (and
// once at the start).
func (a *App) receiptsSync() {
	if a.receiptsApplied && a.receiptsVal == a.prefs.receipts {
		return
	}
	if rp, ok := a.src.(model.ReceiptPrivacy); ok {
		rp.SetPublicReceipts(a.prefs.receipts)
	}
	a.receiptsApplied, a.receiptsVal = true, a.prefs.receipts
}

// receiptState is where the user's own messages stand in a room, as
// Element X and WhatsApp show it: the last one is "sent" (a hollow check)
// until someone reaches it; then the check fills with the picture of whoever
// has read it — several, overlapping, in a group. Each reader shows once, on
// the user's last message at or before their place.
type receiptState struct {
	sentID string              // the user's last message, when nobody has read it yet
	readID string              // the user's last message that someone has read up to
	readBy map[string][]string // the user's message → who has read up to it (and no further one of theirs)
}

// delivered says the server has the message: it is not still being sent, and
// it did not fail. Only a delivered message gets the "sent" check.
func delivered(ev model.Event) bool { return !ev.Pending && !ev.Failed }

// receiptStateOf reads it off a room's timeline (one pass, once a frame).
//
// With the setting off the receipts are not looked at, but the user's last
// message is still "sent": that says only that the server has it, nothing
// about anyone else.
//
// Someone who wrote after a message has read it, setting or not (their
// message is there for all to see): their picture fills the check then,
// rather than a lone "sent" staying under a message that was answered.
func (a *App) receiptStateOf(room string) (st receiptState) {
	me := a.src.Me().ID
	evs := a.src.Timeline(room)
	// Where each other person is: the furthest event their receipt names (with
	// the setting on), or that they wrote (setting or not).
	place := map[string]int{}
	at := func(who string, i int) {
		if who == me || who == "" {
			return
		}
		if p, ok := place[who]; !ok || i > p {
			place[who] = i
		}
	}
	own := func(ev model.Event) bool {
		return ev.Sender == me && ev.Kind != model.EventSystem && delivered(ev)
	}
	lastOwn := -1
	for i, ev := range evs {
		if a.prefs.receipts {
			for _, who := range ev.ReadBy {
				at(who, i)
			}
		}
		if ev.Sender != me && ev.Kind != model.EventSystem && !ev.Pending {
			at(ev.Sender, i)
		}
		if own(ev) {
			lastOwn = i
		}
	}
	// Each reader on the user's last message at or before their place.
	readers := make([]string, 0, len(place))
	for who := range place {
		readers = append(readers, who)
	}
	sort.Strings(readers) // the same order every frame
	furthest := -1
	for _, who := range readers {
		for i := min(place[who], lastOwn); i >= 0; i-- {
			if own(evs[i]) {
				if st.readBy == nil {
					st.readBy = map[string][]string{}
				}
				st.readBy[evs[i].ID] = append(st.readBy[evs[i].ID], who)
				if i > furthest {
					furthest, st.readID = i, evs[i].ID
				}
				break
			}
		}
	}
	if lastOwn >= 0 && lastOwn > furthest {
		st.sentID = evs[lastOwn].ID
	}
	return
}

// readers are the pictures of who has read up to ev and no further message of
// the user's: the circles that fill its check. Only the user's own messages
// have them.
func (a *App) readers(ev model.Event) []kit.AvatarSpec {
	ids := a.rs.readBy[ev.ID]
	if len(ids) == 0 {
		return nil
	}
	out := make([]kit.AvatarSpec, 0, len(ids))
	for _, id := range ids {
		p, _ := a.person(id)
		out = append(out, a.avatar(p, 16))
	}
	return out
}

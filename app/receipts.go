package app

import (
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
// Element shows it: the last one is "sent" (a hollow check) until someone's
// place in the room reaches it, and the last one anyone has read is "read".
type receiptState struct {
	sentID string // the user's last message, when nobody has read it yet
	readID string // the user's last message that someone has read up to
}

// delivered says the server has the message: it is not still being sent, and
// it did not fail. Only a delivered message gets the "sent" check.
func delivered(ev model.Event) bool { return !ev.Pending && !ev.Failed }

// receiptStateOf reads it off a room's timeline (one pass, once a frame).
//
// With the setting off nothing is shown of who has read what (the furthest
// read place is not looked at), but the user's last message is still "sent":
// that says only that the server has it, nothing about anyone else.
//
// Someone who wrote after a message has read it, setting or not (their
// message is there for all to see): the mark moves on to "read" then, rather
// than a lone "sent" staying under a message that was answered.
func (a *App) receiptStateOf(room string) (st receiptState) {
	me := a.src.Me().ID
	evs := a.src.Timeline(room)
	furthest, lastOwn := -1, -1
	for i, ev := range evs {
		if a.prefs.receipts && len(ev.ReadBy) > 0 {
			furthest = i // the furthest anyone has read
		}
		if ev.Sender != me && ev.Sender != "" && ev.Kind != model.EventSystem && !ev.Pending {
			furthest = i // they answered: they have read what came before
		}
		if ev.Sender == me && ev.Kind != model.EventSystem && delivered(ev) {
			lastOwn = i
		}
	}
	if furthest >= 0 {
		for i := min(lastOwn, furthest); i >= 0; i-- {
			if evs[i].Sender == me && evs[i].Kind != model.EventSystem && delivered(evs[i]) {
				st.readID = evs[i].ID
				break
			}
		}
	}
	if lastOwn >= 0 && lastOwn > furthest {
		st.sentID = evs[lastOwn].ID
	}
	return
}

// readers are the avatars of who has read up to ev — none while the
// setting is off. The sender of ev is left out: whoever wrote a message is
// placed on it (sending is reading), and their own circle beside their own
// words says nothing.
func (a *App) readers(ev model.Event) []kit.AvatarSpec {
	if !a.prefs.receipts || len(ev.ReadBy) == 0 {
		return nil
	}
	out := make([]kit.AvatarSpec, 0, len(ev.ReadBy))
	for _, id := range ev.ReadBy {
		if id == ev.Sender {
			continue
		}
		p, _ := a.person(id)
		out = append(out, a.avatar(p, 16))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

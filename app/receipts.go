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

// receiptStateOf reads it off a room's timeline (one pass, once a frame).
func (a *App) receiptStateOf(room string) (st receiptState) {
	if !a.prefs.receipts {
		return
	}
	me := a.src.Me().ID
	evs := a.src.Timeline(room)
	furthest, lastOwn := -1, -1
	for i, ev := range evs {
		if len(ev.ReadBy) > 0 {
			furthest = i // the furthest anyone has read
		}
		if ev.Sender == me && ev.Kind != model.EventSystem && !ev.Pending {
			lastOwn = i
		}
	}
	for i := min(lastOwn, furthest); i >= 0; i-- {
		if evs[i].Sender == me && evs[i].Kind != model.EventSystem && !evs[i].Pending {
			st.readID = evs[i].ID
			break
		}
	}
	if lastOwn >= 0 && lastOwn > furthest {
		st.sentID = evs[lastOwn].ID
	}
	return
}

// readers are the avatars of who has read up to ev — none while the
// setting is off.
func (a *App) readers(ev model.Event) []kit.AvatarSpec {
	if !a.prefs.receipts || len(ev.ReadBy) == 0 {
		return nil
	}
	out := make([]kit.AvatarSpec, 0, len(ev.ReadBy))
	for _, id := range ev.ReadBy {
		p, _ := a.person(id)
		out = append(out, a.avatar(p, 16))
	}
	return out
}

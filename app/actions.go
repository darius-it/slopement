package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// msgMenu is what a message's menu offers the user: anything can be
// replied to, copied and pinned; their own words can be edited; their own
// messages deleted. A message still being sent, or one that could not be
// read, has none.
func (a *App) msgMenu(ev model.Event) kit.MessageMenuState {
	st := kit.MessageMenuState{
		Off: ev.Pending || ev.Failed || ev.Kind == model.EventUndecryptable || ev.Kind == model.EventSystem,
	}
	st.HasText = (ev.Kind == model.EventText || ev.Kind == model.EventNotice || ev.Kind == model.EventEmote) && strings.TrimSpace(ev.Body) != ""
	st.Mine = ev.Sender == a.src.Me().ID
	if acts, ok := a.src.(model.Actions); ok {
		st.Pinned = acts.IsPinned(ev.Room, ev.ID)
	}
	return st
}

// messageChoice carries out what was chosen from a message's menu.
func (a *App) messageChoice(room string, ev model.Event, ch kit.MessageChoice) {
	acts, canAct := a.src.(model.Actions)
	switch ch {
	case kit.MsgOpenSheet:
		a.msgSheet.Open, a.msgRoom, a.msgID = true, room, ev.ID
	case kit.MsgCopyText:
		a.copyToClipboard(ev.Body, "Copied")
	case kit.MsgCopyLink:
		a.copyToClipboard("https://matrix.to/#/"+room+"/"+ev.ID, "Link copied")
	case kit.MsgEdit:
		if canAct {
			st := a.draft(room)
			st.Editing, st.Text = ev.ID, ev.Body
			st.Reply = nil
			delete(a.replyTo, room)
			st.Focus()
		}
	case kit.MsgPin:
		if canAct {
			acts.Pin(room, ev.ID, true)
		}
	case kit.MsgUnpin:
		if canAct {
			acts.Pin(room, ev.ID, false)
		}
	case kit.MsgDelete:
		if canAct {
			a.confirmDelete(room, ev.ID)
		}
	}
}

// copyToClipboard copies text and says so.
func (a *App) copyToClipboard(text, said string) {
	if a.win == nil {
		return
	}
	mygo.Clipboard.WriteText(text)
	a.toast = said
}

// confirmDelete asks before deleting (a deleted message is gone for
// everyone), in the system's dialog.
func (a *App) confirmDelete(room, id string) {
	acts, ok := a.src.(model.Actions)
	if !ok || a.win == nil {
		return
	}
	win := a.win
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Title: "Delete this message?",
			Message: "It is removed for everyone in the room.", Buttons: []string{"Delete", "Cancel"},
			DefaultButton: 1, CancelButton: 1, DestructiveButtons: []int{0}})
		if err != nil || res.Button != 0 {
			return
		}
		win.Update(func() { acts.Redact(room, id) })
	}()
}

// editStep sends an edit when the composer was editing a message: the
// words replace the message's; empty words stop the edit. It reports
// whether this send was an edit.
func (a *App) editStep(room string, st *kit.ComposerState) bool {
	if st.Editing == "" {
		return false
	}
	id := st.Editing
	st.Editing = ""
	if acts, ok := a.src.(model.Actions); ok && hasWords(st.Text) {
		acts.Edit(room, id, st.Text)
	}
	st.Text = ""
	return true
}

// messageSheet is a phone's menu of one message, shown on a long press.
func (a *App) messageSheet(c *ui.Context) {
	var ev model.Event
	found := false
	for _, e := range a.src.Timeline(a.msgRoom) {
		if e.ID == a.msgID {
			ev, found = e, true
		}
	}
	if !found {
		a.msgSheet.Open = false
		return
	}
	st := a.msgMenu(ev)
	kit.Sheet(c, &a.msgSheet, "", func() {
		row := func(icon *ui.SVG, title string, ch kit.MessageChoice) {
			if kit.ChoiceRow(c, icon, title, "").Clicked() {
				a.msgSheet.Open = false
				if ch == kit.MsgReply {
					d := a.draft(a.msgRoom)
					d.Reply = a.quote(a.msgRoom, ev.ID)
					a.replyTo[a.msgRoom] = ev.ID
					return
				}
				a.messageChoice(a.msgRoom, ev, ch)
			}
		}
		row(kit.IconReply, "Reply", kit.MsgReply)
		if st.HasText {
			row(kit.IconCopy, "Copy text", kit.MsgCopyText)
		}
		row(kit.IconLink, "Copy link", kit.MsgCopyLink)
		if st.Pinned {
			row(kit.IconPin, "Unpin", kit.MsgUnpin)
		} else {
			row(kit.IconPin, "Pin", kit.MsgPin)
		}
		if st.Mine {
			if st.HasText {
				row(kit.IconPencil, "Edit", kit.MsgEdit)
			}
			row(kit.IconTrash, "Delete", kit.MsgDelete)
		}
	})
}

// pinnedEvents are a room's pinned messages, in the order pinned: all of
// them where the source can fetch the ones it does not hold, else those it
// holds.
func (a *App) pinnedEvents(room string) []model.Event {
	if p, ok := a.src.(model.Pins); ok {
		return p.PinnedEvents(room)
	}
	acts, ok := a.src.(model.Actions)
	if !ok {
		return nil
	}
	held := map[string]model.Event{}
	for _, e := range a.src.Timeline(room) {
		held[e.ID] = e
	}
	var out []model.Event
	for _, id := range acts.Pinned(room) {
		if e, ok := held[id]; ok {
			out = append(out, e)
		}
	}
	return out
}

// pinText is a pinned message in one line: its words, "Attachment" for a
// file or picture, and for one whose key has not come yet a quiet note (the
// app keeps looking for it; it is not a call to verify).
func pinText(ev model.Event) string {
	if ev.Kind == model.EventUndecryptable {
		return "Encrypted message, loading…"
	}
	if text := strings.Join(strings.Fields(ev.Body), " "); text != "" {
		return text
	}
	return "Attachment"
}

// pinsPanel is the side panel of pinned messages: each with who and what,
// a press that goes to it, and a way to unpin.
func (a *App) pinsPanel(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	pins := a.pinnedEvents(r.ID)
	if len(pins) == 0 {
		ui.Column(c).Padding(kit.S6, kit.S3).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, kit.IconPin).Size(24, 24).TextColor(p.TextFaint)
			ui.Text(c, "Nothing pinned yet").FontSize(kit.SizeBody).TextColor(p.TextMuted)
			ui.Text(c, "Pin a message from its ⋯ menu to keep it here.").FontSize(kit.SizeSmall).TextColor(p.TextFaint).TextAlign(ui.Center)
		})
		return
	}
	acts, _ := a.src.(model.Actions)
	for i := len(pins) - 1; i >= 0; i-- { // newest pin first
		ev := pins[i]
		sender, _ := a.src.Person(ev.Sender)
		text := pinText(ev)
		card := ui.ButtonBase(c.Key("pin:"+ev.ID)).Padding(kit.S3).Gap(kit.S1).Radius(kit.RadiusMd).Background(p.Bg).
			Border(1, p.Border).Label("Go to pinned message from " + sender.Name)
		if card.Hovered() {
			card.Background(p.Hover)
		}
		card.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(kit.S1).Children(func() {
				ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
					kit.Avatar(c, a.avatar(sender, 20))
					ui.Text(c, sender.Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).TextColor(a.colors.Color(p, ev.Sender)).SingleLine().Shrink(1)
					ui.Text(c, timeLabel(ev.Time, a.now)).FontSize(kit.SizeCaption).TextColor(p.TextFaint).SingleLine()
				})
				ui.Text(c, text).FontSize(kit.SizeSmall).TextColor(p.Text).MaxLines(4)
			})
		})
		if card.Clicked() {
			a.jumpToEvent(r.ID, ev)
		}
		if acts != nil {
			ui.Row(c).Justify(ui.End).Children(func() {
				if kit.Button(c, kit.Ghost, "Unpin", kit.IconPin).Height(24).Clicked() {
					acts.Pin(r.ID, ev.ID, false)
				}
			})
		}
	}
}

// pinBar is the line under a room's header when messages are pinned in it:
// the one shown, newest pin first, and a press that goes to it and on to
// the next older one. The pin shown is kept by its ID, not its place: the
// pins the server is asked for arrive one by one, and a place would point
// at another message each time one lands.
func (a *App) pinBar(c *ui.Context, room string) {
	pins := a.pinnedEvents(room)
	if len(pins) == 0 {
		return
	}
	at := 0 // newest first: pins are in the order pinned
	for i := range pins {
		if pins[len(pins)-1-i].ID == a.pinAt[room] {
			at = i
			break
		}
	}
	ev := pins[len(pins)-1-at]
	sender, _ := a.src.Person(ev.Sender)
	text := pinText(ev)
	if kit.PinBar(c, at, len(pins), sender.Name, text) {
		a.jumpToEvent(room, ev)
		a.pinAt[room] = pins[len(pins)-1-(at+1)%len(pins)].ID
	}
}

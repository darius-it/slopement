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

// pinBar is the line under a room's header when messages are pinned in it:
// the one shown, among those the app holds, and a press that goes to it and
// on to the next.
func (a *App) pinBar(c *ui.Context, room string) {
	acts, ok := a.src.(model.Actions)
	if !ok {
		return
	}
	pinned := acts.Pinned(room)
	if len(pinned) == 0 {
		return
	}
	held := map[string]model.Event{}
	for _, e := range a.src.Timeline(room) {
		held[e.ID] = e
	}
	var shown []model.Event
	for _, id := range pinned {
		if e, ok := held[id]; ok {
			shown = append(shown, e)
		}
	}
	if len(shown) == 0 {
		return // pinned, but older than what is loaded
	}
	at := a.pinAt[room]
	if at >= len(shown) {
		at = 0
	}
	ev := shown[at]
	sender, _ := a.src.Person(ev.Sender)
	text := strings.Join(strings.Fields(ev.Body), " ")
	if text == "" {
		text = "Attachment"
	}
	if kit.PinBar(c, at, len(shown), sender.Name, text) {
		a.jumpTo(room, ev.ID)
		a.pinAt[room] = (at + 1) % len(shown)
	}
}

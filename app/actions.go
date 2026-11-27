package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/format"
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
	st.Thread = a.threadsOn()
	if _, ok := a.src.(model.Packs); ok && !st.Off && len(ev.Media) > 0 && ev.Media[0].Sticker && strings.HasPrefix(ev.Media[0].URL, "mxc://") {
		st.Sticker = true
	}
	if acts, ok := a.src.(model.Actions); ok {
		st.Pinned = acts.IsPinned(ev.Room, ev.ID)
	}
	_, st.Source = a.src.(model.EventSourcer)
	if _, ok := a.src.(model.Forwarder); ok && !st.Off {
		st.Forward = ev.Kind == model.EventText || ev.Kind == model.EventNotice || ev.Kind == model.EventEmote || ev.Kind == model.EventMedia
	}
	return st
}

// messageChoice carries out what was chosen from a message's menu.
func (a *App) messageChoice(room string, ev model.Event, ch kit.MessageChoice) {
	acts, canAct := a.src.(model.Actions)
	switch ch {
	case kit.MsgThread:
		a.openThread(room, ev)
	case kit.MsgOpenSheet:
		a.msgPop = msgPopState{Open: true}
		a.msgRoom, a.msgID = room, ev.ID
	case kit.MsgCopyText:
		a.copyToClipboard(ev.Body, "Copied")
	case kit.MsgCopyLink:
		a.copyToClipboard("https://matrix.to/#/"+room+"/"+ev.ID, "Link copied")
	case kit.MsgViewSource:
		a.openSource(room, ev.ID)
	case kit.MsgForward:
		a.openForward(room, ev.ID)
	case kit.MsgAddToPack:
		if len(ev.Media) > 0 {
			m := ev.Media[0]
			name := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(m.Name)), "_"))
			a.addToPack(model.PackEmoji{Shortcode: name, Body: m.Name, URL: m.URL, Mime: m.Mime, W: m.W, H: m.H, Emoticon: true, Sticker: true})
		}
	case kit.MsgEdit:
		if canAct {
			a.startEdit(room, ev)
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

// editLast starts editing the user's last message in the room that can be
// edited (↑ in an empty composer). A message still sending, one that failed, a
// picture and a system line are passed over.
func (a *App) editLast(room string) {
	me := a.src.Me().ID
	tl := a.src.Timeline(room)
	for i := len(tl) - 1; i >= 0; i-- {
		ev := tl[i]
		if ev.Sender != me || ev.Pending || ev.Failed {
			continue
		}
		if st := a.msgMenu(ev); st.Mine && st.HasText && !st.Off {
			a.messageChoice(room, ev, kit.MsgEdit)
			return
		}
	}
}

// inlineEdit is the message being edited in its own place in the timeline,
// with a field of its own: the composer keeps what was being written.
type inlineEdit struct {
	room   string
	st     kit.ComposerState
	scroll bool // bring the message into view (↑ may pick one out of sight)
}

// startEdit begins editing a message: in its place on a wide window, in the
// composer on a narrow one (a phone), which sets the draft aside meanwhile.
func (a *App) startEdit(room string, ev model.Event) {
	var st *kit.ComposerState
	if a.narrow {
		a.edit = nil
		st = a.draft(room)
	} else {
		a.edit = &inlineEdit{room: room, scroll: true}
		st = &a.edit.st
	}
	st.BeginEdit(ev.ID, ev.Body)
	st.Focus()
}

// editing is the edit shown in this message's place, if there is one. A reply
// of a thread is edited in the thread's panel, everything else in the room.
func (a *App) editing(room string, ev model.Event) *inlineEdit {
	e := a.edit
	if e == nil || e.room != room || e.st.Editing != ev.ID || a.inThread != (a.threadsOn() && ev.ThreadRoot != "") {
		return nil
	}
	return e
}

// editorStep runs before a room's timeline is built: an edit that was left
// (Escape, Cancel, saved) gives the keyboard back to the composer, and a new
// one is scrolled into view.
func (a *App) editorStep(room string, rows []trow, ls *ui.ListState) {
	e := a.edit
	if e == nil {
		return
	}
	if e.st.Editing == "" {
		a.edit, a.focusRoom = nil, e.room
		return
	}
	if !e.scroll || e.room != room {
		return
	}
	e.scroll = false
	for i := range rows {
		if rows[i].ev != nil && rows[i].ev.ID == e.st.Editing {
			ls.ScrollIntoView(i)
		}
	}
}

// editStep sends an edit when the field was editing a message: the words
// replace the message's; empty words stop the edit. It reports whether this
// send was an edit.
func (a *App) editStep(room string, st *kit.ComposerState) bool {
	if st.Editing == "" {
		return false
	}
	id, text := st.Editing, st.Text
	st.EndEdit()
	if acts, ok := a.src.(model.Actions); ok && hasWords(text) {
		acts.Edit(room, id, text)
	}
	return true
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
	body := ev.Body
	if ev.Formatted != "" {
		body = format.Plain(format.ParseHTML(ev.Formatted)) // what it says, not how it was typed
	}
	if text := strings.Join(strings.Fields(body), " "); text != "" {
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
		a.pinCard(c, r, ev, acts)
	}
}

// pinCard is one pinned message in the panel: a card with the sender, the
// message as the timeline draws it (formatting, pictures, GIFs, files), a
// press that goes to it, and Unpin in its corner, shown while the pointer
// is on the card.
func (a *App) pinCard(c *ui.Context, r model.Room, ev model.Event, acts model.Actions) {
	p := kit.Of(c)
	sender, _ := a.person(ev.Sender)
	card := ui.Column(c.Key("pin:" + ev.ID)).Padding(kit.S3).Gap(kit.S2).Radius(kit.RadiusMd).
		Border(1, p.Border).Label("Pinned message from " + sender.Name)
	hov := card.Hovered()
	bg := p.Bg
	if hov {
		bg = p.Hover.Over(p.Bg)
	}
	card.Background(card.AnimateColor("bg", bg, kit.Color)).Cursor(ui.CursorPointer)
	var unpin, opened bool
	card.Children(func() {
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Height(24).Children(func() {
			kit.Avatar(c, a.avatar(sender, 20))
			ui.Text(c, sender.Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).TextColor(a.colors.Color(p, ev.Sender)).SingleLine().Shrink(1)
			ui.Text(c, timeLabel(ev.Time, a.now)).FontSize(kit.SizeCaption).TextColor(p.TextFaint).SingleLine().Grow(1).
				Tooltip(fullTime(ev.Time, a.now))
			// A place kept for it, so the card does not move as it shows: a
			// small struck-through pin, level with the name.
			ui.Box(c).Size(24, 24).Shrink(0).Children(func() {
				if hov && acts != nil {
					b := ui.ButtonBase(c.Key("unpin:" + ev.ID)).Size(24, 24).Radius(kit.RadiusSm).
						AlignItems(ui.Center).Justify(ui.Center).Label("Unpin").Tooltip("Unpin")
					fg := p.TextMuted
					if b.Hovered() {
						b.Background(p.Hover)
						fg = p.Text
					}
					kit.Pressable(b)
					b.Children(func() { ui.Icon(c, kit.IconPinOff).Size(15, 15).TextColor(fg) })
					unpin = b.Clicked()
				}
			})
		})
		ui.Column(c).Gap(kit.S2).MinWidth(0).MaxHeight(320).Clip().Children(func() {
			if ev.Kind == model.EventUndecryptable {
				ui.Text(c, pinText(ev)).FontSize(kit.SizeSmall).Italic().TextColor(p.TextMuted)
				return
			}
			doc, media, _ := a.content(ev)
			if doc != nil {
				// A click on the words goes to the message as one beside them does;
				// a drag over them selects.
				kit.MessageBody(c, "pin:"+ev.ID, doc, a.reveals, kit.BodyStyle{Size: kit.SizeSmall + 1,
					OnClick: func() { a.jumpToEvent(r.ID, ev) }})
			}
			if len(media) > 0 {
				if i := kit.MediaGrid(c, media, 236, a.reveals); i >= 0 {
					a.viewerItems, a.viewerAt, a.viewer = media, i, true
					opened = true
				}
			}
			if f := a.fileSpec(ev); f != nil && kit.FileCard(c, *f).Clicked() {
				a.downloadFile(ev)
				opened = true
			}
			if doc == nil && len(media) == 0 && ev.Media == nil {
				ui.Text(c, pinText(ev)).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			}
		})
	})
	switch {
	case unpin:
		acts.Pin(r.ID, ev.ID, false)
	case card.Clicked() && !opened:
		a.jumpToEvent(r.ID, ev)
	}
}


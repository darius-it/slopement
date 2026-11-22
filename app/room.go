package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// trow is a row of the timeline: a date separator, or an event.
type trow struct {
	key   string
	day   string // a separator's label
	ev    *model.Event
	first bool // the first of a run from one sender
}

// rows lays out a room's timeline: separators between days, runs of one
// sender within five minutes grouped.
func (a *App) rows(room string) []trow {
	evs := a.src.Timeline(room)
	out := make([]trow, 0, len(evs)+2)
	var prev *model.Event
	threads := a.threadsOn()
	for i := range evs {
		ev := &evs[i]
		if threads && ev.ThreadRoot != "" {
			continue // a reply in a thread shows in its thread's panel
		}
		newDay := prev == nil || !sameDay(prev.Time, ev.Time)
		if newDay {
			out = append(out, trow{key: "day:" + ev.Time.Format("2006-01-02"), day: dayLabel(ev.Time, a.now)})
		}
		first := newDay || prev.Sender != ev.Sender || ev.Time.Sub(prev.Time) > 5*time.Minute ||
			prev.Kind == model.EventSystem || ev.Kind == model.EventPoll || prev.Kind == model.EventPoll
		out = append(out, trow{key: ev.ID, ev: ev, first: first})
		prev = ev
	}
	return out
}

func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func (a *App) listState(room string) *ui.ListState {
	ls := a.lists[room]
	if ls == nil {
		ls = &ui.ListState{FollowEnd: true}
		a.lists[room] = ls
	}
	return ls
}

// scrollState is the room timeline's scroll position, for Jump to latest.
func (a *App) scrollState(room string) *ui.ScrollState {
	s := a.scrolls[room]
	if s == nil {
		s = &ui.ScrollState{}
		a.scrolls[room] = s
	}
	return s
}

func (a *App) draft(room string) *kit.ComposerState {
	d := a.drafts[room]
	if d == nil {
		d = &kit.ComposerState{}
		a.drafts[room] = d
	}
	return d
}

// roomView is the open room on desktops: header, call banner, timeline,
// who is typing, and the composer.
func (a *App) roomView(c *ui.Context, back bool) {
	r, _ := a.src.Room(a.room)
	// What is pinned here, as the server has it (asked once per room a run).
	if pins, ok := a.src.(model.Pins); ok {
		pins.RefreshPins(r.ID)
	}
	col := ui.Column(c.Key("room:" + r.ID)).Grow(1).MinWidth(0)
	col.Children(func() {
		a.roomHeader(c, r, back)
		a.findBar(c) // ⌘F, under the header
		if r.Call {
			if kit.CallBanner(c, 3, a.inCall == r.ID) {
				a.joinCall(r)
			}
		}
		if a.inCall == r.ID {
			a.callArea(c, r)
		}
		rows := a.rows(r.ID)
		a.rs = a.receiptStateOf(r.ID)
		ls := a.listState(r.ID)
		ls.Key = func(i int) any { return rows[i].key }
		a.jumpStep(r.ID, rows, ls)
		a.readStep(r, rows, ls)
		a.pageStep(r.ID, ls)
		// The timeline with what floats over it: who is typing and Jump to
		// latest sit on its bottom edge, in the 24 left under the last
		// message, and take no room — a line that came and went would move
		// every message.
		ui.Column(c.Key("tlarea:"+r.ID)).Grow(1).Children(func() {
			// 24 under the last message before the composer, as Meetup's and
			// Fresha's web chats leave (Mobbin); it scrolls with the messages.
			ui.List(c.Key("tl:"+r.ID), ls, len(rows), func(i int) { a.timelineRow(c, r, rows[i]) }).
				Grow(1).Justify(ui.End).Padding(kit.S2, 0, kit.S6, 0).TrackScroll(a.scrollState(r.ID)).
				Selectable(). // one selection over the messages built: a drag crosses them
				Children(func() {
				if len(rows) == 0 {
					emptyNote(c, "No messages yet. Say hi!")
				}
			})
			if !ls.AtEnd() && len(rows) > 0 && !(a.jumpLooking && a.jumpRoom == r.ID) { // the seeking pill takes its place
				ui.Row(c.Key("jump")).Attach(ui.AnchorBottom, ui.AnchorBottom).Top(-kit.S1).Children(func() {
					if kit.Button(c, kit.Secondary, "Jump to latest", kit.IconArrowDown).Clicked() {
						// The list knows where its end is, rows it has not
						// measured yet included: aiming the scroll at a far
						// offset instead overshoots and then snaps back.
						ls.ScrollToEnd()
					}
				})
			}
			if len(r.Typing) > 0 {
				kit.TypingOverlay(c, strings.Join(r.Typing, ", ")+" is typing…")
			}
			a.seekPill(c, r.ID)
		})
		if a.encryptedWaiting(rows) {
			a.verifyBanner(c)
		}
		a.composer(c, r)
	})
	// Files dropped anywhere on the room wait in its composer.
	if files := col.DroppedFiles(); len(files) > 0 {
		a.addPaths(r.ID, files)
	}
	if col.FileDragOver() {
		col.Background(kit.Of(c).AccentSubtle.Alpha(0.5))
	}
}

// encryptedWaiting reports whether this room has messages waiting for
// keys and the session is unverified: the quiet banner under the
// timeline, the entry point that never goes away until it's done.
func (a *App) encryptedWaiting(rows []trow) bool {
	if a.verifier == nil || !a.verifier.State().Needed {
		return false
	}
	for i := range rows {
		if rows[i].ev != nil && rows[i].ev.Kind == model.EventUndecryptable {
			return true
		}
	}
	return false
}

// verifyBanner is the quiet line under a timeline with messages this
// session cannot read yet.
func (a *App) verifyBanner(c *ui.Context) {
	p := kit.Of(c)
	ui.Row(c).Padding(kit.S2, kit.S3).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, kit.IconShield).Size(16, 16).TextColor(p.Accent)
		ui.Text(c, "Encrypted messages need this session verified.").FontSize(kit.SizeSmall).
			TextColor(p.TextMuted).Grow(1)
		if kit.Button(c, kit.Secondary, "Verify", nil).Clicked() {
			a.verify.Open, a.verifyStep, a.verifyKey = true, 0, ""
		}
	})
}

// roomHeader is the bar over the timeline. A DM's name opens the
// person's card; the buttons call, search, and open the side panel.
func (a *App) roomHeader(c *ui.Context, r model.Room, back bool) {
	p := kit.Of(c)
	ui.Row(c).Height(52).PaddingX(kit.S3).Gap(kit.S2).AlignItems(ui.Center).Shrink(0).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
		if back && kit.IconButton(c, kit.Ghost, kit.IconChevronL, "Back").Clicked() {
			a.pushed = false
		}
		if !back && !a.sidebar && kit.IconButton(c, kit.Ghost, kit.IconChevronR, "Show the sidebar").Clicked() {
			a.sidebar = true
		}
		dm := r.Kind == model.KindDM
		partner, _ := a.src.Person(r.Partner)
		who := ui.ButtonBase(c.Key("who")).Gap(kit.S2).AlignItems(ui.Center).Justify(ui.Start).Grow(1).MinWidth(0).
			Radius(kit.RadiusSm).Label(r.Name)
		who.Children(func() {
			av := kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Colors: &a.colors, Square: r.Kind == model.KindGroup}
			if dm {
				av = a.avatar(partner, 28)
			}
			if r.Kind == model.KindRoom {
				ui.Icon(c, kit.IconHash).Size(18, 18).TextColor(p.TextMuted)
			} else {
				kit.Avatar(c, av)
			}
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Row(c).Gap(kit.S15).AlignItems(ui.Center).Children(func() {
					ui.Text(c, r.Name).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).SingleLine().Shrink(1)
					if !r.Encrypted {
						kit.Pill(c, "Not encrypted")
					}
					if r.Muted {
						ui.Icon(c, kit.IconBellOff).Size(12, 12).TextColor(p.TextFaint).Label(kit.MutedUntilText(r.MutedUntil, a.now))
					}
				})
				switch {
				case dm && partner.Status.Set(a.now):
					kit.StatusLine(c, status(partner.Status), a.now, false)
				case dm && partner.Presence == model.Online:
					ui.Text(c, "Online").FontSize(kit.SizeCaption).TextColor(p.TextMuted)
				case r.Topic != "":
					ui.Text(c, r.Topic+" · "+strconv.Itoa(r.Members)+" members").FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
				}
			})
		})
		if who.Clicked() {
			if dm {
				a.profileOpen = partner.ID
			} else {
				a.togglePanel(panelInfo)
			}
		}
		if dm {
			open := a.profileOpen == partner.ID && !kit.Compact(c)
			ui.PopoverBase(c, who, &open, func(panel ui.Element) {
				panel.Margin(6, 0, 0, 0).Padding(kit.S4).Radius(kit.RadiusLg).Background(p.Raised).Border(1, p.Border).
					Shadow(0, 12, 32, -6, p.Shadow).Transition(kit.PopIn())
				a.profileActs(partner.ID, kit.ProfileCard(c, *a.profile(partner.ID), &a.colors, a.now))
			})
			if !open && a.profileOpen == partner.ID && !kit.Compact(c) {
				a.profileOpen = ""
			}
		}
		if kit.IconButton(c, kit.Ghost, kit.IconPhone, "Call").Clicked() {
			a.joinCall(r)
		}
		if !back {
			if kit.IconButton(c, kit.Ghost, kit.IconSearch, "Find in this chat ("+kit.CmdF()+")").Clicked() {
				if a.find.Open {
					a.closeFind()
				} else {
					a.openFind()
				}
			}
			// Pinned messages, with how many there are in the label.
			pinLabel := "Pinned messages"
			if n := len(a.pinnedEvents(r.ID)); n > 0 {
				pinLabel += " (" + strconv.Itoa(n) + ")"
			}
			if kit.IconButton(c, kit.Ghost, kit.IconPin, pinLabel).AccessSelected(a.panel == panelPins).Clicked() {
				a.togglePanel(panelPins)
			}
			if !dm {
				if kit.IconButton(c, kit.Ghost, kit.IconUsers, "Members").Clicked() {
					a.togglePanel(panelMembers)
				}
			}
		}
		// The room's ⋯ opens its panel, with every option and setting,
		// rather than a dropdown (as Element's room options do).
		if kit.IconButton(c, kit.Ghost, kit.IconMore, "Room options").AccessSelected(a.panel == panelInfo).Clicked() {
			a.togglePanel(panelInfo)
		}
	})
}

func (a *App) togglePanel(p panel) {
	if a.panel == p {
		a.panel = panelNone
	} else {
		a.panel = p
	}
}

// timelineRow builds one row: a separator, a system line, or a message.
func (a *App) timelineRow(c *ui.Context, r model.Room, tr trow) {
	if tr.ev == nil {
		kit.DateSeparator(c, tr.day)
		return
	}
	ev := *tr.ev
	if ev.Kind == model.EventSystem {
		kit.SystemEvent(c, ev.Body)
		return
	}
	sender, _ := a.src.Person(ev.Sender)
	me := ev.Sender == a.src.Me().ID
	m := kit.MessageData{ID: ev.ID, Sender: sender.Name, SenderID: sender.ID, Time: ev.Time.Format("15:04"), TimeFull: fullTime(ev.Time, a.now), First: tr.first,
		Highlight: ev.Mentions, Flash: ev.ID == a.flash, FlashOut: ev.ID == a.flashOut, Edited: ev.Edited, Pending: ev.Pending, Failed: ev.Failed, Error: ev.Error, Undecryptable: ev.Kind == model.EventUndecryptable,
		Reactions: reactions(ev.Reactions), Reply: a.quote(r.ID, ev.ReplyTo), Reveals: a.reveals,
		Avatar: a.avatar(sender, 36), Unverified: sender.Trust == model.TrustWarning, ReadBy: a.readers(ev),
		Sent: ev.ID == a.rs.sentID && a.rs.sentID != ""}
	if ev.Thread != nil && a.threadsOn() && !a.inThread {
		m.Thread = a.threadChipOf(ev.Thread)
	}
	if ev.Kind != model.EventUndecryptable {
		m.Doc, m.Media, m.YouTube = a.content(ev)
		m.File = a.fileSpec(ev)
		if am, ok := audioOf(ev); ok {
			m.Audio = a.audioData(ev, am) // it plays; the file card stays for saving it
		}
		if m.YouTube == nil && len(m.Media) == 0 {
			m.Embed = a.linkCard(ev, m.Doc)
		}
	}
	m.Menu = a.msgMenu(ev)
	if ev.Poll != nil {
		m.Poll = a.pollData(ev.ID, *ev.Poll, me)
	}
	// Your own name and picture open your card too.
	m.Profile, m.ProfileOpen = a.profile(ev.Sender), a.profileMsg == ev.ID
	ui.Column(c.Key(ev.ID)).Children(func() {
		a.messageAct(r.ID, ev, m.Media, kit.Message(c, m, &a.colors))
	})
}

// messageAct applies what a message asked for.
func (a *App) messageAct(room string, ev model.Event, media []kit.MediaSpec, act kit.MessageAction) {
	switch {
	case act.React != "":
		a.src.React(room, ev.ID, act.React)
	case act.OpenEmoji:
		a.reactTo = ev.ID
		a.picker.Open, a.picker.Tab = true, kit.TabEmoji
	case act.Reply:
		d := a.draft(room)
		d.Reply = a.quote(room, ev.ID)
		a.replyTo[room] = ev.ID
		d.Focus()
	case act.OpenThread:
		a.openThread(room, ev)
	case act.Audio.Toggle || act.Audio.Seek:
		a.audioAct(ev, act.Audio)
	case act.Menu != kit.MsgNone:
		a.messageChoice(room, ev, act.Menu)
	case act.OpenMedia >= 0:
		a.viewerItems, a.viewerAt, a.viewer = media, act.OpenMedia, true
	case act.JumpTo != "":
		a.jumpTo(room, act.JumpTo)
	case act.Download:
		a.downloadFile(ev)
	case act.DismissPreview:
		a.noPreview[ev.ID] = true
	case act.PlayVideo:
		a.toast = "Inline YouTube playback comes later"
	case act.Vote >= 0:
		a.src.Vote(room, ev.ID, act.Vote)
	case act.EndPoll:
		a.src.EndPoll(room, ev.ID)
	case act.Profile:
		if a.profileMsg == ev.ID {
			a.profileMsg = ""
		} else {
			a.profileMsg = ev.ID
		}
	case act.ProfileAct.SetColor && !act.ProfileClosed: // chosen with the card still open
		a.chooseColor(ev.Sender, act.ProfileAct.Color)
	case act.ProfileClosed:
		a.profileMsg = ""
		a.profileActs(ev.Sender, act.ProfileAct)
	}
}

// profileActs applies a profile card's buttons.
func (a *App) profileActs(person string, act kit.ProfileAction) {
	switch {
	case act.SetColor: // the card stays open: the name changes under it
		a.chooseColor(person, act.Color)
	case act.Ignore, act.Unignore:
		a.profileOpen, a.profileMsg, a.profileSheet.Open = "", "", false
		a.setIgnore(person, act.Ignore)
	case act.OpenPicture:
		a.openProfilePicture(person)
	case act.Message:
		a.profileOpen, a.profileMsg, a.profileSheet.Open = "", "", false
		a.openDM(person, a.open)
	case act.Call:
		a.profileOpen, a.profileSheet.Open = "", false
		a.openDM(person, func(room string) {
			if r, ok := a.src.Room(room); ok {
				a.open(r.ID)
				a.joinCall(r)
			}
		})
	case act.Verify:
		a.profileOpen, a.profileSheet.Open = "", false
		a.verify.Open, a.verifyStep = true, 0
	case act.EditStatus:
		a.profileOpen, a.profileMsg, a.profileSheet.Open = "", "", false
		a.statusDraft.Edit(status(a.src.Me().Status))
	}
}

// composer is the message field with its picker: Enter sends.
func (a *App) composer(c *ui.Context, r model.Room) {
	if r.ReadOnly {
		// The room's power levels do not let the user write: a bar says so
		// instead of a field that would only fail to send.
		a.picker.Open = false
		kit.ReadOnlyBar(c, "You don't have permission to post in this room")
		return
	}
	st := a.draft(r.ID)
	place := "Message " + r.Name
	if r.Kind == model.KindRoom {
		place = "Message #" + r.Name
	}
	a.typingStep(r.ID, st.Text) // tell the room they are typing
	st.Suggest = a.suggestions(r.ID, st)
	st.OnPick = func(it kit.SuggestItem) { a.applySuggestion(r.ID, st, it) }
	var res kit.ComposerResult
	if kit.Compact(c) {
		res = kit.PhoneComposer(c.Key("composer:"+r.ID), st, "Message", &a.colors)
	} else {
		res = kit.Composer(c.Key("composer:"+r.ID), st, place, &a.colors)
	}
	a.composerAct(c, r, st, res)
}

func (a *App) composerAct(c *ui.Context, r model.Room, st *kit.ComposerState, res kit.ComposerResult) {
	switch {
	case res.Emoji && res.Unified:
		a.picker.Open = !a.picker.Open
	case res.GIF:
		a.picker.Open = !a.picker.Open || a.picker.Tab != kit.TabGIFs
		a.picker.Tab = kit.TabGIFs
	case res.Emoji:
		a.picker.Open = !a.picker.Open || a.picker.Tab != kit.TabEmoji
		a.picker.Tab = kit.TabEmoji
	case res.Poll:
		a.pollRoom, a.pollSheet.Open = r.ID, true
	case res.Attach:
		a.pickFiles(r.ID)
	}
	if res.Pick >= 0 && res.Pick < len(st.Suggest) {
		a.applySuggestion(r.ID, st, st.Suggest[res.Pick])
		c.Invalidate() // changed after the build: draw it now, not at the next event
		return         // taking a suggestion is not sending
	}
	if res.Send || res.Paste || res.RemovedFile >= 0 || res.Attach {
		defer c.Invalidate() // these change the draft after the build, too
	}
	if res.Paste {
		a.pasteStep(r.ID)
	}
	if res.RemovedFile >= 0 {
		a.removeFile(r.ID, res.RemovedFile)
	}
	if a.picker.Open {
		a.pickerRoom = r.ID
	}
	if a.pick != nil && a.pickerRoom == r.ID {
		anchor := res.GIFButton
		if (a.picker.Tab == kit.TabEmoji || res.Unified) && res.EmojiButton.Valid() {
			anchor = res.EmojiButton
		}
		m := a.pick.Model(&a.picker)
		if a.reactTo == "" {
			a.fillPacks(r.ID, m) // a reaction is not a custom emoji (yet)
		}
		pr := kit.Picker(c, anchor, &a.picker, m)
		if pr.ImportDiscord {
			a.importDiscordGIFs()
		}
		a.packAct(r.ID, st, pr)
		ch := a.pick.Handle(&a.picker, pr)
		switch {
		case ch.Emoji != "" && a.reactTo != "":
			a.src.React(r.ID, a.reactTo, ch.Emoji)
			a.reactTo, a.picker.Open = "", false
		case ch.Emoji != "":
			st.Text += ch.Emoji
		case ch.Item != nil:
			// A GIF or sticker is uploaded as a picture (its link where
			// that cannot be).
			a.sendPicked(r.ID, *ch.Item)
			a.picker.Open = false
		}
	}
	if !a.picker.Open {
		a.reactTo = ""
	}
	if res.Send && a.editStep(r.ID, st) {
		return // the words replaced a message; nothing new was sent
	}
	if res.Send {
		reply := a.replyTo[r.ID]
		if a.sendFiles(r.ID, reply) {
			reply = "" // the files took the reply; the words follow as their own message
		}
		if hasWords(st.Text) {
			a.src.Send(r.ID, st.Draft(), reply) // with the mentions chosen as links to their people
		}
		st.Text, st.Reply = "", nil
		delete(a.replyTo, r.ID)
		a.listState(r.ID).ScrollToEnd()
	}
}

// ---- Phone ----------------------------------------------------------------

// phoneRoom is a chat on a phone: bubbles on the wallpaper.
func (a *App) phoneRoom(c *ui.Context) {
	p := kit.Of(c)
	r, _ := a.src.Room(a.room)
	ui.Column(c).Grow(1).MinHeight(0).Background(p.Wallpaper).Children(func() {
		sub := ""
		av := kit.AvatarSpec{Name: r.Name, ID: r.ID, Colors: &a.colors, Square: r.Kind != model.KindDM}
		if r.Kind == model.KindDM {
			pp, _ := a.src.Person(r.Partner)
			av = a.avatar(pp, 0)
			switch {
			case pp.Status.Set(a.now):
				sub = pp.Status.Emoji + " " + pp.Status.Text
			case pp.Presence == model.Online:
				sub = "online"
			}
		} else {
			sub = strconv.Itoa(r.Members) + " members"
		}
		unread := 0
		for _, o := range a.src.Rooms() {
			if o.ID != r.ID && !o.Muted {
				unread += o.Unread
			}
		}
		h := kit.PhoneChatHeader(c, kit.ChatHeader{Title: r.Name, Status: sub, Avatar: av, Unread: unread, Muted: r.Muted})
		switch {
		case h.Back:
			a.pushed = false
		case h.Call:
			a.joinCall(r)
		case h.Info:
			if r.Kind == model.KindDM {
				a.profileOpen, a.profileSheet.Open = r.Partner, true
			} else {
				a.panel, a.roomInfo.Open = panelInfo, true
			}
		case h.More:
			a.roomSheet.Open, a.roomSheetID = true, r.ID
		}
		if a.find.Open && a.find.Room == r.ID {
			a.findBar(c) // the same bar as wide windows, under the header
		}
		if a.inCall == r.ID {
			a.callArea(c, r)
		}
		rows := a.rows(r.ID)
		a.rs = a.receiptStateOf(r.ID)
		ls := a.listState(r.ID)
		ls.Key = func(i int) any { return rows[i].key }
		a.jumpStep(r.ID, rows, ls)
		a.readStep(r, rows, ls)
		a.pageStep(r.ID, ls)
		// The messages with the typing bubble floating on their bottom
		// edge: it takes no room, so nothing above it moves when it comes
		// and goes.
		ui.Column(c.Key("ptlarea:"+r.ID)).Grow(1).Children(func() {
			ui.List(c.Key("ptl:"+r.ID), ls, len(rows), func(i int) { a.bubbleRow(c, r, rows, i) }).
				Grow(1).Justify(ui.End).Padding(kit.S2, 0, kit.S4, 0)
			if len(r.Typing) > 0 && r.Kind == model.KindDM {
				pp, _ := a.src.Person(r.Partner)
				ui.Column(c.Key("ptyping")).Attach(ui.AnchorBottomLeft, ui.AnchorBottomLeft).Left(kit.S3).Top(-kit.S1).PassThrough().
					Children(func() { kit.TypingBubble(c, pp.Name, a.avatar(pp, 0), &a.colors) })
			}
			a.seekPill(c, r.ID)
		})
		a.composer(c, r)
	})
}

// bubbleRow is row i of a phone's timeline: a date, a system line, or a
// message bubble.
func (a *App) bubbleRow(c *ui.Context, r model.Room, rows []trow, i int) {
	me := a.src.Me().ID
	tr := rows[i]
	if tr.ev == nil {
		kit.DatePill(c, tr.day)
		return
	}
	ev := *tr.ev
	if ev.Kind == model.EventSystem {
		kit.SystemEvent(c, ev.Body)
		return
	}
	sender, _ := a.src.Person(ev.Sender)
	last := i == len(rows)-1 || rows[i+1].ev == nil || rows[i+1].first
	b := kit.BubbleData{ID: ev.ID, Sender: sender.Name, SenderID: sender.ID, Avatar: a.avatar(sender, 0), Out: ev.Sender == me,
		Time: ev.Time.Format("15:04"), Reactions: reactions(ev.Reactions), Reply: a.quote(r.ID, ev.ReplyTo),
		First: tr.first, Last: last, Pending: ev.Pending, Failed: ev.Failed, Error: ev.Error, Edited: ev.Edited, MaxWidth: 300}
	if a.prefs.receipts {
		b.Read = b.Out && ev.ID == a.rs.readID // your last message that was read
		b.Sent = b.Out && a.rs.sentID != "" && ev.ID == a.rs.sentID
		if r.Kind != model.KindDM { // a DM's "Read" says it all
			b.ReadBy = a.readers(ev)
		}
	}
	if ev.Kind != model.EventUndecryptable {
		var yt *kit.YouTubeSpec
		b.Doc, b.Media, yt = a.content(ev)
		b.File = a.fileSpec(ev)
		if yt == nil && len(b.Media) == 0 {
			b.Embed = a.linkCard(ev, b.Doc)
		}
	}
	b.Menu = a.msgMenu(ev)
	if ev.Poll != nil {
		b.Poll = a.pollData(ev.ID, *ev.Poll, b.Out)
	}
	if !b.Out {
		b.Profile, b.ProfileOpen = a.profile(ev.Sender), a.profileMsg == ev.ID
	}
	ui.Column(c.Key(ev.ID)).Children(func() {
		a.messageAct(r.ID, ev, b.Media, kit.Bubble(c, b, &a.colors, a.reveals))
	})
}

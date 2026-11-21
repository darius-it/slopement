package gallery

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/format"
	"slopement/kit"
)

func muteEnd(a *App, d kit.MuteFor) time.Time {
	if d == 0 {
		return time.Time{}
	}
	return a.now.Add(time.Duration(d))
}

// applyRoom applies a room menu's action to the demo state.
func (a *App) applyRoom(id string, act kit.RoomAction) {
	st := a.roomStates[id]
	switch {
	case act.Mute != nil:
		st.Muted, st.MutedUntil = true, muteEnd(a, *act.Mute)
		a.did = kit.MutedUntilText(st.MutedUntil, a.now)
	case act.Unmute:
		st.Muted = false
		a.did = "Unmuted"
	case act.ToggleFav:
		st.Favourite = !st.Favourite
		a.did = "Favourite toggled"
	case act.ToggleLow:
		st.LowPrio = !st.LowPrio
	case act.MarkRead:
		st.Unread = false
	case act.MarkUnread:
		st.Unread = true
	case act.Leave:
		a.dialog.Open = true
	case act.CopyLink:
		a.did = "Copied link"
	}
}

// rail is the column of spaces.
func (a *App) rail(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Width(kit.RailWidth).Shrink(0).Background(p.BgSubtle).PaddingY(kit.S3).Gap(kit.S2).
		BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
		for i, s := range spaces {
			if kit.RailItem(c, s, a.space == s.ID).Clicked() {
				a.space = s.ID
			}
			if i == 0 {
				kit.RailDivider(c)
			}
		}
		// Right under the spaces it adds to.
		kit.RailAdd(c, &a.railAdd)
		if a.railAdd != kit.RailAddNone {
			a.did = []string{"", "Create a space", "Join a space", "Explore public spaces"}[a.railAdd]
			a.railAdd = kit.RailAddNone
		}
		ui.Spacer(c)
		// The user, at the foot: their avatar sets their status.
		ui.Row(c).Justify(ui.Center).Children(func() {
			me := ui.ButtonBase(c.Key("me")).Radius(kit.RadiusFull).Label("Set a status").Tooltip(a.myStatusTip())
			me.Children(func() {
				kit.Avatar(c, kit.AvatarSpec{Name: "You", ID: "@me:matrix.org", Size: 32, Presence: kit.PresenceOnline, Colors: &a.colors})
				if a.myStatus.Set(a.now) {
					ui.Box(c).Attach(ui.AnchorTopRight, ui.AnchorCenter).Top(4).Left(-4).Children(func() {
						ui.Text(c, a.myStatus.Emoji).FontSize(12)
					})
				}
			})
			if me.Clicked() {
				a.statusDraft.Edit(a.myStatus)
			}
		})
	})
}

func (a *App) spaceName() string {
	for _, s := range spaces {
		if s.ID == a.space {
			return s.Name
		}
	}
	return ""
}

func (a *App) rowData(r kit.RoomRowData) kit.RoomRowData {
	st := a.roomStates[r.ID]
	r.Muted, r.Fav = st.Muted, st.Favourite
	if !st.Unread {
		r.Unread, r.Mention = 0, 0
	} else if r.Unread == 0 && r.Mention == 0 {
		r.Unread = 1
	}
	r.Avatar.Colors = &a.colors
	if r.ID == "!r2" {
		r.Status = alexStatus(a.now) // Alex's DM
	}
	return r
}

// roomList is the panel of the space's rooms.
func (a *App) roomList(c *ui.Context, width float32) {
	p := kit.Of(c)
	col := ui.Column(c).Background(p.BgSubtle).Shrink(0)
	if width > 0 {
		col.Width(width)
	} else {
		col.Grow(1)
	}
	col.Children(func() {
		ui.Row(c).Height(52).PaddingStart(kit.S4).PaddingEnd(kit.S2).AlignItems(ui.Center).Gap(kit.S1).Children(func() {
			ui.Text(c, a.spaceName()).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).SingleLine().Grow(1).MinWidth(0)
			kit.IconButton(c, kit.Ghost, kit.IconPencil, "New message")
		})
		ui.Column(c).PaddingX(kit.S2).Gap(kit.S2).Children(func() {
			if kit.SearchButton(c.Key("rl-search"), "Search", kit.CmdK()).Clicked() {
				a.switcher.Toggle()
			}
			ui.Row(c).Gap(kit.S1).Children(func() {
				for i, l := range []string{"All", "Unread", "People"} {
					if kit.Chip(c.Key("f"+l), l, a.chip == i).Clicked() {
						a.chip = i
					}
				}
			})
		})
		ui.Scroll(c).Grow(1).Padding(kit.S2, kit.S2).Gap(1).Children(func() {
			// People: everyone's status at a glance, above the rooms.
			ppl := ui.ButtonBase(c.Key("people")).Height(36).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusMd - 2).
				AlignItems(ui.Center).Justify(ui.Start).Label("People").AccessSelected(a.room == "people")
			bg, fg := ui.Transparent, p.Text
			switch {
			case a.room == "people":
				bg, fg = p.AccentSubtle, p.AccentStrong
			case ppl.Hovered():
				bg = p.Hover
			}
			ppl.Background(ppl.AnimateColor("bg", bg, kit.Color))
			ppl.Children(func() {
				ui.Box(c).Width(36).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, kit.IconUsers).Size(18, 18).TextColor(fg)
				})
				ui.Text(c, "People").FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).TextColor(fg).Grow(1)
				on := 0
				for _, ps := range a.contacts() {
					if ps.Presence != kit.PresenceNone {
						on++
					}
				}
				ui.Text(c, strconv.Itoa(on)+" online").FontSize(kit.SizeCaption).TextColor(p.TextMuted)
			})
			if ppl.Clicked() {
				a.room = "people"
			}
			fav, dm, rest := []kit.RoomRowData{}, []kit.RoomRowData{}, []kit.RoomRowData{}
			for _, r := range rooms {
				r = a.rowData(r)
				if a.chip == 1 && r.Unread == 0 && r.Mention == 0 {
					continue
				}
				isDM := r.Avatar.Presence != kit.PresenceNone
				if a.chip == 2 && !isDM {
					continue
				}
				switch {
				case r.Fav:
					fav = append(fav, r)
				case isDM:
					dm = append(dm, r)
				default:
					rest = append(rest, r)
				}
			}
			for i, sec := range []struct {
				title string
				rows  []kit.RoomRowData
			}{{"Favourites", fav}, {"People", dm}, {"Rooms", rest}} {
				if len(sec.rows) == 0 {
					continue
				}
				kit.SectionHeader(c, sec.title, &a.sections[i], nil)
				if !a.sections[i] {
					continue
				}
				for _, r := range sec.rows {
					id := r.ID
					st := a.roomStates[id]
					if kit.RoomRow(c, r, a.room == id, func(m *ui.Menu) { a.applyRoom(id, kit.RoomMenu(m, *st, a.now)) }).Clicked() {
						a.room = id
						st.Unread = false
					}
				}
			}
		})
	})
}

func (a *App) currentRoom() kit.RoomRowData {
	for _, r := range rooms {
		if r.ID == a.room {
			return a.rowData(r)
		}
	}
	return rooms[0]
}

// roomHeader is the bar over the timeline.
func (a *App) roomHeader(c *ui.Context, back bool) {
	p := kit.Of(c)
	r := a.currentRoom()
	st := a.roomStates[r.ID]
	ui.Row(c).Height(52).PaddingX(kit.S3).Gap(kit.S2).AlignItems(ui.Center).Shrink(0).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
		if back {
			kit.IconButton(c, kit.Ghost, kit.IconChevronL, "Back")
		}
		dm := r.Avatar.Presence != kit.PresenceNone
		// The avatar and name open the profile card in a DM.
		who := ui.ButtonBase(c.Key("who")).Gap(kit.S2).AlignItems(ui.Center).Justify(ui.Start).Grow(1).MinWidth(0).
			Radius(kit.RadiusSm).Label(r.Name)
		who.Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Colors: &a.colors})
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Row(c).Gap(kit.S15).AlignItems(ui.Center).Children(func() {
					ui.Text(c, r.Name).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).SingleLine().Shrink(1)
					if !r.Encrypted {
						kit.Pill(c, "Not encrypted")
					}
					if st.Muted {
						ui.Icon(c, kit.IconBellOff).Size(12, 12).TextColor(p.TextFaint)
					}
				})
				switch {
				case dm && r.Status.Set(a.now):
					kit.StatusLine(c, r.Status, a.now, false) // under the name
				case !back && !dm:
					ui.Text(c, "Design reviews, every Tuesday · 12 members").FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
				}
			})
		})
		if dm && who.Clicked() {
			a.profileOpen = !a.profileOpen
		}
		if dm {
			ui.PopoverBase(c, who, &a.profileOpen, func(panel ui.Element) {
				panel.Margin(6, 0, 0, 0).Padding(kit.S4).Radius(kit.RadiusLg).Background(p.Raised).Border(1, p.Border).
					Shadow(0, 12, 32, -6, p.Shadow).Transition(kit.PopIn())
				if act := kit.ProfileCard(c, kit.ProfileSpec{Name: r.Name, MXID: "@alex:example.com", Pronouns: "they/them",
					Status: r.Status, Trust: kit.TrustVerified, Avatar: r.Avatar}, &a.colors, a.now); act.Message || act.Call {
					a.profileOpen = false
				}
			})
		}
		kit.IconButton(c, kit.Ghost, kit.IconPhone, "Call")
		if !back {
			kit.IconButton(c, kit.Ghost, kit.IconSearch, "Search in room")
		}
		more := kit.IconButton(c, kit.Ghost, kit.IconMore, "Room options")
		kit.MenuButton(c, more, func(m *ui.Menu) { a.applyRoom(r.ID, kit.RoomMenu(m, *st, a.now)) })
	})
}

// timeline is the demo conversation.
func (a *App) timeline(c *ui.Context) {
	kit.DateSeparator(c, "Today")
	kit.SystemEvent(c, "Mira changed the topic to “Design reviews, every Tuesday”")
	for _, m := range msgs(a.reveals) {
		a.asMedia(&m.Doc, &m.Media, m.ID)
		if m.SenderID != "@me:matrix.org" {
			m.Profile, m.ProfileOpen = a.profileOf(m.SenderID, m.Sender), a.profileMsg == m.ID
		}
		act := kit.Message(c, m, &a.colors)
		a.profileAct(m.ID, act)
		switch {
		case act.Reply:
			body := m.Body
			if m.Doc != nil {
				body = format.Plain(m.Doc)
			}
			a.composer.Reply = &kit.Quote{Sender: m.Sender, SenderID: m.SenderID, Body: body}
		case act.React != "":
			a.did = "Reacted " + act.React
		case act.OpenMedia >= 0:
			a.openViewer(m.Media, act.OpenMedia)
		case act.PlayVideo:
			a.did = "Would play the video inline"
		}
	}
	// Mira's poll, then what the user sent.
	if act := kit.Message(c, kit.MessageData{ID: "poll-msg", Sender: "Mira", SenderID: "@mira:matrix.org", Time: "14:04",
		First: true, Poll: &a.demoPoll}, &a.colors); act.Vote >= 0 || act.EndPoll {
		a.pollAct(&a.demoPoll, act)
	}
	for _, m := range a.sent {
		m.Media = a.live(m.Media)
		ui.Column(c.Key("sent:"+m.ID)).Transition(kit.Rise()).Children(func() { kit.Message(c, m, &a.colors) })
	}
	for _, pd := range a.sentPolls {
		ui.Column(c.Key("sentpoll:"+pd.ID)).Transition(kit.Rise()).Children(func() {
			if act := kit.Message(c, kit.MessageData{ID: pd.ID, Sender: "You", SenderID: "@me:matrix.org",
				Time: a.now.Format("15:04"), First: true, Poll: pd}, &a.colors); act.Vote >= 0 || act.EndPoll {
				a.pollAct(pd, act)
			}
		})
	}
}

// conversation is the header, timeline and composer of the open room.
func (a *App) conversation(c *ui.Context, back bool) {
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		a.roomHeader(c, back)
		ui.Scroll(c).Grow(1).PaddingY(kit.S2).Children(func() { a.timeline(c) })
		kit.TypingLine(c, "Jun is typing…")
		a.composerBox(c)
	})
}

func (a *App) desktopScreen(c *ui.Context) {
	p := kit.Of(c)
	layout := a.navLayout.Resolve(false)
	ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Layout: "+kit.NavLayouts[layout].Name+" (from Settings → Appearance, or View → Layout). Try "+kit.CmdK()+", right-click a room, hover a message, the GIF button.").
			FontSize(kit.SizeSmall).TextColor(p.TextMuted).Grow(1)
	})
	// The main area: the People view, or the conversation.
	pane := func() {
		if a.room == "people" {
			a.peopleScreen(c)
		} else {
			a.conversation(c, false)
		}
	}
	frame(c, "desk", 720, func() {
		switch layout {
		case kit.NavTabs:
			a.deskA(c, pane)
		case kit.NavInbox:
			a.deskC(c, pane)
		default:
			a.deskB(c, pane)
		}
	})
	a.overlays(c)
}

// phoneHome is the phone's first screen, in the layout chosen: tabs
// (Chats, Communities, People) or the one inbox.
func (a *App) phoneHome(c *ui.Context) {
	if a.navLayout.Resolve(true) == kit.NavInbox {
		a.phoneC(c)
		return
	}
	a.phoneA(c, a.phoneTab)
}

func (a *App) phoneScreen(c *ui.Context) {
	p := kit.Of(c)
	ui.Text(c, "Phone, layout "+kit.NavLayouts[a.navLayout.Resolve(true)].Name+" (Settings → Appearance): the home, and a chat in bubbles on a soft wallpaper (after KakaoTalk, LINE and Signal on Mobbin). Type and send: it pops in. The smiley opens emoji, stickers and GIFs.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	ui.Row(c).Gap(kit.S6).Wrap().Children(func() {
		phone(c, "chats", func() { a.phoneHome(c) })
		phone(c, "chat", func() { a.phoneChat(c) })
	})
	a.overlays(c)
}

// phoneChat is a DM in bubbles.
func (a *App) phoneChat(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).Background(p.Wallpaper).Children(func() {
		status := "online"
		if s := alexStatus(a.now); s.Set(a.now) {
			status = s.Emoji + " " + s.Text // under the name, as the desktop header
		}
		kit.PhoneChatHeader(c, kit.ChatHeader{Title: "Alex Rivera", Status: status, Unread: 8,
			Avatar: kit.AvatarSpec{ID: "@alex:example.com", Presence: kit.PresenceOnline, Colors: &a.colors}})
		ui.Scroll(c).Grow(1).PaddingY(kit.S2).Children(func() {
			kit.DatePill(c, "Today")
			list := bubbles()
			for i, b := range list {
				if i == 5 {
					kit.UnreadDivider(c, 2)
				}
				b.MaxWidth = 280
				a.asMedia(&b.Doc, &b.Media, b.ID)
				if !b.Out {
					b.Profile, b.ProfileOpen = a.profileOf(b.SenderID, b.Sender), a.profileMsg == b.ID
				}
				act := kit.Bubble(c, b, &a.colors, a.reveals)
				a.profileAct(b.ID, act)
				if act.OpenMedia >= 0 {
					a.openViewer(b.Media, act.OpenMedia)
				} else if act.More {
					a.did = "Message actions"
				}
			}
			for _, b := range a.sentBubbles {
				b.MaxWidth = 280
				b.Media = a.live(b.Media)
				if b.Sticker != nil {
					s := a.live([]kit.MediaSpec{*b.Sticker})[0]
					b.Sticker = &s
				}
				kit.Bubble(c, b, &a.colors, a.reveals)
			}
			for _, pd := range a.sentPolls {
				if act := kit.Bubble(c, kit.BubbleData{ID: "pb:" + pd.ID, Out: true, First: true, Last: true, Live: true,
					Time: a.now.Format("15:04"), Poll: pd, MaxWidth: 280}, &a.colors, a.reveals); act.Vote >= 0 || act.EndPoll {
					a.pollAct(pd, act)
				}
			}
			kit.TypingBubble(c, "Alex", kit.AvatarSpec{ID: "@alex:example.com", Name: "Alex Rivera"}, &a.colors)
			ui.Box(c).Height(kit.S2)
		})
		r := kit.PhoneComposer(c.Key("phone-composer"), &a.phoneComposer, "Message", &a.colors)
		a.openPicker(r)
		if r.Poll {
			a.pollSheet.Open = true
		}
		if ch := a.runPicker(c, r.EmojiButton); ch.Emoji != "" {
			a.phoneComposer.Text += ch.Emoji
		} else if ch.Item != nil {
			a.sendItem(*ch.Item)
		}
		if r.Send {
			a.sentBubbles = append(a.sentBubbles, kit.BubbleData{ID: "pb" + strconv.Itoa(len(a.sentBubbles)), Out: true, Live: true,
				Time: a.now.Format("15:04"), Doc: format.Parse(a.phoneComposer.Text), First: true, Last: true})
			a.phoneComposer.Text = ""
		}
	})
}

func phone(c *ui.Context, key string, fn func()) {
	p := kit.Of(c)
	ui.Column(c.Key(key)).Width(390).Height(780).Radius(36).Border(6, p.Text.Alpha(0.85)).Background(p.Bg).Clip().
		Children(func() {
		// The status bar.
		ui.Row(c).Height(44).PaddingX(kit.S6).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, "9:41").FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold).Grow(1)
			ui.Box(c).Size(24, 11).Radius(3).Border(1, p.Text)
		})
		fn()
	})
}


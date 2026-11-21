package gallery

import (
	"time"

	"github.com/egoist/mygo/ui"

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
		a.dialog = true
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
		ui.Spacer(c)
		ui.Row(c).Justify(ui.Center).Children(func() {
			kit.IconButton(c, kit.Ghost, kit.IconPlus, "Create or join a space")
		})
		ui.Row(c).Justify(ui.Center).Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: "You", ID: "@me:matrix.org", Size: 32, Presence: kit.PresenceOnline, Colors: &a.colors})
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
		kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Colors: &a.colors})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(kit.S15).AlignItems(ui.Center).Children(func() {
				ui.Text(c, r.Name).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).SingleLine().Shrink(1)
				if r.Encrypted {
					ui.Icon(c, kit.IconLock).Size(12, 12).TextColor(p.TextFaint)
				}
				if st.Muted {
					ui.Icon(c, kit.IconBellOff).Size(12, 12).TextColor(p.TextFaint)
				}
			})
			if !back {
				ui.Text(c, "Design reviews, every Tuesday · 12 members").FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
			}
		})
		kit.IconButton(c, kit.Ghost, kit.IconPhone, "Call")
		if !back {
			kit.IconButton(c, kit.Ghost, kit.IconSearch, "Search in room")
		}
		more := kit.IconButton(c, kit.Ghost, kit.IconMore, "Room options")
		more.PopoverMenuButton(func(m *ui.Menu) { a.applyRoom(r.ID, kit.RoomMenu(m, *st, a.now)) },
			ui.PopoverMenuButtonOptions{Panel: kit.MenuStyle(c).Panel})
	})
}

// timeline is the demo conversation.
func (a *App) timeline(c *ui.Context) {
	kit.DateSeparator(c, "Today")
	kit.SystemEvent(c, "Mira changed the topic to “Design reviews, every Tuesday”")
	for _, m := range msgs() {
		if act := kit.Message(c, m, &a.colors); act.Reply {
			a.composer.Reply = &kit.Quote{Sender: m.Sender, SenderID: m.SenderID, Body: m.Body}
		} else if act.React != "" {
			a.did = "Reacted " + act.React
		}
	}
	for _, m := range a.sent {
		ui.Column(c.Key("sent:"+m.ID)).Transition(kit.Rise()).Children(func() { kit.Message(c, m, &a.colors) })
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
	ui.Text(c, "Rail · room list · conversation, built from kit parts only. Try "+kit.CmdK()+", right-click a room, hover a message, the GIF button.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	ui.Row(c).Height(720).Radius(kit.RadiusLg).Border(1, p.Border).Clip().AlignItems(ui.Stretch).Background(p.Bg).
		Shadow(0, 24, 64, -24, p.Shadow).Children(func() {
		a.rail(c)
		a.roomList(c, kit.RoomListWidth)
		ui.Box(c).Width(1).Background(p.Border)
		a.conversation(c, false)
	})
	a.overlays(c)
}

func (a *App) phoneScreen(c *ui.Context) {
	p := kit.Of(c)
	ui.Text(c, "Phone: the chats list (spaces as a row on top, a calm large title, filter chips) and a conversation pushed over it.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	ui.Row(c).Gap(kit.S6).Wrap().Children(func() {
		phone(c, "chats", func() { a.phoneChats(c) })
		phone(c, "chat", func() { a.conversation(c, true) })
	})
	a.overlays(c)
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

func (a *App) phoneChats(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).Children(func() {
		ui.Row(c).PaddingX(kit.S4).Height(52).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Chats").FontSize(kit.SizeDisplay).FontWeight(kit.WeightSemibold).Grow(1)
			kit.IconButton(c, kit.Ghost, kit.IconPencil, "New message")
		})
		ui.Column(c).PaddingX(kit.S4).Gap(kit.S3).PaddingY(kit.S2).Children(func() {
			if kit.SearchButton(c.Key("ph-search"), "Search", "").Clicked() {
				a.switcher.Toggle()
			}
			ui.Scroll(c).Row().Gap(kit.S3).Scrollbars(false).Children(func() {
				for _, s := range spaces {
					s := s
					sel := a.space == s.ID
					b := ui.ButtonBase(c.Key("ps"+s.ID)).Width(56).Gap(kit.S1).Column().AlignItems(ui.Center).Label(s.Name)
					b.Children(func() {
						if s.Icon != nil {
							bg, fg := p.BgInset, p.TextMuted
							if sel {
								bg, fg = p.Accent, p.OnAccent
							}
							ui.Box(c).Size(44, 44).Radius(12).Background(bg).AlignItems(ui.Center).Justify(ui.Center).
								Children(func() { ui.Icon(c, s.Icon).Size(20, 20).TextColor(fg) })
						} else {
							ui.Box(c).Size(44, 44).Children(func() {
								kit.Avatar(c, kit.AvatarSpec{Name: s.Name, ID: s.ID, Size: 44, Square: true})
								if sel {
									ui.Box(c).Size(44, 44).Radius(12).Border(2, p.Accent).Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).PassThrough()
								}
							})
						}
						fg := p.TextMuted
						if sel {
							fg = p.Text
						}
						ui.Text(c, s.Name).FontSize(kit.SizeCaption).TextColor(fg).SingleLine().MaxWidth(56)
					})
					if b.Clicked() {
						a.space = s.ID
					}
				}
			})
			ui.Row(c).Gap(kit.S1 + 2).Children(func() {
				for i, l := range []string{"All", "Unread", "People"} {
					if kit.Chip(c.Key("pf"+l), l, a.chip == i).Clicked() {
						a.chip = i
					}
				}
			})
		})
		ui.Scroll(c).Grow(1).PaddingX(kit.S2).Children(func() {
			for _, r := range rooms {
				r = a.rowData(r)
				if kit.RoomRow(c, r, false, nil).Clicked() {
					a.room = r.ID
				}
			}
		})
		// A tab bar, as the system draws one on phones (mygo.NewTabs).
		ui.Row(c).Height(72).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Background(p.BgSubtle).
			Padding(kit.S2, 0, 0, 0).Children(func() {
			for i, t := range []struct {
				n string
				i *ui.SVG
			}{{"Chats", kit.IconMessage}, {"Calls", kit.IconPhone}, {"Search", kit.IconSearch}, {"Settings", kit.IconSettings}} {
				fg := p.TextMuted
				if i == 0 {
					fg = p.AccentStrong
				}
				ui.Column(c.Key(t.n)).Grow(1).Gap(2).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, t.i).Size(22, 22).TextColor(fg)
					ui.Text(c, t.n).FontSize(10).FontWeight(kit.WeightMedium).TextColor(fg)
				})
			}
		})
	})
}

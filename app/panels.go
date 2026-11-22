package app

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// sidePanel is the right panel: room info or members. It stays open when
// switching rooms, showing the new room's.
func (a *App) sidePanel(c *ui.Context) {
	p := kit.Of(c)
	r, _ := a.src.Room(a.room)
	width := float32(300)
	if a.panel == panelThread {
		width = 380 // a thread holds whole messages: room for their text
	}
	ui.Column(c.Key("panel")).Width(width).Shrink(0).Background(p.BgSubtle).BorderWidth(0, 0, 0, 1).BorderColor(p.Border).
		Transition(ui.ElementTransition{Duration: kit.DurBase, ExitDuration: kit.DurFast, Width: true}).Children(func() {
		ui.Row(c).Height(52).PaddingStart(kit.S4).PaddingEnd(kit.S2).AlignItems(ui.Center).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
			title := "Room info"
			switch a.panel {
			case panelMembers:
				title = "Members"
			case panelPins:
				title = "Pinned messages"
			case panelThread:
				title = "Thread"
			}
			ui.Text(c, title).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).Grow(1)
			if kit.IconButton(c, kit.Ghost, kit.IconX, "Close the panel").Clicked() {
				a.panel = panelNone
			}
		})
		if a.panel == panelThread { // the thread scrolls as a list of its own
			a.threadPanel(c, r)
			return
		}
		ui.Scroll(c.Key("panel:" + r.ID)).Grow(1).Padding(kit.S3).Gap(kit.S3).Children(func() {
			switch a.panel {
			case panelMembers:
				a.members(c, r)
			case panelPins:
				a.pinsPanel(c, r)
			default:
				a.roomInfoBody(c, r)
			}
		})
	})
}

// roomInfoBody is a room's details and settings.
func (a *App) roomInfoBody(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	ui.Column(c).AlignItems(ui.Center).Gap(kit.S2).Padding(kit.S2).Children(func() {
		kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 64, Square: r.Kind != model.KindDM, Colors: &a.colors})
		ui.Text(c, r.Name).FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold)
		if r.Topic != "" {
			ui.Text(c, r.Topic).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		}
		ui.Row(c).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
			ic, txt := kit.IconLock, "Encrypted"
			if !r.Encrypted {
				ic, txt = kit.IconGlobe, "Not encrypted"
			}
			ui.Icon(c, ic).Size(12, 12).TextColor(p.TextMuted)
			ui.Text(c, txt+" · "+strconv.Itoa(r.Members)+" members").FontSize(kit.SizeCaption).TextColor(p.TextMuted)
		})
		// Muted: for how long, right under the name.
		if r.Muted {
			ui.Row(c).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, kit.IconBellOff).Size(12, 12).TextColor(p.TextMuted)
				ui.Text(c, kit.MutedUntilText(r.MutedUntil, a.now)).FontSize(kit.SizeCaption).TextColor(p.TextMuted)
			})
		}
	})
	// The actions, as small tiles: Mute (a menu of durations) or Unmute,
	// Copy link, Invite.
	ui.Row(c).Gap(kit.S2).Children(func() {
		if r.Muted {
			if actionTile(c, kit.IconBell, "Unmute").Clicked() {
				a.src.Unmute(r.ID)
			}
		} else {
			id := r.ID
			kit.MenuButton(c, actionTile(c, kit.IconBellOff, "Mute"), func(m *ui.Menu) {
				for _, ch := range kit.MuteChoices {
					if m.Item(ch.Label).Chosen() {
						d := ch.For
						a.roomAction(id, kit.RoomAction{Mute: &d})
					}
				}
			})
		}
		id := r.ID
		kit.MenuButton(c, actionTile(c, kit.IconBell, "Notify"), func(m *ui.Menu) {
			for i, name := range kit.NotifyLevelNames {
				if m.Item(name).Radio(int(r.Notify) == i).Chosen() {
					a.roomAction(id, kit.RoomAction{SetLevel: true, Level: i})
				}
			}
		})
		if actionTile(c, kit.IconLink, "Copy link").Clicked() {
			a.roomAction(id, kit.RoomAction{CopyLink: true})
		}
		if actionTile(c, kit.IconUser, "Invite").Clicked() {
			a.openInvite(r.ID)
		}
		if r.Kind != model.KindDM && a.canEditRooms() && actionTile(c, kit.IconPencil, "Edit").Clicked() {
			a.openRoomEdit(r.ID)
		}
	})
	ui.Column(c).Radius(kit.RadiusMd).Background(p.Raised).Border(1, p.Border).Padding(kit.S1).Children(func() {
		fav := r.Favourite
		if switchRow(c, "Favourite", "At the top of Chats", &fav) {
			a.src.SetFavourite(r.ID, fav)
		}
		low := r.LowPriority
		if switchRow(c, "Low priority", "At the bottom", &low) {
			a.src.SetLowPriority(r.ID, low)
		}
	})
	if kit.Button(c, kit.Danger, "Leave", kit.IconLogOut).Clicked() {
		a.leaving, a.leave.Open = r.ID, true
	}
}

// actionTile is a small action button: an icon over its label, on the
// raised surface, sharing a row equally with its neighbours.
func actionTile(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	p := kit.Of(c)
	b := ui.ButtonBase(c.Key("tile:"+label)).Grow(1).Basis(0).Column().Gap(kit.S1).Padding(kit.S2, kit.S1).Radius(kit.RadiusMd).
		AlignItems(ui.Center).Label(label)
	bg := p.Raised
	switch {
	case b.Pressed():
		bg = p.Pressed
	case b.Hovered():
		bg = p.Hover
	}
	b.Background(b.AnimateColor("bg", bg, kit.Color)).Border(1, p.Border)
	b.Children(func() {
		ui.Icon(c, icon).Size(18, 18).TextColor(p.Text)
		ui.Text(c, label).FontSize(kit.SizeCaption).FontWeight(kit.WeightMedium).TextColor(p.Text).SingleLine()
	})
	return kit.Pressable(b)
}

// switchRow is a setting with a switch; it reports a change.
// soonMsg is what a setting that is not built yet answers to a press.
const soonMsg = "Coming soon"

// soonSwitch is a switch setting that is not built yet: faded, with "Soon"
// in place of the switch, and a press says so.
func (a *App) soonSwitch(c *ui.Context, title, sub string) {
	p := kit.Of(c)
	row := ui.ButtonBase(c.Key("soon:"+title)).Opacity(0.55).Cursor(ui.CursorDefault).MinHeight(44).
		Padding(kit.S2, kit.S3).Gap(kit.S3).AlignItems(ui.Center).Label(title + ", coming soon")
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(kit.SizeBody).TextColor(p.TextMuted)
			if sub != "" {
				ui.Text(c, sub).FontSize(kit.SizeSmall).TextColor(p.TextFaint)
			}
		})
		kit.SoonTag(c)
	})
	if row.Clicked() {
		a.toast = soonMsg
	}
}

func switchRow(c *ui.Context, title, sub string, on *bool) (changed bool) {
	p := kit.Of(c)
	was := *on
	ui.Row(c.Key(title)).MinHeight(44).Padding(kit.S2, kit.S3).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(kit.SizeBody).TextColor(p.Text)
			if sub != "" {
				ui.Text(c, sub).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			}
		})
		ui.Switch(c, on).Label(title)
	})
	return *on != was
}

// members lists who is in the room (the mock: everyone known).
func (a *App) members(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	list := []model.Person{a.src.Me()}
	if r.Kind == model.KindDM {
		pp, _ := a.src.Person(r.Partner)
		list = append(list, pp)
	} else {
		list = append(list, a.src.People()...)
	}
	ui.Text(c, strconv.Itoa(r.Members)+" members").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted)
	for _, ps := range list {
		b := ui.ButtonBase(c.Key("m:" + ps.ID)).Height(40).PaddingX(kit.S2).Gap(kit.S2).Radius(kit.RadiusSm).AlignItems(ui.Center).
			Justify(ui.Start).Label(ps.Name)
		if b.Hovered() {
			b.Background(p.Hover)
		}
		b.Children(func() {
			kit.Avatar(c, a.avatar(ps, 28))
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, ps.Name).FontSize(kit.SizeBody).TextColor(a.colors.Color(p, ps.ID)).SingleLine()
				if ps.Status.Set(a.now) {
					kit.StatusLine(c, status(ps.Status), a.now, false)
				}
			})
		})
		if b.Clicked() && ps.ID != a.src.Me().ID {
			a.profileOpen, a.profileSheet.Open = ps.ID, true
		}
	}
}

// ---- Calls (scaffold) -----------------------------------------------------

// joinCall joins (or starts) the room's call. The scaffold shows the call
// area with the control bar; MatrixRTC with the Jitsi fallback comes with
// the calls layer (docs/calls-research.md).
func (a *App) joinCall(r model.Room) {
	if a.inCall == r.ID {
		return
	}
	a.inCall = r.ID
	a.call = kit.NewCall() // mic on, camera off
	a.toast = "Calls are coming soon (this is how they will look)"
}

// callArea is the call over the timeline: tiles and the control bar.
func (a *App) callArea(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	ui.Column(c.Key("call")).Height(260).Shrink(0).Background(p.BgInset).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).
		Padding(kit.S3).Gap(kit.S3).Transition(ui.ElementTransition{Duration: kit.DurBase, ExitDuration: kit.DurFast,
		Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).Children(func() {
		ui.Row(c).Grow(1).Gap(kit.S3).Justify(ui.Center).Children(func() {
			who := []model.Person{a.src.Me()}
			if r.Kind == model.KindDM {
				pp, _ := a.src.Person(r.Partner)
				who = append(who, pp)
			}
			for _, ps := range who {
				ui.Column(c.Key("tile:"+ps.ID)).Grow(1).MaxWidth(320).Radius(kit.RadiusLg).Background(p.Raised).Border(1, p.Border).
					AlignItems(ui.Center).Justify(ui.Center).Gap(kit.S2).Children(func() {
					kit.Avatar(c, a.avatar(ps, 64))
					ui.Text(c, ps.Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium)
				})
			}
		})
		ui.Row(c).Justify(ui.Center).Children(func() {
			if kit.CallBar(c, &a.call).Leave {
				a.inCall = ""
			}
		})
	})
}

// ---- People -----------------------------------------------------------------

// peopleView is People: the grid on desktops, the list on phones.
func (a *App) peopleView(c *ui.Context, back bool) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		ui.Row(c).Height(52).PaddingX(kit.S4).Gap(kit.S2).AlignItems(ui.Center).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
			if back && kit.IconButton(c, kit.Ghost, kit.IconChevronL, "Back").Clicked() {
				a.view = viewRoom
			}
			ui.Icon(c, kit.IconUsers).Size(18, 18).TextColor(p.TextMuted)
			ui.Text(c, "People").FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold).Grow(1)
			if kit.Button(c, kit.Secondary, "Add person", kit.IconPlus).Clicked() {
				a.openAddPerson()
			}
		})
		ui.Scroll(c).Grow(1).Padding(kit.S4).Children(func() {
			var act kit.PeopleAction
			a.people.Sub = a.prefs.peopleSub
			if kit.Compact(c) {
				act = kit.People(c, &a.people, a.contacts(), &a.colors, a.now)
			} else {
				act = kit.PeopleGrid(c, &a.people, a.contacts(), &a.colors, a.now)
			}
			switch {
			case act.Open != "":
				a.openDM(act.Open, a.open)
				a.community = ""
			case act.Profile != "":
				a.profileOpen, a.profileSheet.Open = act.Profile, true
			}
		})
	})
}

// clearAt is when a status set now with the editor's choice i clears.
func clearAt(i int, now time.Time) time.Time { return kit.ClearTime(i, now) }

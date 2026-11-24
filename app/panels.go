package app

import (
	"context"
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
	r, _ := a.roomByID(a.room)
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
		if a.panel == panelMembers { // a long list: only the rows in view are built
			a.membersPanel(c, r)
			return
		}
		ui.Scroll(c.Key("panel:" + r.ID)).Grow(1).Padding(kit.S3).Gap(kit.S3).Children(func() {
			switch a.panel {
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
	if cc, ok := a.src.(model.CallCleaner); ok && a.inCall != r.ID {
		if kit.ChoiceRow(c, kit.IconPhone, "Clear my call from this room", "If a call you left still shows as going on").Clicked() {
			a.clearCall(cc, r.ID)
		}
	}
	if kit.Button(c, kit.Danger, "Leave", kit.IconLogOut).Clicked() {
		a.leaving, a.leave.Open = r.ID, true
	}
}

// clearCall removes the user's call memberships from a room (a call that
// never ended), off the UI thread, and says what came of it.
func (a *App) clearCall(cc model.CallCleaner, room string) {
	run := a.runUI
	if run == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		n, err := cc.ClearCallMembership(ctx, room, false)
		run(func() {
			switch {
			case err != nil:
				a.toast = "Could not clear the call: " + err.Error()
			case n == 0:
				a.toast = "You are not in a call in this room"
			default:
				a.toast = "Your call in this room is cleared"
			}
		})
	}()
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

// roomMembers are the people in a room, in an order that stays put: the
// source's (by name; the server's own list, fetched once), else — a source
// that cannot list them — the user and, in a chat with one person, that person.
func (a *App) roomMembers(r model.Room) []model.Person {
	var list []model.Person
	if rm, ok := a.src.(model.RoomMembers); ok {
		list = append(list, rm.RoomMembers(r.ID)...)
	} else {
		list = []model.Person{a.src.Me()}
		if r.Kind == model.KindDM {
			pp, _ := a.src.Person(r.Partner)
			list = append(list, pp)
		}
	}
	for i := range list {
		list[i].Name = a.nameFor(list[i].ID, list[i].Name) // nickname or username, as chosen
	}
	return list
}

// membersPanel is the side panel's list of the room's people: a count, then
// one row each, only those in view built.
func (a *App) membersPanel(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	list := a.roomMembers(r)
	count := r.Members
	if count == 0 || len(list) > count {
		count = len(list)
	}
	if a.membersRoom != r.ID {
		a.membersRoom, a.membersLS = r.ID, ui.ListState{} // each room starts at its top
	}
	ls := &a.membersLS
	ls.Key = func(i int) any {
		if i < len(list) {
			return list[i].ID
		}
		return i
	}
	ui.Text(c, strconv.Itoa(count)+" members").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).
		Padding(kit.S3, kit.S3, kit.S1, kit.S3)
	ui.List(c.Key("members:"+r.ID), ls, len(list), func(i int) {
		if i < len(list) {
			a.memberRow(c, list[i])
		}
	}).Grow(1).PaddingX(kit.S3).Gap(1)
	if len(list) == 0 {
		ui.Text(c, "Loading…").FontSize(kit.SizeSmall).TextColor(p.TextMuted).Padding(kit.S3)
	}
}

// memberRow is one person of the room: their picture, name and status.
func (a *App) memberRow(c *ui.Context, ps model.Person) {
	p := kit.Of(c)
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

// ---- Calls (scaffold) -----------------------------------------------------

// joinCall joins (or starts) the room's call. The scaffold shows the call
// area with the control bar; MatrixRTC with the Jitsi fallback comes with
// the calls layer (docs/calls-research.md).
func (a *App) joinCall(r model.Room) {
	if a.inCall == r.ID {
		return
	}
	if a.inCall != "" { // one call at a time: the other ends
		a.endCall(a.callSess)
	}
	if a.startCall(r) {
		return
	}
	a.inCall = r.ID
	a.call = kit.NewCall() // mic on, camera off
	a.toast = "Calls are coming soon (this is how they will look)"
}

// callArea is the call over the timeline: the call page, or (a source that
// cannot run calls) the scaffold's tiles and control bar. fixed is a height
// for the page (0: the room a split gives it).
func (a *App) callArea(c *ui.Context, r model.Room, fixed float32) {
	p := kit.Of(c)
	if s := a.callSess; s != nil && s.room == r.ID {
		a.realCallArea(c, s, fixed)
		return
	}
	ui.Column(c.Key("call")).Height(260).Shrink(0).Background(p.BgInset).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).
		Padding(kit.S3).Gap(kit.S3).Transition(ui.ElementTransition{Duration: kit.DurBase, ExitDuration: kit.DurFast,
		Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).Children(func() {
		ui.Row(c).Grow(1).Gap(kit.S3).Justify(ui.Center).Children(func() {
			who := []model.Person{a.src.Me()}
			if r.Kind == model.KindDM {
				pp, _ := a.person(r.Partner)
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
			// Your own server's people first; the others and the bridges' are one press away.
			_, hidden := a.contactsFor(false)
			label := "Show other servers and bridges (" + strconv.Itoa(hidden) + ")"
			if a.peopleAll {
				label = "Only my server"
			}
			compact := kit.Compact(c)
			if !compact && (hidden > 0 || a.peopleAll) && kit.Button(c, kit.Ghost, label, nil).Clicked() {
				a.peopleAll = !a.peopleAll
			}
			if compact { // a phone's header has room for the title and one icon
				if kit.IconButton(c, kit.Ghost, kit.IconPlus, "Add person").Clicked() {
					a.openAddPerson()
				}
			} else if kit.Button(c, kit.Secondary, "Add person", kit.IconPlus).Clicked() {
				a.openAddPerson()
			}
		})
		if _, hidden := a.contactsFor(false); kit.Compact(c) && (hidden > 0 || a.peopleAll) {
			label := "Show other servers and bridges (" + strconv.Itoa(hidden) + ")"
			if a.peopleAll {
				label = "Only my server"
			}
			ui.Row(c).PaddingX(kit.S3).PaddingY(kit.S1).Shrink(0).Children(func() {
				if kit.Button(c, kit.Ghost, label, nil).Clicked() {
					a.peopleAll = !a.peopleAll
				}
			})
		}
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

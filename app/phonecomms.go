package app

import (
	"sort"
	"strconv"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// phoneCommunities is a phone's Communities tab: every community as a card
// on one page, its rooms with something new under its name, as WhatsApp's
// Communities tab lists each community with its groups (Mobbin:
// https://mobbin.com/screens/0e37aaa5-ea77-4797-8b5f-6d9bb93e76f2). A room is
// one tap from the overview; "All rooms" unfolds the rest in place. No
// page to go into, so no Back.
func (a *App) phoneCommunities(c *ui.Context) {
	comms := a.communities()
	if len(comms) == 0 {
		a.phoneNoCommunities(c)
		return
	}
	ui.Scroll(c.Key("comm-cards")).Grow(1).MinHeight(0).Padding(kit.S1, kit.S3, kit.S6, kit.S3).Gap(kit.S3).Children(func() {
		for i, cm := range comms {
			if i == 0 && cm.Favourite {
				a.sectionHeader(c, "Favourites", kit.IconStar)
			}
			if i > 0 && comms[i-1].Favourite && !cm.Favourite {
				a.sectionHeader(c, "All communities", nil)
			}
			a.phoneCommunityCard(c, cm)
		}
		if kit.Button(c, kit.Secondary, "Join or create a community", kit.IconPlus).Clicked() {
			a.openRooms(2, 0)
		}
	})
}

// phoneCommunityPeek is how many rooms a folded card shows.
const phoneCommunityPeek = 3

// phoneCommunityCard is one community: its picture, name and who is in it
// (a tap folds or unfolds it; ⋯ and a long press give its options), then
// its rooms, those with something new first.
func (a *App) phoneCommunityCard(c *ui.Context, cm model.Community) {
	p := kit.Of(c)
	key := "phone:" + cm.ID // a.expanded is kept between runs
	open := a.expanded[key]
	ids := a.communityRoomIDs(cm, 0)
	ui.Column(c.Key("card:"+cm.ID)).Background(p.BgSubtle).Radius(kit.RadiusLg).Border(1, p.Border).Clip().Children(func() {
		// The head.
		unread, ment := a.communityCounts(cm)
		ui.Row(c).Padding(kit.S2, kit.S1, kit.S2, kit.S3).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
			head := ui.ButtonBase(c.Key("head")).Grow(1).MinWidth(0).Gap(kit.S3).AlignItems(ui.Center).Justify(ui.Start).
				MinHeight(kit.TouchTarget).Label(cm.Name).Expanded(open)
			head.Children(func() {
				kit.Avatar(c, kit.AvatarSpec{Name: cm.Name, ID: cm.ID, Size: 44, Square: true, Image: a.avatarPic(cm.Avatar)})
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, cm.Name).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).SingleLine()
					ui.RichText(c,
						ui.Span{Text: "● ", Color: p.Success},
						ui.Span{Text: compact(cm.Online) + " online · " + strconv.Itoa(len(ids)) + " rooms", Color: p.TextMuted},
					).FontSize(kit.SizeSmall).SingleLine()
				})
				switch {
				case ment > 0:
					kit.Count(c, ment, true)
				case unread:
					kit.Dot(c, false)
				}
			})
			kit.ContextMenu(c, head, func(m *ui.Menu) { a.communityMenu(cm, m) })
			if head.Clicked() {
				a.setExpanded(key, !open)
			}
			more := kit.IconButton(c, kit.Ghost, kit.IconMore, cm.Name+" options")
			kit.MenuButton(c, more, func(m *ui.Menu) {
				if m.Item("Invite people").Chosen() {
					a.openInvite(cm.ID)
				}
				m.Separator()
				a.communityMenu(cm, m)
			})
		})
		ui.Box(c).Height(1).Background(p.Border)
		// The rooms.
		ui.Column(c).Padding(kit.S1, kit.S1).Children(func() {
			if open {
				a.phoneCommunityRoomsAt(c, cm, 0) // every room, in the space's order
				a.joinableRooms(c, cm)            // then what it offers to join
			} else {
				for _, r := range a.peekRooms(ids) {
					a.phoneRoomRow(c, r)
				}
			}
			if len(ids) > phoneCommunityPeek || open {
				label := "All " + strconv.Itoa(len(ids)) + " rooms"
				icon := kit.IconChevronD
				if open {
					label, icon = "Show less", kit.IconChevronU
				}
				t := ui.ButtonBase(c.Key("toggle")).Height(kit.TouchTarget).PaddingX(kit.S2).Gap(kit.S2).Radius(kit.RadiusSm).
					AlignItems(ui.Center).Justify(ui.Start).Label(label).Expanded(open)
				t.Children(func() {
					ui.Icon(c, icon).Size(16, 16).TextColor(p.AccentStrong)
					ui.Text(c, label).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).TextColor(p.AccentStrong)
				})
				if t.Clicked() {
					a.setExpanded(key, !open)
				}
			}
		})
	})
}

// phoneCommunityRoomsAt is an unfolded card's rooms, in the space's order,
// each sub-space's under its name (a level deeper, never past four), in the
// same rows as the folded card's.
func (a *App) phoneCommunityRoomsAt(c *ui.Context, cm model.Community, depth int) {
	for _, id := range cm.Rooms {
		if r, ok := a.roomByID(id); ok {
			a.phoneRoomRow(c, r)
		}
	}
	if depth >= 3 {
		return
	}
	for _, sid := range cm.Spaces {
		if sub, ok := a.communityOf(sid); ok {
			a.sectionHeader(c, sub.Name, nil)
			a.phoneCommunityRoomsAt(c, sub, depth+1)
		}
	}
}

// setExpanded folds or unfolds a group, kept with the app's state.
func (a *App) setExpanded(key string, open bool) {
	if a.expanded == nil {
		a.expanded = map[string]bool{}
	}
	if open {
		a.expanded[key] = true
	} else {
		delete(a.expanded, key)
	}
}

// peekRooms are a folded card's rooms: those with mentions, then unread,
// then the most recently active, at most phoneCommunityPeek.
func (a *App) peekRooms(ids []string) []model.Room {
	rooms := make([]model.Room, 0, len(ids))
	for _, id := range ids {
		if r, ok := a.roomByID(id); ok {
			rooms = append(rooms, r)
		}
	}
	rank := func(r model.Room) int {
		switch {
		case r.Muted:
			return 0
		case r.Mentions > 0:
			return 3
		case r.Unread > 0:
			return 2
		}
		return 1
	}
	sort.SliceStable(rooms, func(i, j int) bool {
		if ri, rj := rank(rooms[i]), rank(rooms[j]); ri != rj {
			return ri > rj
		}
		return rooms[i].LastActivity.After(rooms[j].LastActivity)
	})
	return rooms[:min(len(rooms), phoneCommunityPeek)]
}

// phoneRoomRow is a room in a folded card: # and its name, its last message
// under it, when, and what is waiting — as a chat in the Chats list.
func (a *App) phoneRoomRow(c *ui.Context, r model.Room) {
	p := kit.Of(c)
	unread := (r.Unread > 0 || r.Mentions > 0) && !r.Muted
	b := ui.ButtonBase(c.Key("peek:"+r.ID)).MinHeight(56).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusSm).
		AlignItems(ui.Center).Justify(ui.Start).Label(r.Name)
	kit.Pressable(b)
	b.Children(func() {
		ui.Box(c).Size(36, 36).Radius(kit.RadiusMd).Background(p.BgInset).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, kit.IconHash).Size(16, 16).TextColor(p.TextMuted)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				w, fg := kit.WeightMedium, p.Text
				if unread {
					w = kit.WeightSemibold
				}
				ui.Text(c, r.Name).FontSize(kit.SizeBody).FontWeight(w).TextColor(fg).SingleLine().Grow(1).MinWidth(0)
				ui.Text(c, timeLabel(r.LastActivity, a.now)).FontSize(kit.SizeCaption).FontFeatures("tnum").TextColor(p.TextMuted).Shrink(0)
			})
			ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				prev := r.Preview
				if prev == "" {
					prev = "No messages yet"
				}
				ui.Text(c, prev).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine().Grow(1).MinWidth(0)
				switch {
				case r.Call:
					ui.Icon(c, kit.IconPhone).Size(13, 13).TextColor(p.Success).Label("Call in progress")
				case r.Muted:
					ui.Icon(c, kit.IconBellOff).Size(13, 13).TextColor(p.TextFaint).Label("Muted")
				case r.Mentions > 0:
					kit.Count(c, r.Mentions, true)
				case r.Unread > 0:
					kit.Count(c, r.Unread, false)
				}
			})
		})
	})
	id := r.ID
	kit.ContextMenu(c, b, func(m *ui.Menu) { a.roomAction(id, kit.RoomMenu(m, a.menuState(r), a.now)) })
	if b.Clicked() {
		a.open(id)
	}
}

// phoneNoCommunities is the tab when the user is in none: what a community
// is, and how to get one.
func (a *App) phoneNoCommunities(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).Padding(kit.S6, kit.S4).Gap(kit.S3).Justify(ui.Center).AlignItems(ui.Stretch).Children(func() {
		ui.Text(c, "No communities yet").FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold).TextAlign(ui.Center)
		ui.Text(c, "Communities group rooms around a topic or a team. Join one with an invite link, or start your own.").
			FontSize(kit.SizeSmall).TextColor(p.TextMuted).TextAlign(ui.Center)
		if kit.Button(c, kit.Primary, "Join a community", nil).Clicked() {
			a.openRooms(0, 0)
		}
		if kit.Button(c, kit.Secondary, "Create a community", nil).Clicked() {
			a.openRooms(1, 2)
		}
	})
}

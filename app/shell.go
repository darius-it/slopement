package app

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// sideBg is the sidebars' background: the window's material while the
// theme follows the system (it tracks the system's appearance), else the
// kit's BgSubtle.
func (a *App) sideBg(c *ui.Context) ui.Color {
	p := kit.Of(c)
	if !a.glass(c) {
		return p.BgSubtle
	}
	if a.prefs.glass == glassClear {
		return ui.Transparent
	}
	// Tinted: the sidebar's own colour over the material, so the wallpaper
	// shows faintly but never washes the sidebar out (too light, or milky
	// over a bright desktop): steady contrast whatever is behind.
	if c.Theme().Dark {
		return p.BgSubtle.Alpha(0.78)
	}
	return p.BgSubtle.Alpha(0.66)
}

// Window glass settings.
const (
	glassOff = iota
	glassTinted
	glassClear
)

// glass reports whether the window's material shows: chosen, available
// (macOS, Windows 11), and only while the theme follows the system, as
// the material does.
func (a *App) glass(c *ui.Context) bool {
	return a.prefs.glass != glassOff && a.theme == 0 && c.Vibrancy()
}

// desktop is the wide layout: the navigation of the layout chosen, then
// the main area, then the side panel.
func (a *App) desktop(c *ui.Context) {
	ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		switch a.nav.Resolve(false) {
		case kit.NavTabs:
			if a.sidebar {
				a.tabsColumn(c)
			}
		case kit.NavInbox:
			if a.sidebar {
				a.inboxColumn(c, 340)
			}
		default:
			a.rail(c)
			// Settings takes the room beside the rail: no Chats column.
			if a.sidebar && a.view != viewSettings {
				a.railColumn(c)
			}
		}
		a.main(c, false)
	})
}

// main is the main area: the open room (with its side panel), People or
// Settings. back adds a back button (phones).
func (a *App) main(c *ui.Context, back bool) {
	p := kit.Of(c)
	ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Background(p.Bg).Children(func() {
		switch a.view {
		case viewPeople:
			a.peopleView(c, back)
		case viewSettings:
			a.settingsView(c, back)
		default:
			if _, ok := a.src.Room(a.room); !ok {
				a.emptyMain(c)
				return
			}
			a.roomView(c, back)
			if !back && a.panel != panelNone {
				a.sidePanel(c)
			}
		}
	})
}

// emptyMain is shown with no room open.
func (a *App) emptyMain(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(kit.S2).DragWindow().Children(func() {
		ui.Icon(c, kit.IconMessage).Size(32, 32).TextColor(p.TextFaint)
		ui.Text(c, "Pick a chat to start").FontSize(kit.SizeBody).TextColor(p.TextMuted)
		ui.Row(c).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "or press").FontSize(kit.SizeSmall).TextColor(p.TextFaint)
			kit.Kbd(c, kit.CmdK())
		})
	})
}

// topBarHeight is the thin bar across the window's top that holds the
// native window controls (macOS's traffic lights, Windows' and Linux's
// buttons); everything else starts under it.
const topBarHeight = 32

// topBar is that bar: the sidebar's material, a hairline under it, the
// place you're in, small and muted, in the middle. It drags the window and
// double-clicking it zooms, as a title bar does.
func (a *App) topBar(c *ui.Context) {
	p := kit.Of(c)
	bar := c.TitleBar()
	ui.Row(c).Height(max(topBarHeight, bar.Height)).Shrink(0).Padding(0, bar.Right+kit.S3, 0, bar.Left+kit.S3).
		AlignItems(ui.Center).Justify(ui.Center).Background(a.sideBg(c)).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).
		DragWindow().Children(func() {
		ui.Text(c, a.placeName()).FontSize(kit.SizeCaption).FontWeight(kit.WeightMedium).TextColor(p.TextMuted).SingleLine().Shrink(1)
	})
}

// placeName names where the user is, for the top bar.
func (a *App) placeName() string {
	switch a.view {
	case viewPeople:
		return "People"
	case viewSettings:
		return "Settings"
	}
	if r, ok := a.src.Room(a.room); ok {
		if cm, ok := a.communityOf(r.Community); ok {
			return cm.Name + " · #" + r.Name
		}
		return r.Name
	}
	return "slopement"
}

// listHeader is a list column's title row; it drags the window.
func listHeader(c *ui.Context, title string, action *ui.SVG, label string) (pressed bool) {
	ui.Row(c).Height(52).PaddingStart(kit.S4).PaddingEnd(kit.S2).AlignItems(ui.Center).Shrink(0).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold).Grow(1).SingleLine()
		if action != nil {
			pressed = kit.IconButton(c, kit.Ghost, action, label).Clicked()
		}
	})
	return pressed
}

// ---- Rail -----------------------------------------------------------------

// rail is the Rail layout's first column: Chats, then the communities, the
// +, and the user at the foot.
func (a *App) rail(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Width(kit.RailWidth).Shrink(0).Background(a.sideBg(c)).DragWindow().Children(func() {
		a.railItems(c, p)
	})
}

// railItems are the rail's contents, under its top strip.
func (a *App) railItems(c *ui.Context, p *kit.Palette) {
	ui.Column(c).Grow(1).Padding(kit.S2, 0, kit.S3, 0).Gap(kit.S2).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
		mentions := 0
		for _, r := range a.src.Rooms() {
			if r.Community == "" {
				mentions += r.Mentions
			}
		}
		if kit.RailItem(c, kit.RailItemData{ID: "chats", Name: "Chats", Icon: kit.IconMessage, Mention: mentions},
			a.community == "" && a.view != viewSettings).Clicked() {
			a.community = ""
			a.view = viewRoom
		}
		kit.RailDivider(c)
		comms := a.communities()
		for i, cm := range comms {
			if i > 0 && comms[i-1].Favourite && !cm.Favourite {
				kit.RailDivider(c) // favourites above the rest
			}
			unread, ment := a.communityCounts(cm)
			item := kit.RailItem(c, kit.RailItemData{ID: cm.ID, Name: cm.Name, Unread: unread, Mention: ment, Avatar: kit.AvatarSpec{Image: a.avatarPic(cm.Avatar)}},
				a.community == cm.ID && a.view != viewSettings)
			kit.ContextMenu(c, item, func(m *ui.Menu) { a.communityMenu(cm, m) })
			if item.Clicked() {
				a.community = cm.ID
				a.view = viewRoom
			}
		}
		var add kit.RailAddAction
		kit.RailAdd(c, &add)
		if add != kit.RailAddNone {
			a.toast = []string{"", "Creating communities", "Joining communities", "Exploring communities"}[add] + " comes with the Matrix layer"
		}
		ui.Spacer(c)
		ui.Column(c).AlignItems(ui.Center).Gap(kit.S2).Children(func() {
			// Settings toggles: a second click goes back.
			set := kit.IconButtonSide(c, kit.Ghost, kit.IconSettings, "Settings").AccessSelected(a.view == viewSettings)
			if a.view == viewSettings {
				set.Background(p.AccentSubtle)
			}
			if set.Clicked() {
				if a.view == viewSettings {
					a.view = viewRoom
				} else {
					a.view = viewSettings
				}
			}
			a.meButton(c, 32)
		})
	})
}

// meButton is the user's avatar: it sets their status; their status emoji
// sits on its corner.
func (a *App) meButton(c *ui.Context, size float32) {
	me := a.src.Me()
	tip := "Set a status"
	if me.Status.Set(a.now) {
		tip = me.Status.Emoji + " " + me.Status.Text
	}
	b := ui.ButtonBase(c.Key("me")).Radius(kit.RadiusFull).Label("Set a status")
	kit.SideTooltip(c, b, tip) // the rail's bottom: beside it, to the right
	b.Children(func() {
		kit.Avatar(c, a.avatar(me, size))
		if me.Status.Set(a.now) {
			ui.Box(c).Attach(ui.AnchorTopRight, ui.AnchorCenter).Top(4).Left(-4).PassThrough().Children(func() {
				ui.Text(c, me.Status.Emoji).FontSize(12)
			})
		}
	})
	if b.Clicked() {
		a.statusDraft.Edit(status(me.Status))
	}
}

// communityCounts sums a community's rooms.
func (a *App) communityCounts(cm model.Community) (unread bool, mentions int) {
	for _, id := range a.communityRoomIDs(cm, 0) {
		if r, ok := a.src.Room(id); ok && !r.Muted {
			unread = unread || r.Unread > 0
			mentions += r.Mentions
		}
	}
	return
}

// railColumn is the Rail layout's list: Chats, or the community picked.
func (a *App) railColumn(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Width(288).Shrink(0).Background(a.sideBg(c)).
		BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
		if cm, ok := a.communityOf(a.community); ok {
			ui.Scroll(c.Key("cm:" + cm.ID)).Grow(1).PaddingX(kit.S2).Children(func() {
				a.communityHome(c, cm)
				ui.Box(c).Height(1).Background(p.Border).MarginY(kit.S1)
				a.communityRooms(c, cm)
			})
			return
		}
		if listHeader(c, "Chats", kit.IconPencil, "New chat") {
			a.view = viewPeople
		}
		ui.Column(c).PaddingX(kit.S2).PaddingY(kit.S1).Children(func() {
			if kit.SearchButton(c.Key("search"), "Search", kit.CmdK()).Clicked() {
				a.switcher.Toggle()
			}
		})
		ui.Scroll(c.Key("chats")).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() {
			a.chatsList(c, true)
		})
	})
}

func (a *App) communityOf(id string) (model.Community, bool) {
	for _, cm := range a.src.Communities() {
		if cm.ID == id {
			return cm, true
		}
	}
	return model.Community{}, false
}

// ---- Tabs -----------------------------------------------------------------

// tabsColumn is the Tabs layout's list: a Chats / Communities switch.
func (a *App) tabsColumn(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Width(300).Shrink(0).Background(a.sideBg(c)).
		BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
		ui.Column(c).Padding(kit.S3).Gap(kit.S3).DragWindow().Children(func() {
			kit.Tabs(c.Key("tabs"), &a.tab, "Chats", "Communities")
			if kit.SearchButton(c.Key("search"), "Search", kit.CmdK()).Clicked() {
				a.switcher.Toggle()
			}
		})
		ui.Scroll(c.Key("tab" + strconv.Itoa(a.tab))).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() {
			if a.tab == 0 {
				a.chatsList(c, true)
				return
			}
			a.communitiesTab(c)
		})
		ui.Row(c).Padding(kit.S2, kit.S3).Gap(kit.S2).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Children(func() {
			a.meButton(c, 28)
			ui.Text(c, a.src.Me().Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).Grow(1).SingleLine()
			if kit.IconButton(c, kit.Ghost, kit.IconSettings, "Settings").Clicked() {
				a.view = viewSettings
			}
		})
	})
}

// ---- One inbox ------------------------------------------------------------

// inboxColumn is the One inbox layout's list, width wide (0: grow).
func (a *App) inboxColumn(c *ui.Context, width float32) {
	p := kit.Of(c)
	col := ui.Column(c).Background(a.sideBg(c)).BorderWidth(0, 1, 0, 0).BorderColor(p.Border)
	if width > 0 {
		col.Width(width).Shrink(0)
	} else {
		col.Grow(1)
	}
	col.Children(func() {
		if listHeader(c, "Inbox", kit.IconPencil, "New chat") {
			a.view = viewPeople
		}
		ui.Column(c).PaddingX(kit.S3).PaddingY(kit.S1).Gap(kit.S2).Children(func() {
			if kit.SearchButton(c.Key("search"), "Search", kit.CmdK()).Clicked() {
				a.switcher.Toggle()
			}
			ui.Scroll(c).Row().Gap(kit.S1).Scrollbars(false).Children(func() {
				for i, l := range []string{"All", "People", "Communities", "Unread"} {
					if kit.Chip(c.Key("f"+l), l, a.filter == i).Clicked() {
						a.filter = i
					}
				}
			})
		})
		ui.Scroll(c.Key("inbox")).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() { a.inboxList(c) })
		if width > 0 {
			ui.Row(c).Padding(kit.S2, kit.S3).Gap(kit.S2).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Children(func() {
				a.meButton(c, 28)
				ui.Text(c, a.src.Me().Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).Grow(1).SingleLine()
				if kit.IconButton(c, kit.Ghost, kit.IconUsers, "People").Clicked() {
					a.view = viewPeople
				}
				if kit.IconButton(c, kit.Ghost, kit.IconSettings, "Settings").Clicked() {
					a.view = viewSettings
				}
			})
		}
	})
}

// ---- Phone ----------------------------------------------------------------

// phone is the compact layout: the lists under a tab bar, and the room
// pushed over them.
func (a *App) phone(c *ui.Context) {
	p := kit.Of(c)
	if a.pushed && a.view == viewRoom {
		if _, ok := a.src.Room(a.room); ok {
			a.phoneRoom(c)
			return
		}
	}
	a.pushed = false
	inbox := a.nav.Resolve(true) == kit.NavInbox
	ui.Column(c).Grow(1).MinHeight(0).Background(p.Bg).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			switch {
			case a.phoneTab == 3:
				a.settingsView(c, false)
			case a.phoneTab == 2:
				a.peopleView(c, false)
			case inbox:
				a.inboxColumn(c, 0)
			case a.phoneTab == 1:
				if listHeader(c, "Communities", kit.IconPlus, "Join or create") {
					a.toast = "Joining communities comes with the Matrix layer"
				}
				ui.Scroll(c.Key("comms")).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() { a.communitiesTab(c) })
			default:
				if listHeader(c, "Chats", kit.IconPencil, "New chat") {
					a.phoneTab = 2
				}
				ui.Column(c).PaddingX(kit.S3).PaddingY(kit.S1).Children(func() {
					if kit.SearchButton(c.Key("search"), "Search", "").Clicked() {
						a.switcher.Toggle()
					}
				})
				ui.Scroll(c.Key("chats")).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() { a.chatsList(c, false) })
			}
		})
		names := []string{"Chats", "Communities", "People", "Settings"}
		icons := []*ui.SVG{kit.IconMessage, kit.IconUsers, kit.IconUser, kit.IconSettings}
		if inbox {
			names[0], names[1], icons[1] = "Inbox", "Calls", kit.IconPhone
		}
		if i := a.tabBar(c, names, icons); i >= 0 {
			if inbox && i == 1 {
				a.toast = "Calls come with the Matrix layer"
			} else {
				a.phoneTab = i
				a.view = []view{viewRoom, viewRoom, viewPeople, viewSettings}[i]
			}
		}
	})
}

// tabBar is the phone's tab bar; it returns the tab pressed, or -1. (In a
// packaged phone build this becomes mygo.NewTabs, the system's.)
func (a *App) tabBar(c *ui.Context, names []string, icons []*ui.SVG) (pressed int) {
	p := kit.Of(c)
	pressed = -1
	ui.Row(c).Height(64).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Background(p.BgSubtle).Padding(kit.S2, 0, 0, 0).Children(func() {
		for i, n := range names {
			fg := p.TextMuted
			if i == a.phoneTab {
				fg = p.AccentStrong
			}
			tab := ui.ButtonBase(c.Key("tab:" + n)).Grow(1).Gap(2).Column().AlignItems(ui.Center).Label(n).AccessSelected(i == a.phoneTab)
			tab.Children(func() {
				ui.Icon(c, icons[i]).Size(22, 22).TextColor(fg)
				ui.Text(c, n).FontSize(10).FontWeight(kit.WeightMedium).TextColor(fg)
			})
			if tab.Clicked() {
				pressed = i
			}
		}
	})
	return pressed
}

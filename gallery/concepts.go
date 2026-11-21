package gallery

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// Concepts explore how to organise a Matrix account so that people are not
// "inside" spaces. Each has a desktop and a phone screen built from kit
// parts. The model they share:
//
//   - Chats: everyone you talk to directly, DMs and small group chats,
//     whatever space a room technically sits in (m.direct rooms, and rooms
//     without a parent space up to ~10 members).
//   - Communities: spaces, the big shared places, each a home page with its
//     rooms. Rooms in no space that are large go to "Other rooms".

// chat is an entry of the Chats list.
type chat struct {
	id, name, preview, time string
	mxid                    string // a DM's partner
	unread, mention         int
	presence                kit.Presence
	group                   bool // a small group chat, not a DM
	status                  kit.Status
}

func (a *App) conceptChats() []chat {
	list := []chat{
		{id: "c-alex", mxid: "@alex:example.com", name: "Alex Rivera", preview: "sounds good, tomorrow 10?", time: "13:48", mention: 1},
		{id: "c-fam", name: "Family 🏡", preview: "Mum: dinner on Sunday?", time: "13:20", unread: 3, group: true},
		{id: "c-mira", mxid: "@mira:matrix.org", name: "Mira", preview: "You: the rail spacing is in", time: "12:02"},
		{id: "c-trip", name: "Lisbon trip", preview: "Kai: flights booked ✈️", time: "Yesterday", group: true},
		{id: "c-noor", mxid: "@noor:example.com", name: "Noor الحسيني", preview: "مرحبا! كيف حالك؟", time: "Yesterday"},
		{id: "c-sam", mxid: "@sam:matrix.org", name: "Sam", preview: "who's up for lunch", time: "Mon"},
	}
	// A DM shows its partner's presence and status.
	for i := range list {
		if ps, ok := a.person(list[i].mxid); ok {
			list[i].presence, list[i].status = ps.Presence, ps.Status
		}
	}
	return list
}

// chatsList is the Chats list of every layout: People (optional), the
// starred chats under Favourites, then the rest by recency. selected
// highlights the open chat.
func (a *App) chatsList(c *ui.Context, people, selected bool) {
	cs := a.concepts()
	if people {
		a.peopleEntry(c)
	}
	var favs, rest []chat
	for _, ch := range a.conceptChats() {
		if a.favChats[ch.id] {
			favs = append(favs, ch)
		} else {
			rest = append(rest, ch)
		}
	}
	section := func(title string, list []chat) {
		if len(list) == 0 {
			return
		}
		ui.Row(c.Key("sec"+title)).Gap(kit.S1).AlignItems(ui.Center).Padding(kit.S2, kit.S2, kit.S1, kit.S2).Children(func() {
			if title == "Favourites" {
				ui.Icon(c, kit.IconStar).Size(12, 12).TextColor(kit.Of(c).TextMuted)
			}
			ui.Text(c, title).FontSize(kit.SizeCaption+1).FontWeight(kit.WeightMedium).TextColor(kit.Of(c).TextMuted)
		})
		for _, ch := range list {
			if a.chatRow(c, ch, selected && cs.chatOpen == ch.id && a.room != "people").Clicked() {
				cs.chatOpen, a.room = ch.id, ""
			}
		}
	}
	section("Favourites", favs)
	section("Chats", rest)
}

// community is a space.
type community struct {
	id, name, about string
	members, online int
	unread, mention int
	rooms           []string
}

var communities = []community{
	{id: "!design", name: "Design Collective", about: "People who care about the details", members: 412, online: 38, unread: 6,
		rooms: []string{"announcements", "design-crit", "general", "weekly-sync", "showcase"}},
	{id: "!matrix", name: "Matrix HQ", about: "The Matrix protocol and its ecosystem", members: 31200, online: 1870, mention: 3,
		rooms: []string{"matrix", "matrix-dev", "offtopic"}},
	{id: "!go", name: "Gophers", about: "Go, the language", members: 5600, online: 240, unread: 1,
		rooms: []string{"go-nuts", "golang-dev", "jobs"}},
}

// conceptState is what the concept screens remember.
type conceptState struct {
	tabA, tabC    int    // A: 0 chats, 1 communities; C: the inbox filter
	openCommunity string // A: the community open, "" for the list
	railB         string // B: "chats" or a community's ID
	expandedC     map[string]bool
	chatOpen      string
}

func (a *App) concepts() *conceptState {
	if a.concept.expandedC == nil {
		a.concept = conceptState{railB: "chats", expandedC: map[string]bool{"!design": true}, chatOpen: "c-alex"}
	}
	return &a.concept
}

// chatRow is a chat in the Chats list: a DM with the person's presence and
// status emoji, or a group chat.
func (a *App) chatRow(c *ui.Context, ch chat, selected bool) ui.Element {
	r := kit.RoomRowData{ID: ch.id, Name: ch.name, Preview: ch.preview, Time: ch.time, Unread: ch.unread, Mention: ch.mention,
		Avatar: kit.AvatarSpec{Presence: ch.presence, Colors: &a.colors}, Status: ch.status}
	if ch.group {
		r.Avatar.Square = true
	}
	// Its menu: star it, mute it, …
	id := ch.id
	st := kit.RoomMenuState{Favourite: a.favChats[id], Unread: ch.unread > 0 || ch.mention > 0}
	return kit.RoomRow(c, r, selected, func(m *ui.Menu) {
		if act := kit.RoomMenu(m, st, a.now); act.ToggleFav {
			a.favChats[id] = !a.favChats[id]
		}
	})
}

// roomLine is a room of a community: # and its name, quiet unless unread.
func roomLine(c *ui.Context, name string, unread bool, selected bool) ui.Element {
	p := kit.Of(c)
	b := ui.ButtonBase(c.Key("room:"+name)).Height(32).PaddingX(kit.S2).Gap(kit.S2).Radius(kit.RadiusSm).
		AlignItems(ui.Center).Justify(ui.Start).Label(name)
	bg, fg, w := ui.Transparent, p.TextMuted, kit.WeightRegular
	switch {
	case selected:
		bg, fg, w = p.AccentSubtle, p.AccentStrong, kit.WeightMedium
	case b.Hovered():
		bg, fg = p.Hover, p.Text
	}
	if unread && !selected {
		fg, w = p.Text, kit.WeightSemibold
	}
	b.Background(b.AnimateColor("bg", bg, kit.Color))
	b.Children(func() {
		ui.Icon(c, kit.IconHash).Size(15, 15).TextColor(fg)
		ui.Text(c, name).FontSize(kit.SizeBody).FontWeight(w).TextColor(fg).SingleLine()
	})
	return b
}

// communityHeader is a community's home: its picture, name, what it is
// about, how many are in it, and invite.
func (a *App) communityHeader(c *ui.Context, cm community) {
	p := kit.Of(c)
	ui.Column(c).Gap(kit.S2).Padding(kit.S3).Children(func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: cm.name, ID: cm.id, Size: 48, Square: true})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, cm.name).FontSize(kit.SizeLabel+1).FontWeight(kit.WeightSemibold).SingleLine()
				ui.RichText(c,
					ui.Span{Text: "● ", Color: p.Success},
					ui.Span{Text: compact(cm.online) + " online · " + compact(cm.members) + " members", Color: p.TextMuted},
				).FontSize(kit.SizeSmall).SingleLine()
			})
		})
		ui.Text(c, cm.about).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		ui.Row(c).Gap(kit.S2).Children(func() {
			kit.Button(c, kit.Secondary, "Invite", kit.IconUser).Grow(1)
			kit.IconButton(c, kit.Secondary, kit.IconSettings, "Community settings")
		})
	})
}

func compact(n int) string {
	if n >= 1000 {
		return strconv.FormatFloat(float64(n)/1000, 'f', 1, 64) + "k"
	}
	return strconv.Itoa(n)
}

// frame is a desktop mock's window.
func frame(c *ui.Context, key string, h float32, fn func()) {
	p := kit.Of(c)
	ui.Row(c.Key(key)).Height(h).Radius(kit.RadiusLg).Border(1, p.Border).Clip().AlignItems(ui.Stretch).Background(p.Bg).
		Shadow(0, 20, 48, -24, p.Shadow).Children(fn)
}

// emptyConversation stands in for the conversation in the concept mocks.
func emptyConversation(c *ui.Context, title, sub string) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		ui.Row(c).Height(52).PaddingX(kit.S4).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).
			Children(func() {
				ui.Column(c).Children(func() {
					ui.Text(c, title).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
					if sub != "" {
						ui.Text(c, sub).FontSize(kit.SizeCaption).TextColor(p.TextMuted)
					}
				})
			})
		ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "Conversation").FontSize(kit.SizeSmall).TextColor(p.TextFaint)
		})
	})
}

// listHeader is a sidebar's title row.
func listHeader(c *ui.Context, title string, action *ui.SVG, label string) {
	ui.Row(c).Height(52).PaddingStart(kit.S4).PaddingEnd(kit.S2).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold).Grow(1)
		if action != nil {
			kit.IconButton(c, kit.Ghost, action, label)
		}
	})
}

// ---- Concept A: Chats & Communities ---------------------------------------

func (a *App) conceptA(c *ui.Context) {
	p := kit.Of(c)
	ui.Text(c, "Two places, never mixed: Chats is everyone you talk to directly (DMs and small group chats, wherever they live); Communities are spaces, each opening to its own home and rooms. After WhatsApp's Chats/Communities and Ladder's Communities | Direct Messages (Mobbin).").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted).MaxWidth(900)
	frame(c, "A-desk", 620, func() { a.deskA(c, nil) })
	ui.Row(c).Gap(kit.S6).Wrap().Children(func() {
		phone(c, "A-phone-chats", func() { a.phoneA(c, 0) })
		phone(c, "A-phone-comm", func() { a.phoneA(c, 1) })
	})
}

// deskA is layout A on a desktop: a Chats | Communities switch over the
// list. pane builds the main area (nil: a placeholder).
func (a *App) deskA(c *ui.Context, pane func()) {
	cs := a.concepts()
	p := kit.Of(c)
	ui.Column(c).Width(300).Shrink(0).Background(p.BgSubtle).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
		ui.Column(c).Padding(kit.S3).Gap(kit.S3).Children(func() {
			kit.Tabs(c.Key("A-tabs"), &cs.tabA, "Chats", "Communities")
			kit.SearchButton(c.Key("A-search"), "Search", kit.CmdK())
		})
		ui.Scroll(c).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() {
			if cs.tabA == 0 {
				a.chatsList(c, true, true)
				return
			}
			a.communityList(c, cs)
		})
	})
	switch {
	case pane != nil:
		pane()
	case cs.tabA == 1 && cs.openCommunity != "":
		emptyConversation(c, "# design-crit", "Design Collective")
	case cs.tabA == 1:
		emptyConversation(c, "Communities", "Pick one to see its rooms")
	default:
		emptyConversation(c, chatName(a.conceptChats(), cs.chatOpen), "")
	}
}

// peopleEntry opens the People view (statuses at a glance) from the top of
// the Chats list.
func (a *App) peopleEntry(c *ui.Context) {
	p := kit.Of(c)
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
}

func chatName(list []chat, id string) string {
	for _, ch := range list {
		if ch.id == id {
			return ch.name
		}
	}
	return ""
}

// communityList is concept A's Communities tab: cards, or one opened.
func (a *App) communityList(c *ui.Context, cs *conceptState) {
	p := kit.Of(c)
	if cs.openCommunity != "" {
		for _, cm := range communities {
			if cm.id != cs.openCommunity {
				continue
			}
			back := ui.ButtonBase(c.Key("back")).Height(28).PaddingX(kit.S1).Gap(kit.S1).Radius(kit.RadiusSm).AlignItems(ui.Center).
				AlignSelf(ui.Start).Label("All communities")
			back.Children(func() {
				ui.Icon(c, kit.IconChevronL).Size(14, 14).TextColor(p.TextMuted)
				ui.Text(c, "All communities").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			})
			if back.Clicked() {
				cs.openCommunity = ""
			}
			a.communityHeader(c, cm)
			ui.Text(c, "Rooms").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).Padding(kit.S2, kit.S2, kit.S1, kit.S2)
			for i, r := range cm.rooms {
				roomLine(c, r, i == 1, i == 1)
			}
		}
		return
	}
	for _, cm := range communities {
		b := ui.ButtonBase(c.Key("cm"+cm.id)).Padding(kit.S2).Gap(kit.S3).Radius(kit.RadiusMd).AlignItems(ui.Center).Justify(ui.Start).Label(cm.name)
		if b.Hovered() {
			b.Background(p.Hover)
		}
		b.Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: cm.name, ID: cm.id, Size: 44, Square: true})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, cm.name).FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold).SingleLine()
				ui.Text(c, strconv.Itoa(len(cm.rooms))+" rooms · "+compact(cm.online)+" online").FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
			})
			switch {
			case cm.mention > 0:
				kit.Count(c, cm.mention, true)
			case cm.unread > 0:
				kit.Dot(c, false)
			}
		})
		if b.Clicked() {
			cs.openCommunity = cm.id
		}
	}
	ui.Box(c).Height(kit.S2)
	kit.Button(c, kit.Secondary, "Join or create a community", kit.IconPlus).Margin(0, kit.S2, 0, kit.S2)
}

// phoneA is concept A on a phone: the tab bar has Chats, Communities and
// People (statuses at a glance).
func (a *App) phoneA(c *ui.Context, tab int) {
	p := kit.Of(c)
	ui.Column(c).Grow(1).Children(func() {
		switch tab {
		case 0:
			listHeader(c, "Chats", kit.IconPencil, "New chat")
			ui.Column(c).PaddingX(kit.S3).PaddingY(kit.S1).Children(func() { kit.SearchButton(c.Key("pa-s"), "Search", "") })
			ui.Scroll(c).Grow(1).PaddingX(kit.S2).Children(func() {
				a.chatsList(c, false, false) // People has its own tab here
			})
		case 2:
			listHeader(c, "People", kit.IconPlus, "Add person")
			ui.Scroll(c).Grow(1).Padding(kit.S2, kit.S3).Children(func() {
				a.peopleAct(kit.People(c, &a.people, a.contacts(), &a.colors, a.now))
			})
		default:
			listHeader(c, "Communities", kit.IconPlus, "Join or create")
			ui.Scroll(c).Grow(1).PaddingX(kit.S3).Gap(kit.S3).Children(func() {
				for _, cm := range communities {
					// A community as a card: header and its first rooms, as
					// WhatsApp's Communities tab.
					ui.Column(c.Key("pc"+cm.id)).Radius(kit.RadiusLg).Background(p.Raised).Border(1, p.Border).Padding(kit.S1).Children(func() {
						ui.Row(c).Gap(kit.S3).Padding(kit.S2).AlignItems(ui.Center).Children(func() {
							kit.Avatar(c, kit.AvatarSpec{Name: cm.name, ID: cm.id, Size: 40, Square: true})
							ui.Column(c).Grow(1).MinWidth(0).Children(func() {
								ui.Text(c, cm.name).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold).SingleLine()
								ui.Text(c, compact(cm.online)+" online").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
							})
							if cm.mention > 0 {
								kit.Count(c, cm.mention, true)
							}
						})
						for i, r := range cm.rooms[:min(2, len(cm.rooms))] {
							roomLine(c, r, cm.unread > 0 && i == 0, false)
						}
						ui.Text(c, "See all "+strconv.Itoa(len(cm.rooms))+" rooms").FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).
							TextColor(p.AccentStrong).Padding(kit.S1, kit.S2, kit.S2, kit.S2)
					})
				}
			})
		}
		if i := conceptTabs(c, tab, []string{"Chats", "Communities", "People", "Settings"},
			[]*ui.SVG{kit.IconMessage, kit.IconUsers, kit.IconUser, kit.IconSettings}); i >= 0 && i <= 2 {
			a.phoneTab = i
		}
	})
}

// conceptTabs is a phone tab bar for the mocks (mygo.NewTabs in the app);
// it returns the tab pressed, or -1.
func conceptTabs(c *ui.Context, sel int, names []string, icons []*ui.SVG) (pressed int) {
	p := kit.Of(c)
	pressed = -1
	ui.Row(c).Height(72).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Background(p.BgSubtle).Padding(kit.S2, 0, 0, 0).Children(func() {
		for i, n := range names {
			fg := p.TextMuted
			if i == sel {
				fg = p.AccentStrong
			}
			tab := ui.ButtonBase(c.Key(n)).Grow(1).Gap(2).Column().AlignItems(ui.Center).Label(n).AccessSelected(i == sel)
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

// ---- Concept B: Chats on the rail -----------------------------------------

func (a *App) conceptB(c *ui.Context) {
	p := kit.Of(c)
	ui.Text(c, "Desktop-first: the rail's first item is Chats (people and group chats, all in one list); below the line, communities, each showing only its own rooms under a header that says what it is. People never appear inside a community. After Discord's DMs vs servers, without the server-channel jargon.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted).MaxWidth(900)
	frame(c, "B-desk", 620, func() { a.deskB(c, nil) })
}

// deskB is layout B: Chats first on the rail, then the communities.
func (a *App) deskB(c *ui.Context, pane func()) {
	cs := a.concepts()
	p := kit.Of(c)
	{
		ui.Column(c).Width(kit.RailWidth).Shrink(0).Background(p.BgSubtle).PaddingY(kit.S3).Gap(kit.S2).
			BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
			unread := 0
			for _, ch := range a.conceptChats() {
				unread += ch.mention
			}
			if kit.RailItem(c, kit.RailItemData{ID: "chats", Name: "Chats", Icon: kit.IconMessage, Mention: unread}, cs.railB == "chats").Clicked() {
				cs.railB = "chats"
			}
			kit.RailDivider(c)
			for _, cm := range communities {
				if kit.RailItem(c, kit.RailItemData{ID: cm.id, Name: cm.name, Unread: cm.unread > 0, Mention: cm.mention}, cs.railB == cm.id).Clicked() {
					cs.railB = cm.id
				}
			}
			var add kit.RailAddAction
			kit.RailAdd(c, &add)
		})
		ui.Column(c).Width(280).Shrink(0).Background(p.BgSubtle).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
			if cs.railB == "chats" {
				listHeader(c, "Chats", kit.IconPencil, "New chat")
				ui.Column(c).PaddingX(kit.S2).PaddingY(kit.S1).Children(func() { kit.SearchButton(c.Key("B-s"), "Search", kit.CmdK()) })
				ui.Scroll(c).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() {
					a.chatsList(c, true, true)
				})
				return
			}
			for _, cm := range communities {
				if cm.id != cs.railB {
					continue
				}
				ui.Scroll(c).Grow(1).PaddingX(kit.S2).Children(func() {
					a.communityHeader(c, cm)
					ui.Box(c).Height(1).Background(p.Border).MarginY(kit.S1)
					for i, r := range cm.rooms {
						roomLine(c, r, i < min(cm.unread, 2), i == 1)
					}
				})
			}
		})
		switch {
		case pane != nil:
			pane()
		case cs.railB == "chats":
			emptyConversation(c, chatName(a.conceptChats(), cs.chatOpen), "")
		default:
			emptyConversation(c, "# design-crit", "")
		}
	}
}

// ---- Concept C: One inbox --------------------------------------------------

func (a *App) conceptC(c *ui.Context) {
	p := kit.Of(c)
	ui.Text(c, "No rail, one list, filters on top (All · People · Communities · Unread), as Telegram's folders and Front's inbox. Chats with people are flat rows; a community is one row that unfolds to its rooms in place. The same layout works unchanged on a phone.").
		FontSize(kit.SizeSmall).TextColor(p.TextMuted).MaxWidth(900)
	frame(c, "C-desk", 620, func() { a.deskC(c, nil) })
	ui.Row(c).Gap(kit.S6).Wrap().Children(func() {
		phone(c, "C-phone", func() { a.phoneC(c) })
	})
}

// deskC is layout C on a desktop: the inbox beside the main area.
func (a *App) deskC(c *ui.Context, pane func()) {
	p := kit.Of(c)
	ui.Column(c).Width(340).Shrink(0).Background(p.BgSubtle).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
		listHeader(c, "Inbox", kit.IconPencil, "New")
		a.inboxList(c, "C-desk-list")
	})
	if pane != nil {
		pane()
	} else {
		emptyConversation(c, "Alex Rivera", "")
	}
}

// phoneC is layout C on a phone: the inbox, or People (a.phoneTab 2).
func (a *App) phoneC(c *ui.Context) {
	sel := 0
	if a.phoneTab == 2 {
		sel = 1
		listHeader(c, "People", kit.IconPlus, "Add person")
		ui.Scroll(c).Grow(1).Padding(kit.S2, kit.S3).Children(func() {
			a.peopleAct(kit.People(c, &a.people, a.contacts(), &a.colors, a.now))
		})
	} else {
		listHeader(c, "Inbox", kit.IconPencil, "New")
		a.inboxList(c, "C-phone-list")
	}
	switch conceptTabs(c, sel, []string{"Inbox", "People", "Calls", "Settings"},
		[]*ui.SVG{kit.IconMessage, kit.IconUsers, kit.IconPhone, kit.IconSettings}) {
	case 0:
		a.phoneTab = 0
	case 1:
		a.phoneTab = 2
	}
}

// inboxList is layout C's one list, with its filters.
func (a *App) inboxList(c *ui.Context, key string) {
	cs := a.concepts()
	{
		ui.Column(c.Key(key)).Grow(1).Children(func() {
			ui.Column(c).Padding(kit.S3).Gap(kit.S2).Children(func() {
				ui.Row(c).Gap(kit.S1).Children(func() {
					for i, l := range []string{"All", "People", "Communities", "Unread"} {
						if kit.Chip(c.Key("C"+l), l, cs.tabC == i).Clicked() {
							cs.tabC = i
						}
					}
				})
			})
			ui.Scroll(c).Grow(1).PaddingX(kit.S2).Gap(1).Children(func() {
				chats := a.conceptChats()
				shownChats := 0
				if cs.tabC != 2 {
					// Starred chats lead, under Favourites, in every filter.
					p := kit.Of(c)
					first := true
					for _, ch := range chats {
						if !a.favChats[ch.id] || (cs.tabC == 3 && ch.unread == 0 && ch.mention == 0) {
							continue
						}
						if first {
							ui.Row(c.Key("favs")).Gap(kit.S1).AlignItems(ui.Center).Padding(kit.S1, kit.S2).Children(func() {
								ui.Icon(c, kit.IconStar).Size(12, 12).TextColor(p.TextMuted)
								ui.Text(c, "Favourites").FontSize(kit.SizeCaption+1).FontWeight(kit.WeightMedium).TextColor(p.TextMuted)
							})
							first = false
						}
						a.chatRow(c, ch, false)
					}
					if !first {
						ui.Box(c.Key("favs-end")).Height(1).Background(p.Border).Margin(kit.S1, kit.S2)
					}
					for _, ch := range chats {
						if a.favChats[ch.id] || (cs.tabC == 3 && ch.unread == 0 && ch.mention == 0) {
							continue
						}
						a.chatRow(c, ch, false)
						shownChats++
						// Communities mixed in by recency in "All": one after
						// the first two chats, as an example.
						if cs.tabC == 0 && shownChats == 2 {
							a.inboxCommunity(c, cs, communities[0])
						}
					}
				}
				if cs.tabC == 1 {
					return
				}
				for i, cm := range communities {
					if cs.tabC == 0 && i == 0 {
						continue // shown above, by recency
					}
					if cs.tabC == 3 && cm.unread == 0 && cm.mention == 0 {
						continue
					}
					a.inboxCommunity(c, cs, cm)
				}
			})
		})
	}
}

// inboxCommunity is a community in concept C's inbox: one row, which
// unfolds to its rooms.
func (a *App) inboxCommunity(c *ui.Context, cs *conceptState, cm community) {
	p := kit.Of(c)
	open := cs.expandedC[cm.id]
	b := ui.ButtonBase(c.Key("ic"+cm.id)).Height(kit.RowHeight).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusMd - 2).
		AlignItems(ui.Center).Justify(ui.Start).Label(cm.name).Expanded(open)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		kit.Avatar(c, kit.AvatarSpec{Name: cm.name, ID: cm.id, Size: 36, Square: true})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, cm.name).FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold).SingleLine()
			ui.Text(c, strconv.Itoa(len(cm.rooms))+" rooms · "+compact(cm.online)+" online").FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
		})
		if cm.mention > 0 {
			kit.Count(c, cm.mention, true)
		} else if cm.unread > 0 {
			kit.Dot(c, false)
		}
		chev := ui.Icon(c, kit.IconChevronD).Size(14, 14).TextColor(p.TextMuted)
		r := float32(-90)
		if open {
			r = 0
		}
		chev.Rotate(chev.AnimateWith("r", r, kit.DurFast, kit.EaseOut))
	})
	if b.Clicked() {
		cs.expandedC[cm.id] = !open
	}
	if open {
		ui.Column(c.Key("icr"+cm.id)).PaddingStart(36+kit.S3).Gap(1).
			Transition(ui.ElementTransition{Duration: kit.DurBase, ExitDuration: kit.DurFast, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).
			Children(func() {
				for i, r := range cm.rooms {
					roomLine(c, r, i < min(cm.unread, 2), false)
				}
			})
	}
}

// conceptsCompare is the overview: the three side by side in words, and a
// recommendation.
func (a *App) conceptsCompare(c *ui.Context) {
	p := kit.Of(c)
	a.specimen(c, "The model all three share", "", true, func() {
		for _, l := range []string{
			"Chats = people. DMs and small group chats, in one list, whatever space Matrix keeps them in. You never go into a space to find a friend.",
			"Communities = spaces. Big shared places with a home (who, what, how many online) and rooms. Joining one doesn't clutter your chats.",
			"Other rooms: big rooms in no space appear as one 'Other rooms' community, so nothing is lost.",
			"Words: 'Community' and 'room' in the UI, never 'space' jargon or nested-space trees in the main navigation.",
		} {
			ui.Row(c.Key(l)).Gap(kit.S2).Children(func() {
				ui.Text(c, "·").FontWeight(kit.WeightSemibold).TextColor(p.Accent)
				ui.Text(c, l).FontSize(kit.SizeBody).Grow(1)
			})
		}
	})
	a.specimen(c, "Compared", "", true, func() {
		rows := [][4]string{
			{"", "A · Chats & Communities", "B · Chats on the rail", "C · One inbox"},
			{"Best for", "Phones first; people + a few communities", "Desktop power users in many communities", "Mostly people, few communities"},
			{"Finding a friend", "Chats tab, always", "Rail → Chats", "Top of the inbox, or the People filter"},
			{"Community unread", "Badge on the card", "Badge on the rail", "Badge on its row"},
			{"Feels like", "WhatsApp + Communities", "Discord without the jargon", "Telegram folders / an email inbox"},
			{"Cost", "One extra tap into a community", "A rail to learn", "Long lists with many communities"},
		}
		ui.Column(c).Radius(kit.RadiusMd).Border(1, p.Border).Clip().Children(func() {
			for i, r := range rows {
				ui.Row(c.Key(i)).Padding(kit.S2, kit.S3).Gap(kit.S3).Background(map[bool]ui.Color{true: p.BgInset, false: ui.Transparent}[i == 0]).
					BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
					for j, cell := range r {
						t := ui.Text(c, cell).FontSize(kit.SizeSmall).Grow(1).Basis(0)
						if i == 0 || j == 0 {
							t.FontWeight(kit.WeightSemibold)
						}
						if j == 0 {
							t.TextColor(p.TextMuted)
						}
					}
				})
			}
		})
	})
	a.specimen(c, "Recommendation", "", true, func() {
		ui.Text(c, "A on phones, B on desktop: the same two places (Chats and Communities), shown as tabs where space is short and as a rail where it isn't. Someone moving between devices finds their people in the same place. C is the lightest, worth it if most use is talking to people.").
			FontSize(kit.SizeBody).TextColor(p.Text).MaxWidth(800)
	})
}

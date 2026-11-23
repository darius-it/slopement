package app

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// chats are the rooms of Chats: DMs and group chats, and rooms in no
// community, most recent first.
func (a *App) chats() []model.Room {
	var out []model.Room
	for _, r := range a.roomList() {
		if r.Community == "" {
			out = append(out, r)
		}
	}
	return out
}

// listLine is one line of a long sidebar list. Only the lines in view are
// drawn (ui.List): building an element for every chat, every frame, made
// typing sluggish in accounts of a few hundred chats.
type listLine struct {
	key  string
	draw func()
}

// showLines draws lines as a list that scrolls and builds only what shows.
func (a *App) showLines(c *ui.Context, key string, ls *ui.ListState, lines []listLine) {
	ls.Key = func(i int) any {
		if i < len(lines) {
			return lines[i].key
		}
		return i
	}
	ui.List(c.Key(key), ls, len(lines), func(i int) {
		if i < len(lines) {
			lines[i].draw()
		}
	}).Grow(1).PaddingX(kit.S2).Gap(1)
}

// chatsListView is the Chats list of every layout, scrolling.
func (a *App) chatsListView(c *ui.Context, key string, people bool) {
	a.showLines(c, key, &a.chatsLS, a.chatLines(c, people))
}

// chatLines are the lines of Chats: People (on desktops), invitations, the
// starred chats under Favourites, the rest by recency, low priority last.
func (a *App) chatLines(c *ui.Context, people bool) []listLine {
	var lines []listLine
	if people {
		lines = append(lines, listLine{"people", func() { a.peopleEntry(c) }})
	}
	if len(a.pendingInvites()) > 0 {
		lines = append(lines, listLine{"invites", func() { a.invitesSection(c) }}) // waiting, above the chats
	}
	var favs, rest, low []model.Room
	for _, r := range a.chats() {
		switch {
		case r.Favourite:
			favs = append(favs, r)
		case r.LowPriority:
			low = append(low, r)
		default:
			rest = append(rest, r)
		}
	}
	section := func(title string, icon *ui.SVG, list []model.Room) {
		if len(list) == 0 {
			return
		}
		lines = append(lines, listLine{"sec:" + title, func() { a.sectionHeader(c, title, icon) }})
		for _, r := range list {
			r := r
			lines = append(lines, listLine{"room:" + r.ID, func() { a.chatRow(c, r) }})
		}
	}
	section("Favourites", kit.IconStar, favs)
	section("Chats", nil, rest)
	section("Low priority", nil, low)
	if len(favs)+len(rest)+len(low) == 0 && len(a.pendingInvites()) == 0 {
		lines = append(lines, listLine{"empty", func() { emptyNote(c, "No chats yet. Start one from People.") }})
	}
	return lines
}

// sectionHeader is a list's title: a small muted label with an icon.
func (a *App) sectionHeader(c *ui.Context, title string, icon *ui.SVG) {
	p := kit.Of(c)
	ui.Row(c.Key("sec:"+title)).Gap(kit.S1).AlignItems(ui.Center).Padding(kit.S2, kit.S2, kit.S1, kit.S2).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(12, 12).TextColor(p.TextMuted)
		}
		ui.Text(c, title).FontSize(kit.SizeCaption + 1).FontWeight(kit.WeightMedium).TextColor(p.TextMuted)
	})
}

// chatRow is a chat: a click opens it; right click (or ⋯) its menu.
func (a *App) chatRow(c *ui.Context, r model.Room) {
	id := r.ID
	sel := a.view == viewRoom && a.room == id && !kit.Compact(c)
	if kit.RoomRow(c, a.row(r), sel, func(m *ui.Menu) {
		a.roomAction(id, kit.RoomMenu(m, a.menuState(r), a.now))
	}).Clicked() {
		a.open(id)
	}
}

// peopleEntry opens People from the top of Chats.
func (a *App) peopleEntry(c *ui.Context) {
	p := kit.Of(c)
	sel := a.view == viewPeople
	b := ui.ButtonBase(c.Key("people")).Height(36).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusMd - 2).
		AlignItems(ui.Center).Justify(ui.Start).Label("People").AccessSelected(sel)
	bg, fg := ui.Transparent, p.Text
	switch {
	case sel:
		bg, fg = p.AccentSubtle, p.AccentStrong
	case b.Hovered():
		bg = p.Hover
	}
	b.Background(b.AnimateColor("bg", bg, kit.Color))
	b.Children(func() {
		ui.Box(c).Width(36).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, kit.IconUsers).Size(18, 18).TextColor(fg)
		})
		ui.Text(c, "People").FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).TextColor(fg).Grow(1)
		ui.Text(c, strconv.Itoa(a.onlineContacts())+" online").FontSize(kit.SizeCaption).TextColor(p.TextMuted)
	})
	if b.Clicked() {
		a.view = viewPeople
	}
}

func emptyNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(kit.SizeSmall).TextColor(kit.Of(c).TextMuted).Padding(kit.S4, kit.S3)
}

// ---- Communities ----------------------------------------------------------

// communities are the spaces, favourites first, each group in the server's
// order.
func (a *App) communities() []model.Community {
	var all []model.Community
	for _, cm := range a.communityList() {
		if cm.Parent == "" { // sub-spaces live inside their parent
			all = append(all, cm)
		}
	}
	out := make([]model.Community, 0, len(all))
	for _, cm := range all {
		if cm.Favourite {
			out = append(out, cm)
		}
	}
	for _, cm := range all {
		if !cm.Favourite {
			out = append(out, cm)
		}
	}
	return out
}

// communityMenu is a community's menu: the favourite, and the settings
// that come with the Matrix layer.
func (a *App) communityMenu(cm model.Community, m *ui.Menu) {
	fav := "Add to favourites"
	if cm.Favourite {
		fav = "Remove from favourites"
	}
	if m.Item(fav).Checked(cm.Favourite).Chosen() {
		a.src.SetCommunityFavourite(cm.ID, !cm.Favourite)
	}
	if _, ok := a.src.(model.CommunityOrder); ok {
		ids := a.communityIDs()
		first, last := len(ids) > 0 && ids[0] == cm.ID, len(ids) > 0 && ids[len(ids)-1] == cm.ID
		if m.Item("Move up").Disabled(first).Chosen() {
			a.moveCommunityBy(cm.ID, -1)
		}
		if m.Item("Move down").Disabled(last).Chosen() {
			a.moveCommunityBy(cm.ID, 1)
		}
	}
	m.Separator()
	if m.Item("Community settings").Chosen() {
		a.openRoomEdit(cm.ID)
	}
}

// communityHome is a community's header: picture, name, how many are in
// it, what it is about, Invite and settings.
func (a *App) communityHome(c *ui.Context, cm model.Community) {
	p := kit.Of(c)
	ui.Column(c).Gap(kit.S2).Padding(kit.S3).Children(func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: cm.Name, ID: cm.ID, Size: 48, Square: true, Image: a.avatarPic(cm.Avatar)})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Row(c).Gap(kit.S1).AlignItems(ui.Center).Children(func() {
					ui.Text(c, cm.Name).FontSize(kit.SizeLabel + 1).FontWeight(kit.WeightSemibold).SingleLine().Shrink(1)
					if cm.Favourite {
						ui.Icon(c, kit.IconStar).Size(12, 12).TextColor(p.TextMuted).Label("Favourite")
					}
				})
				ui.RichText(c,
					ui.Span{Text: "● ", Color: p.Success},
					ui.Span{Text: compact(cm.Online) + " online · " + compact(cm.Members) + " members", Color: p.TextMuted},
				).FontSize(kit.SizeSmall).SingleLine()
			})
		})
		if cm.About != "" {
			ui.Text(c, cm.About).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		}
		ui.Row(c).Gap(kit.S2).Children(func() {
			if kit.Button(c, kit.Secondary, "Invite", kit.IconUser).Grow(1).Clicked() {
				a.openInvite(cm.ID)
			}
			menu := kit.IconButton(c, kit.Secondary, kit.IconSettings, "Community settings")
			kit.MenuButton(c, menu, func(m *ui.Menu) { a.communityMenu(cm, m) })
		})
	})
}

// communityRooms lists a community's rooms.
func (a *App) communityRooms(c *ui.Context, cm model.Community) {
	a.communityRoomsAt(c, cm, 0)
	a.joinableRooms(c, cm) // what it offers that the user is not in
}

// communityRoomsAt lists a community's rooms, then each sub-space as a
// titled group with its own rooms (nested a level deeper, never past four).
func (a *App) communityRoomsAt(c *ui.Context, cm model.Community, depth int) {
	for _, id := range cm.Rooms {
		r, ok := a.src.Room(id)
		if !ok {
			continue
		}
		if a.roomLine(c, r, a.view == viewRoom && a.room == id && !kit.Compact(c)).Clicked() {
			a.open(id)
		}
	}
	if depth >= 3 {
		return
	}
	for _, sid := range cm.Spaces {
		sub, ok := a.communityOf(sid)
		if !ok {
			continue
		}
		a.sectionHeader(c, sub.Name, nil)
		a.communityRoomsAt(c, sub, depth+1)
	}
}

// communityRoomIDs is every room of a community, its sub-spaces' too.
func (a *App) communityRoomIDs(cm model.Community, depth int) []string {
	ids := append([]string(nil), cm.Rooms...)
	if depth >= 3 {
		return ids
	}
	for _, sid := range cm.Spaces {
		if sub, ok := a.communityOf(sid); ok {
			ids = append(ids, a.communityRoomIDs(sub, depth+1)...)
		}
	}
	return ids
}

// roomLine is a community's room: # and its name, quiet unless unread,
// with its menu on right click.
func (a *App) roomLine(c *ui.Context, r model.Room, selected bool) ui.Element {
	p := kit.Of(c)
	b := ui.ButtonBase(c.Key("room:" + r.ID)).Height(32).PaddingX(kit.S2).Gap(kit.S2).Radius(kit.RadiusSm).
		AlignItems(ui.Center).Justify(ui.Start).Label(r.Name).AccessSelected(selected)
	if kit.Touch(c) {
		b.Height(44)
	}
	unread := (r.Unread > 0 || r.Mentions > 0) && !r.Muted
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
		ui.Text(c, r.Name).FontSize(kit.SizeBody).FontWeight(w).TextColor(fg).SingleLine().Grow(1).MinWidth(0)
		switch {
		case r.Call:
			ui.Icon(c, kit.IconPhone).Size(13, 13).TextColor(p.Success).Label("Call in progress")
		case r.Muted:
			ui.Icon(c, kit.IconBellOff).Size(13, 13).TextColor(p.TextFaint).Label("Muted")
		case r.Mentions > 0:
			kit.Count(c, r.Mentions, true)
		}
	})
	id := r.ID
	kit.ContextMenu(c, b, func(m *ui.Menu) { a.roomAction(id, kit.RoomMenu(m, a.menuState(r), a.now)) })
	return b
}

// communitiesTab is the Tabs layout's Communities: cards, or the one
// opened with its rooms.
func (a *App) communitiesTab(c *ui.Context) {
	p := kit.Of(c)
	if cm, ok := a.communityOf(a.community); ok {
		back := ui.ButtonBase(c.Key("back")).Height(28).PaddingX(kit.S1).Gap(kit.S1).Radius(kit.RadiusSm).AlignItems(ui.Center).
			AlignSelf(ui.Start).Label("All communities")
		back.Children(func() {
			ui.Icon(c, kit.IconChevronL).Size(14, 14).TextColor(p.TextMuted)
			ui.Text(c, "All communities").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		})
		if back.Clicked() {
			a.community = ""
		}
		a.communityHome(c, cm)
		ui.Text(c, "Rooms").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).Padding(kit.S2, kit.S2, kit.S1, kit.S2)
		a.communityRooms(c, cm)
		return
	}
	var favs, rest []model.Community
	for _, cm := range a.communities() {
		if cm.Favourite {
			favs = append(favs, cm)
		} else {
			rest = append(rest, cm)
		}
	}
	if len(favs) > 0 {
		a.sectionHeader(c, "Favourites", kit.IconStar)
	}
	for _, cm := range favs {
		a.communityCard(c, cm)
	}
	if len(favs) > 0 && len(rest) > 0 {
		a.sectionHeader(c, "All communities", nil)
	}
	for _, cm := range rest {
		a.communityCard(c, cm)
	}
	ui.Box(c).Height(kit.S2)
	if kit.Button(c, kit.Secondary, "Join or create a community", kit.IconPlus).Margin(0, kit.S2, 0, kit.S2).Clicked() {
		a.openRooms(2, 0)
	}
}

// communityCard is a community in the Tabs list: a click opens it, right
// click its menu.
func (a *App) communityCard(c *ui.Context, cm model.Community) {
	p := kit.Of(c)
	b := ui.ButtonBase(c.Key("cm" + cm.ID)).Padding(kit.S2).Gap(kit.S3).Radius(kit.RadiusMd).AlignItems(ui.Center).Justify(ui.Start).Label(cm.Name)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	unread, ment := a.communityCounts(cm)
	b.Children(func() {
		kit.Avatar(c, kit.AvatarSpec{Name: cm.Name, ID: cm.ID, Size: 44, Square: true, Image: a.avatarPic(cm.Avatar)})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, cm.Name).FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold).SingleLine()
			ui.Text(c, strconv.Itoa(len(a.communityRoomIDs(cm, 0)))+" rooms · "+compact(cm.Online)+" online").FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
		})
		switch {
		case ment > 0:
			kit.Count(c, ment, true)
		case unread:
			kit.Dot(c, false)
		}
	})
	kit.ContextMenu(c, b, func(m *ui.Menu) { a.communityMenu(cm, m) })
	a.reorderable(c, b, cm) // drag a card to where it belongs
	if b.Clicked() {
		a.community = cm.ID
	}
}

// ---- One inbox ------------------------------------------------------------

// inboxEntry is a chat or a community in One inbox's list.
type inboxEntry struct {
	room *model.Room
	comm *model.Community
}

// inboxData is what One inbox lists, for the filter chosen: kept while the
// source does not change (it takes a pass over every room of every community).
type inboxData struct {
	filter   int
	favs     []model.Room
	favComms []model.Community
	list     []inboxEntry
}

// inboxList is One inbox's list: starred chats under Favourites, then
// chats and communities by recency; a community unfolds to its rooms. Only
// the lines in view are built.
func (a *App) inboxList(c *ui.Context) {
	d := a.inboxModel()
	p := kit.Of(c)
	var lines []listLine
	if len(d.favs) > 0 || len(d.favComms) > 0 {
		lines = append(lines, listLine{"sec:Favourites", func() { a.sectionHeader(c, "Favourites", kit.IconStar) }})
		for _, r := range d.favs {
			r := r
			lines = append(lines, listLine{"room:" + r.ID, func() { a.chatRow(c, r) }})
		}
		for _, cm := range d.favComms {
			cm := cm
			lines = append(lines, listLine{"comm:" + cm.ID, func() { a.inboxCommunity(c, cm) }})
		}
		lines = append(lines, listLine{"favs-end", func() {
			ui.Box(c.Key("favs-end")).Height(1).Background(p.Border).Margin(kit.S1, kit.S2)
		}})
	}
	for _, e := range d.list {
		e := e
		if e.room != nil {
			lines = append(lines, listLine{"room:" + e.room.ID, func() { a.chatRow(c, *e.room) }})
		} else {
			lines = append(lines, listLine{"comm:" + e.comm.ID, func() { a.inboxCommunity(c, *e.comm) }})
		}
	}
	if len(d.list) == 0 {
		lines = append(lines, listLine{"empty", func() { emptyNote(c, "Nothing here.") }})
	}
	a.showLines(c, "inbox", &a.inboxLS, lines)
}

// inboxModel is the data of inboxList.
func (a *App) inboxModel() *inboxData {
	a.scEnsure()
	if d := a.sc.inbox; d != nil && d.filter == a.filter {
		return d
	}
	d := a.buildInbox()
	a.sc.inbox = d
	return d
}

func (a *App) buildInbox() *inboxData {
	unread := func(r model.Room) bool { return r.Unread > 0 || r.Mentions > 0 }
	keep := func(r model.Room) bool { return a.filter != 3 || unread(r) }
	var favs []model.Room
	if a.filter != 2 {
		for _, r := range a.chats() {
			if r.Favourite && keep(r) {
				favs = append(favs, r)
			}
		}
	}
	var favComms []model.Community
	if a.filter != 1 {
		for _, cm := range a.communities() {
			if !cm.Favourite {
				continue
			}
			if u, m := a.communityCounts(cm); a.filter == 3 && !u && m == 0 {
				continue
			}
			favComms = append(favComms, cm)
		}
	}
	// Chats and communities together, by their latest activity.
	type entry = inboxEntry
	var list []entry
	if a.filter != 2 {
		for _, r := range a.chats() {
			if !r.Favourite && keep(r) {
				r := r
				list = append(list, entry{room: &r})
			}
		}
	}
	if a.filter != 1 {
		comms := append([]model.Community(nil), a.communityList()...)
		for i := range comms {
			if comms[i].Parent != "" {
				continue // a sub-space is listed inside its parent
			}
			if comms[i].Favourite {
				continue // already in Favourites above
			}
			u, m := a.communityCounts(comms[i])
			if a.filter == 3 && !u && m == 0 {
				continue
			}
			// After the first chat as recent as the community's latest room.
			at := len(list)
			latest := a.latest(comms[i])
			for j, e := range list {
				if e.room != nil && e.room.LastActivity.Before(latest) {
					at = j
					break
				}
			}
			list = append(list[:at], append([]entry{{comm: &comms[i]}}, list[at:]...)...)
		}
	}
	return &inboxData{filter: a.filter, favs: favs, favComms: favComms, list: list}
}

func (a *App) latest(cm model.Community) (t time.Time) {
	for _, id := range cm.Rooms {
		if r, ok := a.src.Room(id); ok && r.LastActivity.After(t) {
			t = r.LastActivity
		}
	}
	return t
}

// inboxCommunity is a community in the inbox: a row unfolding its rooms.
func (a *App) inboxCommunity(c *ui.Context, cm model.Community) {
	p := kit.Of(c)
	open := a.expanded[cm.ID]
	b := ui.ButtonBase(c.Key("ic" + cm.ID)).Height(kit.RowHeight).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusMd - 2).
		AlignItems(ui.Center).Justify(ui.Start).Label(cm.Name).Expanded(open)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	unread, ment := a.communityCounts(cm)
	b.Children(func() {
		kit.Avatar(c, kit.AvatarSpec{Name: cm.Name, ID: cm.ID, Size: 36, Square: true, Image: a.avatarPic(cm.Avatar)})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, cm.Name).FontSize(kit.SizeBody).FontWeight(kit.WeightSemibold).SingleLine()
			ui.Text(c, strconv.Itoa(len(a.communityRoomIDs(cm, 0)))+" rooms · "+compact(cm.Online)+" online").FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
		})
		if ment > 0 {
			kit.Count(c, ment, true)
		} else if unread {
			kit.Dot(c, false)
		}
		chev := ui.Icon(c, kit.IconChevronD).Size(14, 14).TextColor(p.TextMuted)
		r := float32(-90)
		if open {
			r = 0
		}
		chev.Rotate(chev.AnimateWith("r", r, kit.DurFast, kit.EaseOut))
	})
	a.reorderable(c, b, cm) // drag a community to where it belongs
	if b.Clicked() {
		a.expanded[cm.ID] = !open
	}
	if open {
		ui.Column(c.Key("icr" + cm.ID)).PaddingStart(36 + kit.S3).Gap(1).
			Transition(ui.ElementTransition{Duration: kit.DurBase, ExitDuration: kit.DurFast, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).
			Children(func() { a.communityRooms(c, cm) })
	}
}

func compact(n int) string {
	if n >= 1000 {
		return strconv.FormatFloat(float64(n)/1000, 'f', 1, 64) + "k"
	}
	return strconv.Itoa(n)
}

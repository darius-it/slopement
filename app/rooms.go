package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Joining, making and finding rooms and communities, inviting people and
// answering invitations: the dialogs and the list section over a source
// that is a model.RoomManager (the real account; the mock has none, and the
// dialogs say so).

// roomsDialog is "Join or create": three tabs, Join (an address or link),
// Create (a private room, a public one, a community) and Explore (the
// homeserver's directory).
type roomsDialog struct {
	Sheet       kit.SheetState
	Tab         int // 0 join, 1 create, 2 explore
	Target      string
	Name, Topic string
	Kind        int // 0 private room, 1 public room, 2 community
	Query       string
	Found       []model.PublicRoom
	Asked       string // the query Found answers; searched again when it differs
	Busy        bool
	Err         string
}

// noSearch marks "not searched yet": no query equals it.
const noSearch = "\x00"

// openRooms opens the dialog on a tab (and, for Create, a kind).
func (a *App) openRooms(tab, kind int) {
	d := &a.rd
	d.Tab, d.Kind, d.Err, d.Busy = tab, kind, "", false
	d.Target, d.Name, d.Topic, d.Query, d.Found, d.Asked = "", "", "", "", nil, noSearch
	d.Sheet.Open = true
}

// roomManager is the source's room management, if it has any.
func (a *App) roomManager() (model.RoomManager, bool) {
	rm, ok := a.src.(model.RoomManager)
	return rm, ok
}

// enter takes the user to a room just joined or made, as soon as the sync has
// brought it (it is a moment for a joined one).
func (a *App) enter(room string, space bool) {
	if space {
		a.community, a.view = room, viewRoom
		return
	}
	a.pendingOpen, a.pendingOpenBy = room, time.Now().Add(20*time.Second)
}

// pendingStep opens the room asked for as soon as it exists; gives up after
// a while.
func (a *App) pendingStep() {
	if a.pendingOpen == "" {
		return
	}
	if _, ok := a.src.Room(a.pendingOpen); ok {
		id := a.pendingOpen
		a.pendingOpen = ""
		a.community = ""
		a.open(id)
	} else if time.Now().After(a.pendingOpenBy) {
		a.pendingOpen = ""
	}
}

// roomsOverlays draws the dialogs (call it with the other overlays).
func (a *App) roomsOverlays(c *ui.Context) {
	a.roomEditView(c)
	a.roomsDialogView(c)
	a.inviteDialogView(c)
}

func (a *App) roomsDialogView(c *ui.Context) {
	d := &a.rd
	if !d.Sheet.Open && !kit.SheetAlive(&d.Sheet) {
		return
	}
	p := kit.Of(c)
	rm, ok := a.roomManager()
	body := func() {
		ui.Column(c).Gap(kit.S3).Width(420).MaxWidthPercent(100).Children(func() {
			kit.SheetTitle(c, "Join or create", &d.Sheet.Open)
			if !ok {
				ui.Text(c, "This account can't join or create rooms here.").FontSize(kit.SizeBody).TextColor(p.TextMuted)
				return
			}
			kit.Tabs(c.Key("rooms-tabs"), &d.Tab, "By address", "New", "Explore")
			switch d.Tab {
			case 0:
				a.joinTab(c, rm)
			case 1:
				a.createTab(c, rm)
			default:
				a.exploreTab(c, rm)
			}
			if d.Err != "" {
				ui.Text(c, d.Err).FontSize(kit.SizeSmall).TextColor(p.Danger)
			}
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, "", body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, "", 460, body)
	}
}

func (a *App) joinTab(c *ui.Context, rm model.RoomManager) {
	d := &a.rd
	p := kit.Of(c)
	ui.Text(c, "A room's address (#room:server.org), its ID, or a link to it.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	in := kit.TextField(c.Key("join-target"), &d.Target, "Room address or link", "#room:server.org")
	if (kit.Button(c, kit.Primary, "Join room", nil).Clicked() || in.Submitted()) && !d.Busy && strings.TrimSpace(d.Target) != "" {
		d.Busy, d.Err = true, ""
		rm.JoinRoom(d.Target, func(room, err string) {
			d.Busy = false
			if err != "" {
				d.Err = err
				return
			}
			d.Sheet.Open = false
			a.enter(room, false)
		})
	}
	if d.Busy {
		ui.Text(c, "Joining…").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	}
}

func (a *App) createTab(c *ui.Context, rm model.RoomManager) {
	d := &a.rd
	p := kit.Of(c)
	kit.Tabs(c.Key("create-kind"), &d.Kind, "Private room", "Public room", "Community")
	note := []string{
		"Invite-only and end-to-end encrypted.",
		"Anyone can find and join it. It gets an address from its name.",
		"A home for rooms: it appears on the rail, and the rooms you add to it list under it.",
	}[d.Kind]
	ui.Text(c, note).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	name := kit.TextField(c.Key("create-name"), &d.Name, "Name", "")
	topic := kit.TextField(c.Key("create-topic"), &d.Topic, "What it is about (optional)", "")
	label := "Create room"
	if d.Kind == 2 {
		label = "Create community"
	}
	if (kit.Button(c, kit.Primary, label, nil).Clicked() || name.Submitted() || topic.Submitted()) && !d.Busy && strings.TrimSpace(d.Name) != "" {
		d.Busy, d.Err = true, ""
		spec := model.RoomSpec{Name: d.Name, Topic: d.Topic, Space: d.Kind == 2, Public: d.Kind == 1}
		rm.CreateRoom(spec, func(room, err string) {
			d.Busy = false
			if err != "" {
				d.Err = err
				return
			}
			d.Sheet.Open = false
			a.enter(room, spec.Space)
		})
	}
	if d.Busy {
		ui.Text(c, "Creating…").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	}
}

func (a *App) exploreTab(c *ui.Context, rm model.RoomManager) {
	d := &a.rd
	p := kit.Of(c)
	in := kit.TextField(c.Key("explore-q"), &d.Query, "Search the directory", "name or topic")
	if (kit.Button(c, kit.Secondary, "Search", kit.IconSearch).Clicked() || in.Submitted()) && !d.Busy {
		d.Asked = noSearch
	}
	if d.Asked != d.Query && d.Asked == noSearch && !d.Busy {
		d.Busy, d.Err = true, ""
		q := d.Query
		rm.PublicRooms(q, "", func(rooms []model.PublicRoom, err string) {
			d.Busy, d.Asked, d.Found, d.Err = false, q, rooms, err
		})
	}
	if d.Busy {
		ui.Text(c, "Looking…").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	} else if len(d.Found) == 0 && d.Err == "" && d.Asked != noSearch {
		ui.Text(c, "Nothing found.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	}
	ui.Scroll(c.Key("explore-list")).MaxHeight(320).Gap(kit.S1).Children(func() {
		for _, r := range d.Found {
			r := r
			ui.Row(c.Key("pub:"+r.ID)).Gap(kit.S3).AlignItems(ui.Center).Padding(kit.S2).Radius(kit.RadiusSm).Children(func() {
				kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 36, Square: true, Colors: &a.colors, Image: a.avatarPic(r.Avatar)})
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					title := r.Name
					if r.Space {
						title += " · community"
					}
					ui.Text(c, title).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).SingleLine()
					sub := strconv.Itoa(r.Members) + " members"
					if r.Topic != "" {
						sub += " · " + strings.Join(strings.Fields(r.Topic), " ")
					}
					ui.Text(c, sub).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
				})
				switch {
				case r.Joined:
					ui.Text(c, "Joined").FontSize(kit.SizeSmall).TextColor(p.TextFaint)
				case kit.Button(c, kit.Secondary, "Join", nil).Height(28).Label("Join "+r.Name).Clicked():
					target := r.Alias
					if target == "" {
						target = r.ID
					}
					d.Err = ""
					rm.JoinRoom(target, func(room, err string) {
						if err != "" {
							d.Err = err
							return
						}
						d.Sheet.Open = false
						a.enter(room, r.Space)
					})
				}
			})
		}
	})
}

// ---- Inviting people and starting chats ----------------------------------------

// inviteDialog finds people to ask into a room, or (Chat) to start a chat
// with.
type inviteDialog struct {
	Sheet kit.SheetState
	Room  string // the room to invite to; "" with Chat
	Chat  bool   // start a chat instead
	Query string
	Found []model.Person
	Asked string
	Busy  bool
	Err   string
	Sent  string // who was just invited
}

func (a *App) openInvite(room string) {
	d := &a.inv
	d.Room, d.Chat, d.Query, d.Found, d.Asked, d.Err, d.Sent, d.Busy = room, false, "", nil, noSearch, "", "", false
	d.Sheet.Open = true
}

// openAddPerson is "Add person" in People: find someone and start a chat.
func (a *App) openAddPerson() {
	a.openInvite("")
	a.inv.Chat = true
}

func (a *App) inviteDialogView(c *ui.Context) {
	d := &a.inv
	if !d.Sheet.Open && !kit.SheetAlive(&d.Sheet) {
		return
	}
	p := kit.Of(c)
	rm, ok := a.roomManager()
	title, action := "Invite people", "Invite"
	if d.Chat {
		title, action = "Start a chat", "Message"
	}
	body := func() {
		ui.Column(c).Gap(kit.S3).Width(420).MaxWidthPercent(100).Children(func() {
			kit.SheetTitle(c, title, &d.Sheet.Open)
			if !ok {
				ui.Text(c, "This account can't invite people here.").FontSize(kit.SizeBody).TextColor(p.TextMuted)
				return
			}
			kit.TextField(c.Key("invite-q"), &d.Query, "Name or address", "@name:server.org").AutoFocus()
			// Search as the text changes (one at a time; the latest text is
			// searched again when an answer was for an older one).
			q := strings.TrimSpace(d.Query)
			if q != d.Asked && !d.Busy {
				if len(q) < 2 {
					d.Found, d.Asked = nil, q
				} else {
					d.Busy, d.Asked = true, q
					rm.SearchPeople(q, func(people []model.Person, err string) {
						d.Busy, d.Found, d.Err = false, people, err
					})
				}
			}
			ui.Scroll(c.Key("invite-list")).MaxHeight(300).Gap(kit.S1).Children(func() {
				for _, pe := range d.Found {
					pe := pe
					if pe.ID == a.src.Me().ID {
						continue
					}
					ui.Row(c.Key("who:"+pe.ID)).Gap(kit.S3).AlignItems(ui.Center).Padding(kit.S2).Children(func() {
						kit.Avatar(c, a.avatar(pe, 32))
						ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
							ui.Text(c, pe.Name).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).SingleLine()
							ui.Text(c, pe.ID).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
						})
						if kit.Button(c, kit.Secondary, action, nil).Height(28).Clicked() {
							a.pickPerson(rm, pe)
						}
					})
				}
			})
			switch {
			case d.Err != "":
				ui.Text(c, d.Err).FontSize(kit.SizeSmall).TextColor(p.Danger)
			case d.Sent != "":
				ui.Text(c, "Invited "+d.Sent).FontSize(kit.SizeSmall).TextColor(p.Success)
			case d.Busy:
				ui.Text(c, "Looking…").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			case len(d.Found) == 0 && len(strings.TrimSpace(d.Query)) >= 2:
				ui.Text(c, "No one found. A full address like @name:server.org works too.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			}
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, "", body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, "", 460, body)
	}
}

// pickPerson invites the person chosen, or opens a chat with them.
func (a *App) pickPerson(rm model.RoomManager, pe model.Person) {
	d := &a.inv
	d.Err, d.Sent = "", ""
	if d.Chat {
		d.Sheet.Open = false
		a.community = ""
		a.openDM(pe.ID, a.open)
		return
	}
	rm.Invite(d.Room, pe.ID, func(err string) {
		if err != "" {
			d.Err = err
			return
		}
		d.Sent = pe.Name
	})
}

// ---- Invitations --------------------------------------------------------------

// pendingInvites are the invitations waiting (none on a source that has none).
func (a *App) pendingInvites() []model.Invite {
	if rm, ok := a.roomManager(); ok {
		return rm.Invites()
	}
	return nil
}

// invitesSection lists the invitations waiting at the top of Chats, each
// with Accept and Decline.
func (a *App) invitesSection(c *ui.Context) {
	rm, ok := a.roomManager()
	if !ok {
		return
	}
	invites := rm.Invites()
	if len(invites) == 0 {
		return
	}
	p := kit.Of(c)
	a.sectionHeader(c, "Invitations", kit.IconBell)
	for _, inv := range invites {
		inv := inv
		who := inv.Inviter
		if pe, ok := a.src.Person(inv.Inviter); ok || pe.Name != "" {
			who = pe.Name
		}
		ui.Column(c.Key("inv:"+inv.ID)).Gap(kit.S2).Padding(kit.S2, kit.S2, kit.S2, kit.S2).Radius(kit.RadiusMd).
			Background(p.AccentSubtle.Alpha(0.5)).Margin(0, 0, kit.S1, 0).Children(func() {
			ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
				kit.Avatar(c, kit.AvatarSpec{Name: inv.Name, ID: inv.ID, Size: 36, Square: !inv.DM, Colors: &a.colors, Image: a.avatarPic(inv.Avatar)})
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, inv.Name).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).SingleLine()
					line := "Invited"
					if who != "" {
						line += " by " + who
					}
					if inv.DM {
						line = who + " wants to chat"
					}
					ui.Text(c, line).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
				})
			})
			ui.Row(c).Gap(kit.S2).Children(func() {
				if kit.Button(c, kit.Primary, "Accept", nil).Height(28).Grow(1).Clicked() {
					rm.AcceptInvite(inv.ID, func(room, err string) {
						if err != "" {
							a.toast = err
							return
						}
						a.enter(room, inv.Space)
					})
				}
				if kit.Button(c, kit.Secondary, "Decline", nil).Height(28).Grow(1).Clicked() {
					rm.DeclineInvite(inv.ID)
				}
			})
		})
	}
}

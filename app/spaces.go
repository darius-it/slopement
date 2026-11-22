package app

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// ---- The rail's order -----------------------------------------------------

// railDrag is a community being dragged on the rail.
type railDrag struct{ ID string }

// communityIDs are the communities of the rail, in the order shown.
func (a *App) communityIDs() []string {
	var out []string
	for _, cm := range a.communities() {
		out = append(out, cm.ID)
	}
	return out
}

// moveCommunity moves a community to another's place on the rail: the one
// dragged goes where the target was (after it when dragged down, before it
// when dragged up). Kept with the account where the source can.
func (a *App) moveCommunity(id, target string) {
	ids := a.communityIDs()
	from, to := -1, -1
	for i, x := range ids {
		switch x {
		case id:
			from = i
		case target:
			to = i
		}
	}
	if from < 0 || to < 0 || from == to {
		return
	}
	ids = append(ids[:from], ids[from+1:]...)
	ids = append(ids[:to], append([]string{id}, ids[to:]...)...)
	a.setCommunityOrder(ids)
}

// moveCommunityBy moves a community one place up (-1) or down (+1).
func (a *App) moveCommunityBy(id string, by int) {
	ids := a.communityIDs()
	for i, x := range ids {
		if x == id && i+by >= 0 && i+by < len(ids) {
			a.moveCommunity(id, ids[i+by])
			return
		}
	}
}

func (a *App) setCommunityOrder(ids []string) {
	if co, ok := a.src.(model.CommunityOrder); ok {
		co.SetCommunityOrder(ids)
	}
}

// ---- Rooms of a community, joined or not ----------------------------------

// hierState is what a community's hierarchy said: asked once per run. Which
// of its rooms the user is in is read live, though — a room left later is
// offered again, one joined elsewhere stops being.
type hierState struct {
	Rooms []model.PublicRoom
	Err   string
	Done  bool
}

// joinableRooms lists, under a community's rooms, everything it offers that
// the user is not in: its own rooms, then each sub-space where it sits in
// the space's order, as a titled group with its own offers beneath. The
// Join buttons are on the rooms themselves; titles are only labels.
func (a *App) joinableRooms(c *ui.Context, cm model.Community) {
	_, ok := a.src.(model.SpaceBrowser)
	rm, ok2 := a.src.(model.RoomManager)
	if !ok || !ok2 {
		return
	}
	if !a.hasMore(cm.ID, 0) {
		return
	}
	p := kit.Of(c)
	ui.Text(c, "More to join").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).
		Padding(kit.S3, kit.S2, kit.S1, kit.S2)
	a.moreSpace(c, cm.ID, rm, 0)
}

// hasMore says whether a space offers anything: a room of its own the user
// is not in, or a sub-space — joined or not — with anything beneath.
func (a *App) hasMore(id string, depth int) bool {
	st := a.hierOf(id)
	if st == nil || !st.Done {
		return false
	}
	for _, r := range st.Rooms {
		if !r.Space && !a.joinedNow(r.ID) {
			return true
		}
	}
	if depth >= 3 {
		return false
	}
	for _, r := range st.Rooms {
		if r.Space && a.hasMore(r.ID, depth+1) {
			return true
		}
	}
	return false
}

// moreSpace renders what one space offers, in the space's own order: its
// rooms to join as rows, and each sub-space as a quiet title over its own
// offers beneath.
func (a *App) moreSpace(c *ui.Context, id string, rm model.RoomManager, depth int) {
	st := a.hierOf(id)
	if st == nil || !st.Done {
		return
	}
	for _, r := range st.Rooms {
		switch {
		case !r.Space && !a.joinedNow(r.ID):
			a.moreRow(c, id, r, rm)
		case r.Space && depth < 3 && a.hasMore(r.ID, depth+1):
			a.moreHead(c, r)
			a.moreSpace(c, r.ID, rm, depth+1)
		}
	}
}

// moreHead is a sub-space's quiet title under "More to join".
func (a *App) moreHead(c *ui.Context, r model.PublicRoom) {
	p := kit.Of(c)
	ui.Text(c.Key("mg:"+r.ID), r.Name).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine().
		Padding(kit.S1, kit.S2, 0, kit.S2)
}

// hierOf is what a community's hierarchy said; it asks the first time and
// answers nil until it has.
func (a *App) hierOf(id string) *hierState {
	sb, ok := a.src.(model.SpaceBrowser)
	if !ok {
		return nil
	}
	if a.hier == nil {
		a.hier = map[string]*hierState{}
	}
	st := a.hier[id]
	if st == nil {
		st = &hierState{}
		a.hier[id] = st
		sb.SpaceRooms(id, func(rooms []model.PublicRoom, err string) {
			st.Rooms, st.Err, st.Done = rooms, err, true
		})
		return nil
	}
	if !st.Done {
		return nil
	}
	return st
}

// moreRow is one room to join: a row with a Join.
func (a *App) moreRow(c *ui.Context, space string, r model.PublicRoom, rm model.RoomManager) {
	p := kit.Of(c)
	ui.Row(c.Key("more:"+space+":"+r.ID)).Gap(kit.S2).AlignItems(ui.Center).Padding(kit.S1, kit.S2).Radius(kit.RadiusSm).Children(func() {
		kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Square: r.Space, Colors: &a.colors, Image: a.avatarPic(r.Avatar)})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, r.Name).FontSize(kit.SizeBody).SingleLine()
			sub := strconv.Itoa(r.Members) + " members"
			if r.Topic != "" {
				sub += " · " + strings.Join(strings.Fields(r.Topic), " ")
			}
			ui.Text(c, sub).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
		})
		if kit.Button(c, kit.Secondary, "Join", nil).Height(26).Label("Join "+r.Name).Clicked() {
			a.joinOffer(r, rm)
		}
	})
}

// joinOffer joins a room offered from a hierarchy list: it takes effect at
// once, and the room opens when the sync has brought it.
func (a *App) joinOffer(r model.PublicRoom, rm model.RoomManager) {
	target := r.Alias
	if target == "" {
		target = r.ID
	}
	rm.JoinRoom(target, func(room, err string) {
		if err != "" {
			a.toast = err
			return
		}
		if room == "" {
			room = r.ID
		}
		if a.justJoined == nil {
			a.justJoined = map[string]bool{}
		}
		a.justJoined[room] = true
		a.enter(room, false)
	})
}

// joinedNow says whether the user is in a room of a hierarchy: live from
// the source, or joined from one of these lists just now (the sync has not
// landed yet). Leaving is the same in reverse: the source no longer has
// the room, so it is offered again.
func (a *App) joinedNow(id string) bool {
	if _, ok := a.src.Room(id); ok {
		return true
	}
	return a.justJoined[id]
}

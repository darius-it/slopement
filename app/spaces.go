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

// hierState is what a community's hierarchy said: asked once per run.
type hierState struct {
	Rooms []model.PublicRoom
	Err   string
	Done  bool
}

// joinableRooms lists, under a community's rooms, the rooms it offers that
// the user is not in, each with a Join.
func (a *App) joinableRooms(c *ui.Context, cm model.Community) {
	sb, ok := a.src.(model.SpaceBrowser)
	rm, ok2 := a.src.(model.RoomManager)
	if !ok || !ok2 {
		return
	}
	if a.hier == nil {
		a.hier = map[string]*hierState{}
	}
	st := a.hier[cm.ID]
	if st == nil {
		st = &hierState{}
		a.hier[cm.ID] = st
		sb.SpaceRooms(cm.ID, func(rooms []model.PublicRoom, err string) {
			st.Rooms, st.Err, st.Done = rooms, err, true
		})
		return
	}
	p := kit.Of(c)
	var more []int
	for i, r := range st.Rooms {
		if !r.Joined {
			more = append(more, i)
		}
	}
	if len(more) == 0 {
		return
	}
	ui.Text(c, "More to join").FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).
		Padding(kit.S3, kit.S2, kit.S1, kit.S2)
	for _, i := range more {
		i, r := i, st.Rooms[i]
		ui.Row(c.Key("more:"+r.ID)).Gap(kit.S2).AlignItems(ui.Center).Padding(kit.S1, kit.S2).Radius(kit.RadiusSm).Children(func() {
			kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Square: !r.Space, Colors: &a.colors, Image: a.avatarPic(r.Avatar)})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, r.Name).FontSize(kit.SizeBody).SingleLine()
				sub := strconv.Itoa(r.Members) + " members"
				if r.Topic != "" {
					sub += " · " + strings.Join(strings.Fields(r.Topic), " ")
				}
				ui.Text(c, sub).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
			})
			if kit.Button(c, kit.Secondary, "Join", nil).Height(26).Label("Join "+r.Name).Clicked() {
				target := r.Alias
				if target == "" {
					target = r.ID
				}
				rm.JoinRoom(target, func(room, err string) {
					if err != "" {
						a.toast = err
						return
					}
					st.Rooms[i].Joined = true
				})
			}
		})
	}
}

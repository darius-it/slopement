package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// forwardDialog is "Forward": a message is chosen, and a chat to send a copy
// of it to.
type forwardDialog struct {
	Sheet    kit.SheetState
	Room, ID string // the message
	Query    string
	Err      string
	Sending  bool
	list     ui.ListState
}

// openForward asks where to forward a message.
func (a *App) openForward(room, id string) {
	if _, ok := a.src.(model.Forwarder); !ok {
		return
	}
	a.fwd = forwardDialog{Room: room, ID: id}
	a.fwd.Sheet.Open = true
}

// forwardTargets are the chats a message can be sent to: the ones the user
// may write in, matching the query, the most recent first.
func (a *App) forwardTargets(query string) []model.Room {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []model.Room
	for _, r := range a.roomList() {
		if r.ReadOnly {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Name), q) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (a *App) forwardDialogView(c *ui.Context) {
	d := &a.fwd
	if !d.Sheet.Open {
		return
	}
	fw, ok := a.src.(model.Forwarder)
	if !ok {
		d.Sheet.Open = false
		return
	}
	p := kit.Of(c)
	body := func() {
		targets := a.forwardTargets(d.Query)
		ui.Column(c).Gap(kit.S2).Children(func() {
			kit.SearchField(c.Key("fwd-q"), &d.Query, "Search chats", "").AutoFocus()
			if d.Err != "" {
				ui.Text(c, d.Err).FontSize(kit.SizeSmall).TextColor(p.Danger)
			}
			if len(targets) == 0 {
				emptyNote(c, "No chat matches.")
				return
			}
			ui.Column(c.Key("fwd-box")).Height(340).Children(func() {
				ui.List(c.Key("fwd-list"), &d.list, len(targets), func(i int) {
					r := targets[i]
					row := ui.ButtonBase(c.Key("fwd:" + r.ID)).Height(44).PaddingX(kit.S2).Gap(kit.S3).Radius(kit.RadiusSm).
						AlignItems(ui.Center).Justify(ui.Start).Label("Forward to " + r.Name)
					if row.Hovered() {
						row.Background(p.Hover)
					}
					row.Children(func() {
						kit.Avatar(c, kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 28, Square: r.Kind != model.KindDM, Colors: &a.colors})
						ui.Text(c, r.Name).FontSize(kit.SizeBody).SingleLine().Grow(1)
					})
					if row.Clicked() && !d.Sending {
						d.Sending, d.Err = true, ""
						room, id, name, to := d.Room, d.ID, r.Name, r.ID
						fw.Forward(room, id, to, func(err string) {
							d.Sending = false
							if err != "" {
								d.Err = err
								return
							}
							d.Sheet.Open = false
							a.toast = "Forwarded to " + name
						})
					}
				}).Grow(1)
			})
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, "Forward to…", body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, "Forward to…", 420, body)
	}
}

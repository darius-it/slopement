package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// roomNickEditor is the dialog for a room's private name: the name the user
// sees it by, kept with their account and shown to nobody else.
type roomNickEditor struct {
	Sheet kit.SheetState
	ID    string
	Name  string // what is being typed
	Real  string // what the room is called for everyone
}

// openRoomNick opens the private name of a room or chat.
func (a *App) openRoomNick(id string) {
	if _, ok := a.src.(model.PrivateRoomNames); !ok {
		return
	}
	r, ok := a.src.Room(id)
	if !ok {
		return
	}
	a.roomNickEd = roomNickEditor{ID: id, Name: a.roomNickname(id), Real: r.Name}
	a.roomNickEd.Sheet.Open = true
}

func (a *App) roomNickView(c *ui.Context) {
	d := &a.roomNickEd
	if !d.Sheet.Open {
		return
	}
	pn, ok := a.src.(model.PrivateRoomNames)
	if !ok {
		d.Sheet.Open = false
		return
	}
	p := kit.Of(c)
	body := func() {
		ui.Column(c).Gap(kit.S3).Children(func() {
			ui.Text(c, "Called "+d.Real+" for everyone").FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
			kit.TextField(c.Key("room-nick"), &d.Name, "Name", d.Real).AutoFocus()
			ui.Text(c, "Only you see this, on all your devices. Others still see the room's own name. It is stored with your account, so your homeserver can read it.").
				FontSize(kit.SizeCaption).TextColor(p.TextMuted)
			ui.Row(c).Gap(kit.S2).Justify(ui.End).Children(func() {
				if a.roomNickname(d.ID) != "" && kit.Button(c, kit.Ghost, "Remove", nil).Clicked() {
					pn.SetRoomNickname(d.ID, "")
					d.Sheet.Open = false
				}
				if kit.Button(c, kit.Secondary, "Cancel", nil).Clicked() {
					d.Sheet.Open = false
				}
				if kit.Button(c, kit.Primary, "Save", nil).Clicked() {
					pn.SetRoomNickname(d.ID, strings.TrimSpace(d.Name))
					d.Sheet.Open = false
				}
			})
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, "Private name", body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, "Private name", 420, body)
	}
}

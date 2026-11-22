package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// Editing a room or a community: its picture, name and topic, in a dialog,
// offering each only where the user's power in the room allows it (asked of
// the source when the dialog opens).

type roomEdit struct {
	Sheet       kit.SheetState
	Room        string
	Space       bool
	Name, Topic string
	Perms       model.RoomPerms
	Loaded      bool // the permissions have come
	Busy        bool
	Err         string
}

// canEditRooms says the source can change rooms' details.
func (a *App) canEditRooms() bool {
	_, ok := a.src.(model.RoomEditor)
	return ok
}

// editTarget is what the dialog shows of a room or community: a space is a
// room to the real source, a community only to a source that lists it so.
func (a *App) editTarget(id string) (r model.Room, ok bool) {
	if r, ok = a.src.Room(id); ok {
		return r, true
	}
	if cm, found := a.communityOf(id); found {
		return model.Room{ID: cm.ID, Name: cm.Name, Topic: cm.About, Avatar: cm.Avatar}, true
	}
	return model.Room{}, false
}

// openRoomEdit opens the dialog on a room or a community.
func (a *App) openRoomEdit(room string) {
	ed, ok := a.src.(model.RoomEditor)
	r, have := a.editTarget(room)
	if !ok || !have {
		a.toast = "This account can't change rooms here"
		return
	}
	_, isSpace := a.communityOf(room)
	d := &a.re
	d.Room, d.Space, d.Name, d.Topic = room, isSpace, r.Name, r.Topic
	d.Perms, d.Loaded, d.Busy, d.Err = model.RoomPerms{}, false, false, ""
	d.Sheet.Open = true
	ed.RoomPermissions(room, func(p model.RoomPerms) { d.Perms, d.Loaded = p, true })
}

func (a *App) roomEditView(c *ui.Context) {
	d := &a.re
	if !d.Sheet.Open && !kit.SheetAlive(&d.Sheet) {
		return
	}
	ed, ok := a.src.(model.RoomEditor)
	r, have := a.editTarget(d.Room)
	if !ok || !have {
		d.Sheet.Open = false
		return
	}
	p := kit.Of(c)
	title := "Room settings"
	if d.Space {
		title = "Community settings"
	}
	body := func() {
		ui.Column(c).Gap(kit.S3).Width(420).MaxWidthPercent(100).Children(func() {
			kit.SheetTitle(c, title, &d.Sheet.Open)
			ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
				av := kit.AvatarSpec{Name: r.Name, ID: r.ID, Size: 64, Square: true, Colors: &a.colors, Image: a.avatarPic(r.Avatar)}
				if d.Perms.Avatar {
					if kit.AvatarEditor(c, av).Clicked() {
						a.pickPicture("Choose a picture", func(f model.File) {
							d.Err = ""
							ed.SetRoomAvatar(d.Room, f, func(err string) { d.Err = err })
						})
					}
				} else {
					kit.Avatar(c, av)
				}
				if d.Perms.Avatar && r.Avatar != "" && kit.Button(c, kit.Ghost, "Remove picture", nil).Clicked() {
					d.Err = ""
					ed.RemoveRoomAvatar(d.Room, func(err string) { d.Err = err })
				}
			})
			if d.Loaded && !d.Perms.Name && !d.Perms.Topic && !d.Perms.Avatar {
				ui.Text(c, "You don't have permission to change this.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
				return
			}
			name := kit.TextField(c.Key("re-name"), &d.Name, "Name", "")
			topic := kit.TextField(c.Key("re-topic"), &d.Topic, "What it is about", "")
			if d.Loaded && !d.Perms.Name {
				ui.Text(c, "You can't change its name.").FontSize(kit.SizeCaption).TextColor(p.TextFaint)
			}
			if d.Loaded && !d.Perms.Topic {
				ui.Text(c, "You can't change its topic.").FontSize(kit.SizeCaption).TextColor(p.TextFaint)
			}
			changedName := d.Perms.Name && strings.TrimSpace(d.Name) != r.Name && strings.TrimSpace(d.Name) != ""
			changedTopic := d.Perms.Topic && strings.TrimSpace(d.Topic) != r.Topic
			if (kit.Button(c, kit.Primary, "Save", nil).Clicked() || name.Submitted() || topic.Submitted()) && !d.Busy &&
				(changedName || changedTopic) {
				a.saveRoomEdit(ed, changedName, changedTopic)
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

// saveRoomEdit sends what changed, the name then the topic, and closes the
// dialog when both went; a refusal stays in it, in a line.
func (a *App) saveRoomEdit(ed model.RoomEditor, name, topic bool) {
	d := &a.re
	d.Busy, d.Err = true, ""
	finish := func(err string) bool {
		if err != "" {
			d.Busy, d.Err = false, err
			return false
		}
		return true
	}
	sendTopic := func() {
		if !topic {
			d.Busy, d.Sheet.Open = false, false
			return
		}
		ed.SetRoomTopic(d.Room, d.Topic, func(err string) {
			if finish(err) {
				d.Busy, d.Sheet.Open = false, false
			}
		})
	}
	if name {
		ed.SetRoomName(d.Room, d.Name, func(err string) {
			if finish(err) {
				sendTopic()
			}
		})
		return
	}
	sendTopic()
}

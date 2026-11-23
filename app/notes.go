package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// noteEditor is the dialog for the nickname and note the user gives someone.
// Both are private: kept with the user's account, shown to nobody else.
type noteEditor struct {
	Sheet kit.SheetState
	ID    string
	Nick  string
	Note  string
	Name  string // their own display name, for the title
}

// openNoteEditor opens the nickname and note of a person.
func (a *App) openNoteEditor(id string) {
	if _, ok := a.src.(model.PrivateNotes); !ok {
		return
	}
	p, _ := a.src.Person(id)
	n := a.noteOf(id)
	a.noteEd = noteEditor{ID: id, Nick: n.Nick, Note: n.Note, Name: p.Name}
	a.noteEd.Sheet.Open = true
}

func (a *App) noteEditorView(c *ui.Context) {
	d := &a.noteEd
	if !d.Sheet.Open {
		return
	}
	pn, ok := a.src.(model.PrivateNotes)
	if !ok {
		d.Sheet.Open = false
		return
	}
	p := kit.Of(c)
	title := "Nickname and note"
	body := func() {
		ui.Column(c).Gap(kit.S3).Children(func() {
			ui.Text(c, d.Name+" "+d.ID).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine()
			kit.TextField(c.Key("note-nick"), &d.Nick, "Nickname", d.Name).AutoFocus()
			kit.TextField(c.Key("note-note"), &d.Note, "Note", "Only you can see this")
			ui.Text(c, "Only you see these, on all your devices. Others still see their own name for you. They are stored with your account, so your homeserver can read them.").
				FontSize(kit.SizeCaption).TextColor(p.TextMuted)
			ui.Row(c).Gap(kit.S2).Justify(ui.End).Children(func() {
				n := a.noteOf(d.ID)
				if (n.Nick != "" || n.Note != "") && kit.Button(c, kit.Ghost, "Remove", nil).Clicked() {
					pn.SetPersonNote(d.ID, model.PersonNote{})
					d.Sheet.Open = false
				}
				if kit.Button(c, kit.Secondary, "Cancel", nil).Clicked() {
					d.Sheet.Open = false
				}
				if kit.Button(c, kit.Primary, "Save", nil).Clicked() {
					pn.SetPersonNote(d.ID, model.PersonNote{Nick: strings.TrimSpace(d.Nick), Note: strings.TrimSpace(d.Note)})
					d.Sheet.Open = false
				}
			})
		})
	}
	if kit.Compact(c) {
		kit.Sheet(c, &d.Sheet, title, body)
	} else {
		kit.Dialog(c, &d.Sheet.Open, title, 420, body)
	}
}

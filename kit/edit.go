package kit

import (
	"github.com/egoist/mygo/ui"
	rich "github.com/egoist/mygo/ui/richtext"
)

// Editing a message. Where there is room it happens in the message's own
// place (EditBox, on a ComposerState of its own), as in Element and desktop
// Discord; a phone edits in the composer, which sets the draft aside until
// the edit is over (BeginEdit, EndEdit), so neither overwrites the other.

// draftStash is what the composer held when an edit took it over.
type draftStash struct {
	text  string
	rich  *rich.Text // the text with its mention marks
	files []ComposerFile
	reply *Quote
}

// BeginEdit takes the field for the message id, holding its text. What was
// being written is set aside; EndEdit brings it back. Starting another edit
// during one keeps the first draft.
func (st *ComposerState) BeginEdit(id, text string) {
	if st.Editing == "" {
		d := &draftStash{text: st.Text, files: st.Files, reply: st.Reply}
		if st.doc != nil {
			st.ensureDoc()
			t := st.doc.Text()
			d.rich = &t
		}
		st.stash = d
		st.Files, st.Reply = nil, nil
	}
	st.Editing, st.Text = id, text
}

// EndEdit leaves the edit, saved or not, and puts the draft back. Files
// added meanwhile wait with it.
func (st *ComposerState) EndEdit() {
	d := st.stash
	st.Editing, st.Text, st.stash = "", "", nil
	if d == nil {
		return
	}
	st.Text, st.Reply = d.text, d.reply
	st.Files = append(d.files, st.Files...)
	if d.rich != nil && st.doc != nil {
		st.doc.SetText(*d.rich)
		n := st.doc.Text().Len()
		st.doc.SetSelection(n, n)
		st.synced = st.Text
	}
}

// DraftText is the text of the new message being written, also while an edit
// has the field: what is kept between launches.
func (st *ComposerState) DraftText() string {
	if st.stash != nil {
		return st.stash.text
	}
	if st.Editing != "" {
		return ""
	}
	return st.Text
}

// EditBox is a message's text being edited where the message is: the field,
// then Cancel and Save. Enter saves and Escape cancels (it clears
// st.Editing); on a touch screen Enter is a new line.
func EditBox(c *ui.Context, st *ComposerState) (save bool) {
	p := Of(c)
	touch := Touch(c)
	ui.Column(c.Key("editbox:"+st.Editing)).Gap(S15).PaddingY(S05).Children(func() {
		ui.Column(c).Padding(S2, S3).Radius(RadiusMd).Background(p.BgInset).Border(1, p.Border).Children(func() {
			composerField(c.Key("edit-input"), st, "Edit message", fieldSize(c), 12, touch)
		})
		ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
			hint := ""
			if !touch {
				hint = "Esc to cancel · Enter to save"
			}
			ui.Text(c, hint).FontSize(SizeCaption).TextColor(p.TextFaint).SingleLine().Grow(1).MinWidth(0)
			if Button(c.Key("edit-cancel"), Ghost, "Cancel", nil).Clicked() {
				st.EndEdit()
				c.Invalidate()
			}
			if Button(c.Key("edit-save"), Primary, "Save", nil).Clicked() {
				save = true
			}
		})
	})
	if st.send {
		st.send, save = false, true
	}
	st.wantPaste = false // an edit takes words only
	return save
}

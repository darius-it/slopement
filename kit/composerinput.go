package kit

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	rich "github.com/egoist/mygo/ui/richtext"
)

// The composer's text field is mygo's rich text editor kept to plain text:
// no Markdown shortcuts, no formatting keys, plain paste — what is typed is
// what is sent. What the editor gives that a text area cannot is a mark on a
// run of the text, which is how a mention chosen from the suggestions shows
// in the field: its name in a colored frame, a mention that is valid (it has
// a person behind it), sent as a link to them.
//
// ComposerState.Text stays the text, as before (drafts, tests and the rest
// of the app read and write it); the editor's document follows it, and the
// state follows the document.

// ClipboardText reads plain text from the system clipboard; the app sets it
// when it has a window. Without it (tests) the paste shortcut is left to the
// app's own handling of pictures and files.
var ClipboardText func() string

// ensureDoc makes the editor's document, and brings it in step with Text if
// the text was changed from outside (a draft restored, the field cleared by
// a send, a message taken to be edited): the document is then that text, its
// marks gone, the caret at its end.
func (st *ComposerState) ensureDoc() {
	if st.doc == nil {
		st.doc = rich.NewDocument(rich.NewText(st.Text))
		st.synced = st.Text
		n := utf8.RuneCountInString(st.Text)
		st.doc.SetSelection(n, n)
		return
	}
	if st.Text != st.synced {
		st.doc.SetText(rich.NewText(st.Text))
		n := utf8.RuneCountInString(st.Text)
		st.doc.SetSelection(n, n)
		st.synced = st.Text
	}
}

// pullDoc takes the document's text back into Text, after the editor or an
// edit of ours changed it.
func (st *ComposerState) pullDoc() {
	if st.doc == nil {
		return
	}
	if s := st.doc.Text().String(); s != st.synced {
		st.Text, st.synced = s, s
	}
}

// Complete replaces the text from startRune (a rune offset) to its end —
// the word being typed — with a suggestion and a space after it, the caret
// after them: the user goes on typing. A mention is marked with the person
// it is of, which is what draws its frame and makes it a link when sent.
func (st *ComposerState) Complete(startRune int, it SuggestItem) {
	st.ensureDoc()
	n := st.doc.Text().Len()
	startRune = max(0, min(startRune, n))
	mark := rich.Attributes{}
	if it.ID != "" {
		mark.Tag = it.ID
	}
	end := startRune + utf8.RuneCountInString(it.Insert)
	st.doc.Edit(func(e *rich.Edit) {
		e.ReplaceString(startRune, n, it.Insert, mark)
		e.ReplaceString(end, end, " ", rich.Attributes{}) // plain: typing continues after the frame
		e.Select(end+1, end+1)
	})
	st.pullDoc()
	st.wantFocus = true
}

// Focus gives the field the keyboard focus on the next frame: after Reply
// or Edit the user types at once.
func (st *ComposerState) Focus() { st.wantFocus = true }

// insertPlain puts plain text at the caret (over the selection).
func (st *ComposerState) insertPlain(s string) {
	st.ensureDoc()
	a, b := st.doc.Selection()
	if a > b {
		a, b = b, a
	}
	st.doc.Edit(func(e *rich.Edit) {
		e.ReplaceString(a, b, s, rich.Attributes{})
		e.Select(a+utf8.RuneCountInString(s), a+utf8.RuneCountInString(s))
	})
	st.pullDoc()
}

// Draft is what is to be sent: the text, with each mention chosen from the
// suggestions as a Markdown link to the person (matrix.to), which the sender
// makes into a pill and a notification. A mention whose frame was edited
// into something else is still a link to the same person, under what the
// text now says.
func (st *ComposerState) Draft() string {
	if st.doc == nil {
		return st.Text
	}
	st.ensureDoc() // Text may have been set from outside since the field last built
	var b strings.Builder
	first := true
	for _, para := range st.doc.Text().Paragraphs() {
		if !first {
			b.WriteByte('\n')
		}
		first = false
		for _, run := range para.Runs {
			if run.Attributes.Tag != "" && strings.TrimSpace(run.Text) != "" {
				label := strings.NewReplacer("[", `\[`, "]", `\]`).Replace(run.Text)
				b.WriteString("[" + label + "](https://matrix.to/#/" + run.Attributes.Tag + ")")
			} else {
				b.WriteString(run.Text)
			}
		}
	}
	return b.String()
}

// composerField is the text field of both composers. size is its font size;
// maxLines, how many lines it grows to before it scrolls.
func composerField(c *ui.Context, st *ComposerState, placeholder string, size float32, maxLines int, touch bool) ui.Element {
	p := Of(c)
	st.ensureDoc()
	// EditorBase: the field has no box of its own (the composer draws its
	// border and background), and a mention's frame is a tint of the accent
	// over the composer's own background.
	frame := p.Accent.Alpha(0.2).Over(p.BgInset)
	ed := rich.EditorBase(c, st.doc, rich.EditorOptions{
		Label:       placeholder,
		Placeholder: placeholder,
		NoMarkdown:  true,
		Font:        ui.Font{Size: size},
		// A mention chosen from the suggestions: its name in a frame.
		TagStyle: func(tag string, s *ui.Span) {
			s.Background, s.Color = frame, p.AccentStrong
			s.Weight = WeightMedium
			s.Radius, s.PaddingX, s.PaddingY = RadiusSm, 4, 1
		},
		OnKey: func(k ui.Key, mods ui.Modifiers) bool {
			taken := fieldKey(st, k, mods, touch)
			if taken {
				c.Invalidate() // what the key changed (a pick, the highlight) is drawn at once
			}
			return taken
		},
		OnCommand: func(name string) bool {
			if name == "paste" {
				defer c.Invalidate()
				return st.paste()
			}
			return false
		},
	})
	// As tall as its text: one line to maxLines, then it scrolls. The height
	// is the text's as the editor last laid it out, so a change in it asks
	// for a frame to show it at once.
	line := size * 1.45
	h := line
	if ch, ok := st.doc.ContentHeight(); ok {
		h = min(max(ch, line), line*float32(maxLines))
	}
	if h != st.fieldH {
		st.fieldH = h
		c.Invalidate()
	}
	ed.Grow(1).MinWidth(0).Height(h)
	st.pullDoc()
	if st.wantFocus {
		st.wantFocus = false
		ed.Focus()
	}
	return ed
}

// paste handles the paste shortcut as plain text: whatever text the
// clipboard has goes in unformatted. With no text on it, the app is asked to
// look for a picture or files. It reports whether the editor must do nothing.
func (st *ComposerState) paste() bool {
	if ClipboardText != nil {
		if s := ClipboardText(); s != "" {
			st.insertPlain(s)
			return true
		}
	}
	st.wantPaste = true
	return true // nothing of text to paste, and the editor's own paste would bring in formats
}

// fieldKey is a key pressed in the field. It reports whether the key was
// taken (the editor then does nothing with it).
func fieldKey(st *ComposerState, k ui.Key, mods ui.Modifiers, touch bool) bool {
	tk := ui.TextKey{Key: k, Mods: mods}
	if suggestKey(st, tk) {
		return true // the suggestions took it
	}
	if pasteKey(tk) {
		return st.paste()
	}
	// The word processor's formatting shortcuts have no place in a message
	// that goes as typed.
	if mods&(ui.Super|ui.Ctrl) != 0 {
		switch k {
		case ui.KeyB, ui.KeyI, ui.KeyU, ui.KeyK:
			return true
		}
	}
	if k == ui.KeyEnter && mods == 0 && !touch {
		st.send = true
		return true
	}
	// ↑ with nothing written (and nothing waiting to be sent) edits the last
	// message of yours, as in Element and Discord.
	if k == ui.KeyUp && mods == 0 && !touch && st.Editing == "" && len(st.Files) == 0 && st.Reply == nil &&
		strings.TrimSpace(st.Draft()) == "" {
		st.wantEditLast = true
		return true
	}
	if k == ui.KeyEscape && st.Editing != "" {
		st.Editing, st.Text = "", "" // changed your mind: the old message stays as it was
		return true
	}
	if k == ui.KeyEscape && st.Reply != nil {
		st.Reply = nil
		return true
	}
	return false
}

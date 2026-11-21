package kit

import "github.com/egoist/mygo/ui"

// ComposerState is the composer of one room: its draft and what it
// replies to. Keep one per room so drafts survive switching rooms.
type ComposerState struct {
	Text  string
	Reply *Quote
	// Editing is the ID of the message being edited ("" when writing a new
	// one): the field holds its text, and sending replaces it.
	Editing string
	// Files wait to be sent with the next message.
	Files []ComposerFile
	send  bool
	// What the + menu chose, picked up by the next build.
	wantAttach, wantPoll bool
	// wantPaste: the paste shortcut was pressed in the field.
	wantPaste bool

	// Suggest are what the word being typed could become (a person to
	// mention, an emoji): set by the app before each build, shown above the
	// field. sel is the highlighted one; suggestOff is the text they were
	// put away at with Escape (they come back when it changes); wantPick:
	// Enter or Tab took the highlighted one.
	Suggest    []SuggestItem
	sel        int
	suggestOff string
	wantPick   bool

	// inputGen names the text field: it changes when the app rewrites the
	// text under the caret (a suggestion taken), which makes the field a new
	// one, caret at the end; wantFocus then gives the new field the focus.
	inputGen  int
	wantFocus bool
}

// Rewrote replaces the draft's text from the app (a suggestion taken) and
// keeps the user typing: the field gets the new text with the caret at its
// end, and the keyboard focus, so nothing has to be clicked again.
func (st *ComposerState) Rewrote(text string) {
	st.Text = text
	st.inputGen++
	st.wantFocus = true
}

// inputKey is the key of the text field.
func (st *ComposerState) inputKey() string { return "composer-input-" + itoa(st.inputGen) }

// SuggestionsOff says the user put the suggestions away for this text.
func (st *ComposerState) SuggestionsOff() bool {
	return st.suggestOff != "" && st.suggestOff == st.Text
}

// plusMenu makes the composer's + a menu: a file, or a poll.
func plusMenu(c *ui.Context, plus ui.Element, st *ComposerState) {
	MenuButton(c, plus, func(m *ui.Menu) {
		if m.Item("Upload a file").Icon(MenuIcon(IconPaperclip)).Chosen() {
			st.wantAttach = true
		}
		if m.Item("Create poll").Icon(MenuIcon(IconPoll)).Chosen() {
			st.wantPoll = true
		}
	})
}

// takePlus moves what the + menu chose into the result.
func (st *ComposerState) takePlus(r *ComposerResult) {
	if st.wantAttach {
		r.Attach, st.wantAttach = true, false
	}
	if st.wantPoll {
		r.Poll, st.wantPoll = true, false
	}
	if st.wantPaste {
		r.Paste, st.wantPaste = true, false
	}
	if st.wantPick {
		st.wantPick = false
		if len(st.Suggest) > 0 {
			r.Pick = min(st.sel, len(st.Suggest)-1)
		}
	}
}

// ComposerResult is what the composer asked for in this build. The
// buttons are returned so popovers (the GIF picker) can anchor to them.
type ComposerResult struct {
	Send        bool
	Attach      bool
	Poll        bool // the + menu's Create poll
	// Paste: the paste shortcut was pressed (look for a picture or files).
	Paste bool
	// RemovedFile is the waiting file whose ✕ was pressed, -1 for none.
	RemovedFile int
	// Pick is the suggestion taken (clicked, or Enter or Tab on the
	// highlighted one), -1 for none.
	Pick int
	Emoji       bool
	GIF         bool
	GIFButton   ui.Element
	EmojiButton ui.Element
	// Unified is set by composers with one button for emoji, GIFs and
	// stickers (the phone's): it opens the picker where it was left.
	Unified bool
}

// Composer is the message field: a reply banner when replying, an attach
// button, the text (Enter sends, Shift+Enter breaks the line; on phones
// Enter breaks the line and the send button sends), emoji and GIF
// buttons, and send, which shows only once there is text.
func Composer(c *ui.Context, st *ComposerState, placeholder string, colors *UserColors) (r ComposerResult) {
	p := Of(c)
	touch := Touch(c)
	r.RemovedFile, r.Pick = -1, -1
	ui.Column(c).Padding(0, S4, S4, S4).Children(func() {
		box := ui.Column(c)
		box.Children(func() {
			if q := st.Reply; q != nil {
				ui.Row(c).Gap(S2).Padding(S2, S2, 0, S3).AlignItems(ui.Center).
					Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).
					Children(func() {
						ui.Icon(c, IconReply).Size(14, 14).TextColor(p.TextMuted)
						ui.RichText(c,
							ui.Span{Text: "Replying to ", Color: p.TextMuted},
							ui.Span{Text: q.Sender, Color: colors.Color(p, q.SenderID), Weight: WeightSemibold},
						).FontSize(SizeSmall).SingleLine().Grow(1)
						x := ui.ButtonBase(c).Size(22, 22).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label("Cancel reply")
						if x.Hovered() {
							x.Background(p.Hover)
						}
						x.Children(func() { ui.Icon(c, IconX).Size(14, 14).TextColor(p.TextMuted) })
						if x.Clicked() {
							st.Reply = nil
						}
					})
			}
			editBanner(c, st)
			r.RemovedFile = fileTray(c, st)
			r.Pick = suggestPopup(c, st)
			// Keyed: the banner, the file chips and the suggestions come and
			// go above, and the field must be the same field when they do —
			// by position it would be a new one, and the focus would be gone.
			ui.Row(c.Key("composer-row")).Gap(S1).Padding(S1+2, S1+2).AlignItems(ui.End).Children(func() {
				plusMenu(c, IconButton(c, Ghost, IconPlus, "Attach or create"), st)
				ta := ui.TextAreaBase(c.Key(st.inputKey()), &st.Text).Grow(1).MinWidth(0).Lines(1, 8).
					Placeholder(placeholder).PlaceholderColor(p.TextMuted).Label(placeholder).
					FontSize(fieldSize(c)).LineHeight(1.4).MarginY(6)
				if st.wantFocus {
					st.wantFocus = false
					ta.Focus()
				}
				ta.OnTextKey(func(k ui.TextKey) bool {
					if suggestKey(st, k) {
						return true // the suggestions took the key
					}
					if pasteKey(k) {
						st.wantPaste = true // and the field pastes its text, if any
						return false
					}
					if k.Key == ui.KeyEnter && k.Mods == 0 && !touch {
						st.send = true
						return true
					}
					if k.Key == ui.KeyEscape && st.Editing != "" {
						st.Editing, st.Text = "", "" // changed your mind: the old message stays as it was
						return true
					}
					if k.Key == ui.KeyEscape && st.Reply != nil {
						st.Reply = nil
						return true
					}
					return false
				})
				r.EmojiButton = IconButton(c, Ghost, IconSmile, "Emoji")
				r.Emoji = r.EmojiButton.Clicked()
				r.GIFButton = IconButton(c, Ghost, IconGIF, "GIFs and stickers")
				r.GIF = r.GIFButton.Clicked()
				if st.canSend() {
					s := controlHeight(c)
					b := ui.ButtonBase(c).Size(s, s).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).
						Label("Send").Transition(ui.ElementTransition{Duration: DurFast, ExitDuration: DurExit, Enter: &ui.Motion{Scale: 0.9}, Exit: &ui.Motion{Scale: 0.9}})
					bg := p.Accent
					if b.Pressed() {
						bg = p.AccentPressed
					} else if b.Hovered() {
						bg = p.AccentHover
					}
					b.Background(b.AnimateColor("bg", bg, Color))
					Pressable(b)
					b.Children(func() { ui.Icon(c, IconSend).Size(16, 16).TextColor(p.OnAccent) })
					if b.Clicked() {
						st.send = true
					}
				}
			})
		})
		border := p.Border
		if box.FocusWithin() {
			border = p.BorderStrong
		}
		box.Radius(RadiusMd+2).Background(p.BgInset).Border(1, box.AnimateColor("bd", border, Color))
	})
	if st.send {
		st.send = false
		if st.canSend() {
			r.Send = true
		}
	}
	st.takePlus(&r)
	return r
}

func hasText(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\n' && r != '\t' {
			return true
		}
	}
	return false
}

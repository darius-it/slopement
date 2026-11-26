package kit

import (
	"github.com/egoist/mygo/ui"
	rich "github.com/egoist/mygo/ui/richtext"
)

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
	// wantPhoto and wantCamera: the + menu's photo library and camera.
	wantPhoto, wantCamera bool
	// plusSheet is the phone's + sheet (plusSheet).
	plusSheet SheetState
	// wantPaste: the paste shortcut was pressed in the field.
	wantPaste bool
	// wantEditLast: ↑ was pressed with nothing written (see fieldKey).
	wantEditLast bool

	// Voice messages. CanRecord is set by the app when this device can record
	// (a microphone is there): the mic button shows with nothing written. Voice
	// is the recording in progress, which the composer then shows in place of its
	// field. The want flags are what was pressed, handed on by takePlus.
	CanRecord                               bool
	Voice                                   VoiceState
	wantMic, wantVoiceCancel, wantVoiceSend bool

	// Suggest are what the word being typed could become (a person to
	// mention, an emoji): set by the app before each build, shown above the
	// field. sel is the highlighted one; suggestOff is the text they were
	// put away at with Escape (they come back when it changes); wantPick:
	// Enter or Tab took the highlighted one.
	Suggest    []SuggestItem
	sel        int
	suggestOff string
	wantPick   bool

	// The field is an editor over doc (composerinput.go): synced is the text
	// doc held when Text last matched it, to tell a change from outside;
	// wantFocus gives the field the keyboard focus on the next build.
	doc       *rich.Document
	synced    string
	wantFocus bool
	fieldH    float32 // the field's height last frame

	// OnPick, set by the app, takes a suggestion at once when Tab picks it,
	// in the key's own handling — so the frame that follows the key already
	// shows it. (Taken after the build instead, it would wait for the next
	// event to be drawn.)
	OnPick func(SuggestItem)
}

// SuggestionsOff says the user put the suggestions away for this text.
func (st *ComposerState) SuggestionsOff() bool {
	return st.suggestOff != "" && st.suggestOff == st.Text
}

// plusMenu makes the composer's + a menu: a file, or a poll. On a phone it
// is a sheet of large tiles instead, as WhatsApp's and Muse's attach sheets
// (Mobbin): a drawn menu's rows were too small for a thumb.
func plusMenu(c *ui.Context, plus ui.Element, st *ComposerState) {
	if Touch(c) {
		if plus.Clicked() {
			st.plusSheet.Open = true
		}
		plusSheet(c, st)
		return
	}
	MenuButton(c, plus, func(m *ui.Menu) {
		if PhoneOS() { // a phone's own picker of photos, and its camera
			if m.Item("Photo or video").Icon(MenuIcon(IconImage)).Chosen() {
				st.wantPhoto = true
			}
			if m.Item("Take a photo").Icon(MenuIcon(IconCamera)).Chosen() {
				st.wantCamera = true
			}
		}
		if m.Item("Upload a file").Icon(MenuIcon(IconPaperclip)).Chosen() {
			st.wantAttach = true
		}
		if m.Item("Create poll").Icon(MenuIcon(IconPoll)).Chosen() {
			st.wantPoll = true
		}
	})
}

// plusSheet is the phone's + : a bottom sheet with a tile for each thing to
// add — a round icon over its name, the whole tile pressable.
func plusSheet(c *ui.Context, st *ComposerState) {
	if !st.plusSheet.Open && !SheetAlive(&st.plusSheet) {
		return
	}
	type tile struct {
		icon  *ui.SVG
		label string
		want  *bool
	}
	var tiles []tile
	if PhoneOS() {
		tiles = append(tiles, tile{IconImage, "Photos", &st.wantPhoto}, tile{IconCamera, "Camera", &st.wantCamera})
	}
	tiles = append(tiles, tile{IconFile, "File", &st.wantAttach}, tile{IconPoll, "Poll", &st.wantPoll})
	Sheet(c, &st.plusSheet, "", func() {
		p := Of(c)
		ui.Row(c).Gap(S2).PaddingY(S2).Justify(ui.SpaceAround).Children(func() {
			for _, t := range tiles {
				b := ui.ButtonBase(c.Key("plus:"+t.label)).Column().Gap(S2).AlignItems(ui.Center).Padding(S1).
					Radius(RadiusMd).MinWidth(72).Label(t.label)
				Pressable(b)
				b.Children(func() {
					ui.Box(c).Size(60, 60).Radius(RadiusFull).Background(p.AccentSubtle).
						AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Icon(c, t.icon).Size(26, 26).TextColor(p.AccentStrong)
					})
					ui.Text(c, t.label).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.Text).SingleLine()
				})
				if b.Clicked() {
					*t.want = true
					st.plusSheet.Open = false
				}
			}
		})
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
	if st.wantPhoto {
		r.Photo, st.wantPhoto = true, false
	}
	if st.wantCamera {
		r.Camera, st.wantCamera = true, false
	}
	if st.wantPaste {
		r.Paste, st.wantPaste = true, false
	}
	if st.wantEditLast {
		r.EditLast, st.wantEditLast = true, false
	}
	if st.wantMic {
		r.Mic, st.wantMic = true, false
	}
	if st.wantVoiceCancel {
		r.VoiceCancel, st.wantVoiceCancel = true, false
	}
	if st.wantVoiceSend {
		r.VoiceSend, st.wantVoiceSend = true, false
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
	Send   bool
	Attach bool
	Poll   bool // the + menu's Create poll
	Photo  bool // the + menu's photo or video from the library (phones)
	Camera bool // the + menu's Take a photo (phones)
	// Paste: the paste shortcut was pressed (look for a picture or files).
	Paste bool
	// EditLast: ↑ was pressed in an empty field: edit the user's last message.
	EditLast bool
	// Mic: the microphone button was pressed (start a voice message);
	// VoiceCancel and VoiceSend: the recording bar's bin and send.
	Mic, VoiceCancel, VoiceSend bool
	// RemovedFile is the waiting file whose ✕ was pressed, -1 for none.
	RemovedFile int
	// Pick is the suggestion taken (clicked, or Enter or Tab on the
	// highlighted one), -1 for none.
	Pick        int
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
						xs := float32(22)
						if Touch(c) {
							xs = 40
						}
						x := ui.ButtonBase(c).Size(xs, xs).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label("Cancel reply")
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
			if st.Voice.Active {
				voiceRow(c, st, false) // recording: the bar takes the field's place
			} else {
				ui.Row(c.Key("composer-row")).Gap(S1).Padding(S1+2, S1+2).AlignItems(ui.End).Children(func() {
					plusMenu(c, IconButton(c, Ghost, IconPlus, "Attach or create"), st)
					// The row is bottom-aligned (it grows upward with the text), so
					// the margin that centers one line on the buttons is what is
					// left of the control's height around the line. The glyphs sit
					// a little high in their line box (the ascenders' room), so the
					// text goes down by optical nudge to read centered on the icons.
					size := fieldSize(c)
					gap := max((controlHeight(c)-size*1.45)/2, 0)
					const nudge = 1.5
					composerField(c.Key("composer-input"), st, placeholder, size, 8, touch).Margin(gap+nudge, 0, max(gap-nudge, 0), 0)
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
					} else if st.CanRecord {
						if IconButton(c.Key("mic"), Ghost, IconMic, "Record a voice message").Clicked() {
							st.wantMic = true
						}
					}
				})
			}
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

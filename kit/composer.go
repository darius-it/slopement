package kit

import "github.com/egoist/mygo/ui"

// ComposerState is the composer of one room: its draft and what it
// replies to. Keep one per room so drafts survive switching rooms.
type ComposerState struct {
	Text  string
	Reply *Quote
	send  bool
}

// ComposerResult is what the composer asked for in this build. The
// buttons are returned so popovers (the GIF picker) can anchor to them.
type ComposerResult struct {
	Send       bool
	Attach     bool
	Emoji      bool
	GIF        bool
	GIFButton  ui.Element
	EmojiButton ui.Element
}

// Composer is the message field: a reply banner when replying, an attach
// button, the text (Enter sends, Shift+Enter breaks the line; on phones
// Enter breaks the line and the send button sends), emoji and GIF
// buttons, and send, which shows only once there is text.
func Composer(c *ui.Context, st *ComposerState, placeholder string, colors *UserColors) (r ComposerResult) {
	p := Of(c)
	touch := Touch(c)
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
			ui.Row(c).Gap(S1).Padding(S1+2, S1+2).AlignItems(ui.End).Children(func() {
				if IconButton(c, Ghost, IconPlus, "Attach a file").Clicked() {
					r.Attach = true
				}
				ta := ui.TextAreaBase(c, &st.Text).Grow(1).MinWidth(0).Lines(1, 8).
					Placeholder(placeholder).PlaceholderColor(p.TextMuted).Label(placeholder).
					FontSize(fieldSize(c)).LineHeight(1.4).MarginY(6)
				ta.OnTextKey(func(k ui.TextKey) bool {
					if k.Key == ui.KeyEnter && k.Mods == 0 && !touch {
						st.send = true
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
				if hasText(st.Text) {
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
		box.Radius(RadiusMd + 2).Background(p.BgInset).Border(1, box.AnimateColor("bd", border, Color))
	})
	if st.send {
		st.send = false
		if hasText(st.Text) {
			r.Send = true
		}
	}
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

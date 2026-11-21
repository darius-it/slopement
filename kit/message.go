package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// Reaction is a reaction key on a message with its count.
type Reaction struct {
	Key   string
	Count int
	Mine  bool
}

// Quote is the message a reply answers.
type Quote struct {
	Sender, SenderID, Body string
}

// MessageData is one event of the timeline.
type MessageData struct {
	ID       string
	Sender   string
	SenderID string
	Avatar   AvatarSpec
	Time     string // "14:02"
	Body     string
	Reply    *Quote
	Reactions []Reaction
	// First is set on the first of a group of consecutive messages from one
	// sender, which shows the avatar and the name.
	First bool
	// Highlight is set when the message mentions the user.
	Highlight bool
	Edited    bool
	// Unverified marks a message from a device not verified.
	Unverified bool
	// Pending is set while it is being sent.
	Pending bool
	// Undecryptable shows the "unable to decrypt" state.
	Undecryptable bool
	// Image, with ImageW/H, is an image message's thumbnail.
	Image          *ui.Bitmap
	ImageW, ImageH float32
}

// MessageAction is what the user did to a message.
type MessageAction struct {
	React  string // a reaction key toggled
	OpenEmoji bool // the reaction picker asked for
	Reply  bool
	More   bool
}

// QuickReactions are offered in the hover toolbar.
var QuickReactions = []string{"👍", "❤️", "😂"}

// MessageGroupGap is the room above the first message of a group.
const MessageGroupGap = S3

// Message is a row of the timeline: flat, not a bubble, with the avatar
// and name on the first message of a group, the time in the gutter of
// the others while hovered, and a toolbar of actions while hovered.
func Message(c *ui.Context, m MessageData, colors *UserColors) (a MessageAction) {
	p := Of(c)
	row := ui.Row(c.Key(m.ID))
	hov := row.Hovered() || row.FocusWithin()
	bg := ui.Transparent
	if m.Highlight {
		bg = p.AccentSubtle.Alpha(0.6)
	}
	if hov {
		bg = p.Hover.Over(bg.Over(p.Bg))
	}
	top := float32(S05)
	if m.First {
		top = MessageGroupGap
	}
	row.Padding(top, S4, S05, S4).Gap(S3).AlignItems(ui.Start).
		Background(row.AnimateColor("bg", bg, Color))
	if m.Highlight {
		row.BorderWidth(0, 0, 0, 2).BorderColor(p.Accent)
	}
	row.Children(func() {
		// The gutter.
		ui.Box(c).Width(36).Shrink(0).AlignItems(ui.End).Children(func() {
			if m.First {
				av := m.Avatar
				av.Size = 36
				if av.Name == "" {
					av.Name, av.ID = m.Sender, m.SenderID
				}
				av.Colors = colors
				Avatar(c, av)
			} else if hov {
				ui.Text(c, m.Time).FontSize(SizeCaption - 1).FontFeatures("tnum").TextColor(p.TextFaint).
					SingleLine().MarginY(3)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(S05).Children(func() {
			if m.First {
				ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
					ui.Text(c, m.Sender).FontSize(SizeBody).FontWeight(WeightSemibold).
						TextColor(colors.Color(p, m.SenderID)).SingleLine().Shrink(1)
					ui.Text(c, m.Time).FontSize(SizeCaption).FontFeatures("tnum").TextColor(p.TextMuted).SingleLine()
					if m.Unverified {
						ui.Icon(c, IconShieldX).Size(12, 12).TextColor(p.Warning).Tooltip("Sent from an unverified device")
					}
				})
			}
			if m.Reply != nil {
				q := m.Reply
				ui.Row(c).Gap(S2).Padding(S1, S2).Radius(RadiusSm).Background(p.BgInset).
					BorderWidth(0, 0, 0, 2).BorderColor(colors.Color(p, q.SenderID)).MaxWidth(520).Children(func() {
					ui.Column(c).Gap(1).MinWidth(0).Children(func() {
						ui.Text(c, q.Sender).FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(colors.Color(p, q.SenderID)).SingleLine()
						ui.Text(c, q.Body).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
					})
				})
			}
			switch {
			case m.Undecryptable:
				ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconLock).Size(14, 14).TextColor(p.TextMuted)
					ui.Text(c, "Waiting for this message. It will appear when the keys arrive.").
						FontSize(SizeBody).Italic().TextColor(p.TextMuted)
				})
			default:
				fg := p.Text
				if m.Pending {
					fg = p.TextMuted
				}
				if m.Body != "" {
					spans := []ui.Span{{Text: m.Body, Color: fg}}
					if m.Edited {
						spans = append(spans, ui.Span{Text: "  (edited)", Color: p.TextFaint, Size: SizeCaption})
					}
					ui.RichText(c, spans...).FontSize(SizeBody).LineHeight(1.45).Selectable()
				}
				if m.Image != nil {
					w, h := fitBox(m.ImageW, m.ImageH, 360, 280)
					ui.Image(c, m.Image).Size(w, h).Fit(ui.Cover).Radius(RadiusMd).MarginY(S1).Label("Image")
				}
			}
			if len(m.Reactions) > 0 {
				ui.Row(c).Wrap().Gap(S1).MarginY(S05).Children(func() {
					for _, r := range m.Reactions {
						if ReactionPill(c, r).Clicked() {
							a.React = r.Key
						}
					}
				})
			}
		})
		// The hover toolbar.
		if hov {
			ui.Row(c).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(-12).Left(-S4).
				Padding(2).Gap(1).Radius(RadiusSm+2).Background(p.Raised).Border(1, p.Border).
				Shadow(0, 2, 6, -2, p.Shadow).Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{}}).
				Children(func() {
					for _, k := range QuickReactions {
						b := toolButton(c, "React "+k, func(fg ui.Color) { ui.Text(c, k).FontSize(15) })
						if b.Clicked() {
							a.React = k
						}
					}
					if toolButton(c, "Add reaction", func(fg ui.Color) { ui.Icon(c, IconSmile).Size(16, 16).TextColor(fg) }).Clicked() {
						a.OpenEmoji = true
					}
					if toolButton(c, "Reply", func(fg ui.Color) { ui.Icon(c, IconReply).Size(16, 16).TextColor(fg) }).Clicked() {
						a.Reply = true
					}
					if toolButton(c, "More", func(fg ui.Color) { ui.Icon(c, IconMore).Size(16, 16).TextColor(fg) }).Clicked() {
						a.More = true
					}
				})
		}
	})
	return a
}

func toolButton(c *ui.Context, label string, content func(fg ui.Color)) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(label)).Size(28, 28).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Tooltip(label)
	fg := p.TextMuted
	if b.Hovered() {
		b.Background(p.Hover)
		fg = p.Text
	}
	b.Children(func() { content(fg) })
	return b
}

// ReactionPill is a reaction under a message: its key and count, tinted
// when the user reacted. Clicked toggles it.
func ReactionPill(c *ui.Context, r Reaction) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key("r:" + r.Key))
	bg, border, fg := p.BgInset, p.BgInset, p.TextMuted
	if r.Mine {
		bg, border, fg = p.AccentSubtle, p.Accent.Alpha(0.5), p.AccentStrong
	}
	if b.Hovered() {
		border = p.BorderStrong
		if r.Mine {
			border = p.Accent
		}
	}
	b.Height(24).PaddingX(S2).Gap(S1).Radius(RadiusFull).AlignItems(ui.Center).
		Background(bg).Border(1, b.AnimateColor("bd", border, Color)).Checked(r.Mine).
		Label(r.Key + " " + strconv.Itoa(r.Count))
	Pressable(b)
	b.Children(func() {
		ui.Text(c, r.Key).FontSize(13)
		ui.Text(c, strconv.Itoa(r.Count)).FontSize(SizeCaption+1).FontWeight(WeightMedium).FontFeatures("tnum").
			TextColor(fg).ContentTransition(ui.ContentTransition{Numeric: true})
	})
	return b
}

// DateSeparator is the line between days of the timeline.
func DateSeparator(c *ui.Context, label string) {
	p := Of(c)
	ui.Row(c.Key("date:"+label)).Gap(S3).AlignItems(ui.Center).Padding(S4, S4, S1, S4).Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(p.Border)
		ui.Text(c, label).FontSize(SizeCaption).FontWeight(WeightMedium).TextColor(p.TextMuted).SingleLine()
		ui.Box(c).Height(1).Grow(1).Background(p.Border)
	})
}

// SystemEvent is a quiet line for membership and state changes.
func SystemEvent(c *ui.Context, text string) {
	p := Of(c)
	ui.Row(c).Padding(S05, S4, S05, S4+36+S3).Children(func() {
		ui.Text(c, text).FontSize(SizeSmall).TextColor(p.TextMuted)
	})
}

// TypingLine says who is typing, under the timeline.
func TypingLine(c *ui.Context, text string) {
	p := Of(c)
	ui.Row(c).Height(20).PaddingX(S4).Gap(S2).AlignItems(ui.Center).Children(func() {
		if text == "" {
			return
		}
		dots := ui.Row(c).Gap(3)
		dots.Children(func() {
			for i := 0; i < 3; i++ {
				d := ui.Box(c).Size(4, 4).Radius(RadiusFull).Background(p.TextMuted)
				phase := d.Loop("t", 1200_000_000, nil)
				o := 0.35 + 0.65*pulse(phase, float32(i)/3)
				d.Opacity(o)
			}
		})
		ui.Text(c, text).FontSize(SizeCaption+1).TextColor(p.TextMuted).SingleLine()
	})
}

func pulse(t, off float32) float32 {
	x := t - off
	if x < 0 {
		x += 1
	}
	if x < 0.5 {
		return 1 - x*2
	}
	return 0
}

// fitBox fits w×h inside maxW×maxH, keeping its aspect ratio.
func fitBox(w, h, maxW, maxH float32) (float32, float32) {
	if w <= 0 || h <= 0 {
		return maxW, maxH * 0.75
	}
	s := min(maxW/w, maxH/h, 1)
	return w * s, h * s
}

package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/format"
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
	// Avatar is the sender's picture, shown small beside their name.
	Avatar AvatarSpec
	// ID is the event replied to: pressing the quote asks to go there
	// (MessageAction.JumpTo).
	ID string
}

// MessageData is one event of the timeline.
type MessageData struct {
	ID        string
	Sender    string
	SenderID  string
	Avatar    AvatarSpec
	Time      string // "14:02"
	Body      string
	Reply     *Quote
	Reactions []Reaction
	// First is set on the first of a group of consecutive messages from one
	// sender, which shows the avatar and the name.
	First bool
	// Highlight is set when the message mentions the user.
	Highlight bool
	Edited    bool
	// Unverified marks a message from a device not verified.
	Unverified bool
	// Pending is set while it is being sent: it shows as any sent message
	// does — only a failure (Failed, with its Error) is marked.
	Pending bool
	Failed  bool
	Error   string
	// Undecryptable shows the "unable to decrypt" state.
	Undecryptable bool
	// ReadBy are the people who have read up to this message, shown small
	// at the end of its row; empty (the default) shows nothing.
	ReadBy []AvatarSpec
	// Sent shows the hollow check of the user's last message, sent and not
	// read yet.
	Sent bool
	// Menu says what the message's ⋯ and context menus offer.
	Menu MessageMenuState
	// Image, with ImageW/H, is an image message's thumbnail.
	Image          *ui.Bitmap
	ImageW, ImageH float32
	// Doc is the formatted body (package format); when set it is shown
	// in place of Body.
	Doc *format.Node
	// Media, File and Embed are attachments and a link's preview.
	Media []MediaSpec
	File  *FileSpec
	Embed *LinkCard
	// YouTube is a YouTube link's card, in place of Embed.
	YouTube *YouTubeSpec
	// Reveals keeps the message's spoilers open once clicked.
	Reveals Reveals
	// Poll is a poll message (m.poll.start), shown as its card.
	Poll *PollData
	// Profile is the sender's card: set, their name and avatar open it
	// (MessageAction.Profile); ProfileOpen shows it, by the name.
	Profile     *ProfileSpec
	ProfileOpen bool
}

// YouTubeSpec is a YouTube link's card.
type YouTubeSpec struct {
	ID, Title, Channel, Duration, Thumb string
}

// MessageAction is what the user did to a message.
type MessageAction struct {
	React     string // a reaction key toggled
	OpenEmoji bool   // the reaction picker asked for
	Reply     bool
	// OpenMedia is the attachment clicked, or -1.
	OpenMedia int
	// PlayVideo is set when an embedded video's play was pressed.
	PlayVideo bool
	// Vote is the poll answer toggled, or -1; EndPoll ends it.
	Vote    int
	EndPoll bool
	// Profile is set when the sender's avatar or name was clicked;
	// ProfileClosed when their open card was dismissed.
	Profile       bool
	ProfileClosed bool
	// ProfileAct is what the open card asked for.
	ProfileAct ProfileAction
	// JumpTo is the event whose quote was pressed: scroll to it.
	JumpTo string
	// Download: the message's file card was pressed.
	Download bool
	// Menu is what was chosen from the message's menu (Reply is Reply).
	Menu MessageChoice
	// DismissPreview: the × of the message's link card was pressed.
	DismissPreview bool
}

// senderName is a sender's name: in their color, a link to their card
// when there is one (underlined on hover, as links are).
func senderName(c *ui.Context, m MessageData, colors *UserColors, size float32) ui.Element {
	p := Of(c)
	n := ui.Text(c.Key("sender"), m.Sender).FontSize(size).FontWeight(WeightSemibold).
		TextColor(colors.Color(p, m.SenderID)).SingleLine().Shrink(1)
	if m.Profile != nil {
		n.Cursor(ui.CursorPointer).Label("Show " + m.Sender + "'s profile")
		if n.Hovered() {
			n.Underline()
		}
	}
	return n
}

// profilePopover shows a sender's card under anchor while open.
func profilePopover(c *ui.Context, anchor ui.Element, m MessageData, colors *UserColors, a *MessageAction) {
	if m.Profile == nil || !m.ProfileOpen {
		return
	}
	p := Of(c)
	open := true
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Padding(S4).Radius(RadiusLg).Background(p.Raised).Border(1, p.Border).
			Shadow(0, 12, 32, -6, p.Shadow).Transition(PopIn())
		a.ProfileAct = ProfileCard(c, *m.Profile, colors, time.Now())
	})
	if !open || a.ProfileAct.Message || a.ProfileAct.Call || a.ProfileAct.Verify {
		a.ProfileClosed = true
	}
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
	a.OpenMedia, a.Vote = -1, -1
	row := ui.Row(c.Key(m.ID))
	hov := row.Hovered() || row.FocusWithin()
	bg := ui.Transparent
	if m.Highlight {
		bg = p.AccentSubtle.Alpha(0.45)
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
				pic := Avatar(c, av)
				if m.Profile != nil {
					pic.Cursor(ui.CursorPointer)
					if pic.Clicked() {
						a.Profile = true
					}
				}
			} else if hov {
				ui.Text(c, m.Time).FontSize(SizeCaption - 1).FontFeatures("tnum").TextColor(p.TextFaint).
					SingleLine().MarginY(3)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(S05).Children(func() {
			if m.First {
				ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
					name := senderName(c, m, colors, SizeBody)
					if m.Profile != nil && name.Clicked() {
						a.Profile = true
					}
					profilePopover(c, name, m, colors, &a)
					ui.Text(c, m.Time).FontSize(SizeCaption).FontFeatures("tnum").TextColor(p.TextMuted).SingleLine()
					if m.Unverified {
						ui.Icon(c, IconShieldX).Size(12, 12).TextColor(p.Warning).Tooltip("Sent from an unverified device")
					}
				})
			}
			if m.Reply != nil {
				if ReplyQuote(c, m.Reply, colors).Clicked() {
					a.JumpTo = m.Reply.ID
				}
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
				if m.Failed {
					fg = p.Danger
				}
				body := func() {
					if m.Doc != nil {
						MessageBody(c, m.ID, m.Doc, m.Reveals, BodyStyle{Color: fg})
					} else if m.Body != "" {
						ui.RichText(c, ui.Span{Text: m.Body, Color: fg}).FontSize(SizeBody).LineHeight(1.45).Selectable()
					}
				}
				if m.Edited {
					// "(edited)" at the end of the text, on its last line's
					// level: right after a short message, at the end of the
					// column of a long one — not a line of its own.
					ui.Row(c.Key("edited")).Gap(S2).AlignItems(ui.End).Children(func() {
						ui.Column(c).Shrink(1).MinWidth(0).Children(body)
						ui.Text(c, "(edited)").FontSize(SizeCaption).TextColor(p.TextFaint).SingleLine().Shrink(0).MarginY(3)
					})
				} else {
					body()
				}
				if m.Failed {
					why := "Not sent"
					if m.Error != "" {
						why += ": " + m.Error
					}
					ui.Text(c, why).FontSize(SizeCaption).TextColor(p.Danger)
				}
				if m.Image != nil {
					w, h := fitBox(m.ImageW, m.ImageH, 360, 280)
					ui.Image(c, m.Image).Size(w, h).Fit(ui.Cover).Radius(RadiusMd).MarginY(S1).Label("Image")
				}
				if len(m.Media) > 0 {
					if i := MediaGrid(c, m.Media, 400, m.Reveals); i >= 0 {
						a.OpenMedia = i
					}
				}
				if m.File != nil && FileCard(c, *m.File).Clicked() {
					a.Download = true
				}
				if m.Poll != nil {
					pd := *m.Poll
					if pd.Colors == nil {
						pd.Colors = colors
					}
					pa := PollCard(c, pd, time.Now(), 420)
					a.Vote, a.EndPoll = pa.Vote, pa.End
				}
				if y := m.YouTube; y != nil {
					if YouTubeCard(c, *y, 420) {
						a.PlayVideo = true
					}
				} else if m.Embed != nil {
					if LinkPreview(c, *m.Embed, 420) {
						a.DismissPreview = true
					}
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
		// Who has read up to here, at the row's end, level with its last line.
		if len(m.ReadBy) > 0 || m.Sent {
			ui.Box(c).Shrink(0).AlignSelf(ui.End).Padding(0, 0, 2, 0).Children(func() {
				ReadStack(c, m.ReadBy)
				if m.Sent {
					ui.Icon(c, IconCircleCheck).Size(15, 15).TextColor(p.TextFaint).Label("Sent").Tooltip("Sent")
				}
			})
		}
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
					if !m.Menu.Off {
						more := toolButton(c, "More", func(fg ui.Color) { ui.Icon(c, IconMore).Size(16, 16).TextColor(fg) })
						MenuButton(c, more, func(mm *ui.Menu) { a.chose(MessageMenu(mm, m.Menu)) })
					}
				})
		}
	})
	if !m.Menu.Off {
		ContextMenu(c, row, func(mm *ui.Menu) { a.chose(MessageMenu(mm, m.Menu)) })
	}
	return a
}

// chose records a choice of a message's menu in the action. Reply is the
// toolbar's Reply, as before.
func (a *MessageAction) chose(ch MessageChoice) {
	switch ch {
	case MsgNone:
	case MsgReply:
		a.Reply = true
	default:
		a.Menu = ch
	}
}

// ReplyQuote is the line a reply shows above its text: a thin neutral bar,
// the quoted sender, and the start of what they said, on one quiet line.
// It is a button: clicking it jumps to the quoted message.
func ReplyQuote(c *ui.Context, q *Quote, colors *UserColors) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key("reply")).Gap(S2).AlignItems(ui.Center).Justify(ui.Start).MaxWidth(560).
		AlignSelf(ui.Start).PaddingY(1).Label("Reply to " + q.Sender)
	fg := p.TextMuted
	if b.Hovered() {
		fg = p.Text
	}
	b.Children(func() {
		ui.Box(c).Width(2).Height(14).Radius(1).Background(p.BorderStrong).Shrink(0)
		quoteAvatar(c, q, 16, colors)
		ui.RichText(c,
			ui.Span{Text: q.Sender, Color: colors.Color(p, q.SenderID), Weight: WeightMedium},
			ui.Span{Text: "  " + q.Body, Color: b.AnimateColor("fg", fg, Color)},
		).FontSize(SizeSmall).SingleLine().Shrink(1).MinWidth(0)
	})
	return b
}

// quoteAvatar is the small picture of who a reply quotes, by their name:
// their profile picture, or their initials on their color.
func quoteAvatar(c *ui.Context, q *Quote, size float32, colors *UserColors) {
	av := q.Avatar
	if av.Name == "" {
		av.Name, av.ID = q.Sender, q.SenderID
	}
	av.Size, av.Presence, av.Trust, av.Colors = size, PresenceNone, TrustUnknown, colors
	Avatar(c, av)
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
		ui.Text(c, strconv.Itoa(r.Count)).FontSize(SizeCaption + 1).FontWeight(WeightMedium).FontFeatures("tnum").
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
				phase := d.Loop("t", 1200*time.Millisecond, ui.Linear)
				o := 0.35 + 0.65*pulse(phase, float32(i)/3)
				d.Opacity(o)
			}
		})
		ui.Text(c, text).FontSize(SizeCaption + 1).TextColor(p.TextMuted).SingleLine()
	})
}

// TypingOverlay is TypingLine floating on the bottom-left edge of the
// timeline it is built in (its parent): over the messages' own bottom
// margin, on a soft backing so it reads over text, taking no room — a
// line that comes and goes in the flow moves every message above it.
func TypingOverlay(c *ui.Context, text string) {
	if text == "" {
		return
	}
	p := Of(c)
	ui.Row(c.Key("typing")).Attach(ui.AnchorBottomLeft, ui.AnchorBottomLeft).Left(S3).Top(-S1).PassThrough().
		Height(20).PaddingX(S2).Gap(S2).AlignItems(ui.Center).Radius(RadiusFull).Background(p.Bg.Alpha(0.92)).Children(func() {
		dots := ui.Row(c).Gap(3)
		dots.Children(func() {
			for i := 0; i < 3; i++ {
				d := ui.Box(c).Size(4, 4).Radius(RadiusFull).Background(p.TextMuted)
				phase := d.Loop("t", 1200*time.Millisecond, ui.Linear)
				d.Opacity(0.35 + 0.65*pulse(phase, float32(i)/3))
			}
		})
		ui.Text(c, text).FontSize(SizeCaption + 1).TextColor(p.TextMuted).SingleLine()
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

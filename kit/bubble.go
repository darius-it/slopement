package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/format"
)

// The phone chat: bubbles on a soft wallpaper, after KakaoTalk, LINE and
// Signal (Mobbin), in the kit's purple. Incoming bubbles are white with
// the sender's avatar and name over the first of a run; outgoing ones are
// light purple on the right. The first bubble of a run has a sharp corner
// toward its sender, the tail. Times and "Read" sit beside the bubble,
// not in it.

// BubbleData is one message as a bubble.
type BubbleData struct {
	ID        string
	Sender    string
	SenderID  string
	Avatar    AvatarSpec
	Out       bool // sent by the user
	Time      string
	Doc       *format.Node
	Reply     *Quote
	Reactions []Reaction
	// Media are attachments (shown at the bubble's top, or alone);
	// Sticker is a sticker or GIF-sticker, shown without a bubble.
	Media   []MediaSpec
	Sticker *MediaSpec
	File    *FileSpec
	// Embed is a link's preview under the text.
	Embed *LinkCard
	// First and Last place it in a run of one sender's messages.
	First, Last bool
	Read        bool // outgoing: read by the other side
	// ReadBy are the people who have read up to this message, in a room
	// with more than one other (a DM says "Read" instead).
	ReadBy []AvatarSpec
	// Sent: the user's last message, sent and not read yet.
	Sent bool
	// Pending shows as any sent message does; only a failure is marked.
	Pending bool
	Failed  bool
	Error   string
	Edited  bool
	// Live is set on messages that arrive while the room is open: they
	// pop in. History never animates.
	Live bool
	// MaxWidth caps the bubble; zero is 74% of the window (phones).
	MaxWidth float32
	// Poll is a poll, shown as its card in place of a bubble.
	Poll *PollData
	// Profile is the sender's card, opened from their name and avatar.
	Profile     *ProfileSpec
	ProfileOpen bool
}

const (
	bubbleRadius = 18
	tailRadius   = 4
	bubbleAvatar = 32
)

// bubbleRadii returns the corners of a bubble: the tail on the first of a
// run, toward the sender.
func bubbleRadii(out, first bool) (tl, tr, br, bl float32) {
	tl, tr, br, bl = bubbleRadius, bubbleRadius, bubbleRadius, bubbleRadius
	if first {
		if out {
			tr = tailRadius
		} else {
			tl = tailRadius
		}
	}
	return
}

// popIn is a live message's entrance: a little spring from its sender's
// side. Playful, but small, and quick to settle.
func popIn(out bool) ui.ElementTransition {
	x := float32(-10)
	if out {
		x = 10
	}
	spring := ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.18}}
	return ui.ElementTransition{Animation: &spring, Enter: &ui.Motion{X: x, Y: 6, Scale: 0.94}}
}

// Bubble is a message as a bubble, with its sender, time, receipts and
// reactions around it. A long press (or right click) asks for its actions.
func Bubble(c *ui.Context, b BubbleData, colors *UserColors, rv Reveals) (a MessageAction) {
	a.OpenMedia, a.Vote = -1, -1
	w, _ := c.Size()
	maxW := min(w*0.74, 520)
	if b.MaxWidth > 0 {
		maxW = b.MaxWidth
	}
	top := float32(2)
	if b.First {
		top = S3
	}
	row := ui.Row(c.Key(b.ID)).Padding(top, S3, 0, S3).Gap(S2).AlignItems(ui.End)
	if b.Out {
		row.Justify(ui.End)
	}
	if b.Live {
		row.Transition(popIn(b.Out))
		if b.Out {
			row.TransformOrigin(1, 1)
		} else {
			row.TransformOrigin(0, 1)
		}
	}
	if row.LongPressed() || row.RightClicked() {
		a.More = true
	}
	row.Children(func() {
		if !b.Out {
			// The avatar column: the avatar on the run's first message,
			// room on the others so the bubbles line up.
			ui.Box(c).Width(bubbleAvatar).Shrink(0).AlignSelf(ui.Start).Children(func() {
				if b.First {
					av := b.Avatar
					av.Size, av.Colors = bubbleAvatar, colors
					if av.Name == "" {
						av.Name, av.ID = b.Sender, b.SenderID
					}
					if Avatar(c, av).Clicked() && b.Profile != nil {
						a.Profile = true
					}
				}
			})
		}
		if b.Out {
			bubbleMeta(c, b)
		}
		ui.Column(c).Gap(S1).MinWidth(0).Shrink(1).AlignItems(alignFor(b.Out)).Children(func() {
			if b.First && !b.Out {
				md := MessageData{Sender: b.Sender, SenderID: b.SenderID, Profile: b.Profile, ProfileOpen: b.ProfileOpen}
				name := senderName(c, md, colors, SizeSmall).PaddingX(S1)
				if b.Profile != nil && name.Clicked() {
					a.Profile = true
				}
				profilePopover(c, name, md, colors, &a)
			}
			if b.Sticker != nil {
				MediaPreview(c, *b.Sticker, 148, 148, rv).Background(ui.Transparent)
			} else if b.Poll != nil {
				pd := *b.Poll
				if pd.Colors == nil {
					pd.Colors = colors
				}
				pa := PollCard(c, pd, time.Now(), maxW)
				a.Vote, a.EndPoll = pa.Vote, pa.End
			} else {
				bubbleBox(c, b, maxW, colors, rv, &a)
			}
			if len(b.Reactions) > 0 {
				ui.Row(c).Gap(S1).Wrap().Margin(-S2, S2, 0, S2).Children(func() {
					for _, r := range b.Reactions {
						if bubbleReaction(c, r).Clicked() {
							a.React = r.Key
						}
					}
				})
			}
		})
		if !b.Out {
			bubbleMeta(c, b)
		}
	})
	return a
}

func alignFor(out bool) ui.Align {
	if out {
		return ui.End
	}
	return ui.Start
}

// bubbleBox is the bubble itself: media at its top, a reply quote, the
// text, a link preview.
func bubbleBox(c *ui.Context, b BubbleData, maxW float32, colors *UserColors, rv Reveals, a *MessageAction) {
	p := Of(c)
	bg, fg := p.BubbleIn, p.Text
	if b.Out {
		bg, fg = p.BubbleOut, p.OnBubbleOut
	}
	tl, tr, br, bl := bubbleRadii(b.Out, b.First)
	hasText := b.Doc != nil && len(b.Doc.Children) > 0
	box := ui.Column(c).MaxWidth(maxW).Radius(tl, tr, br, bl).Background(bg).Clip()
	if !b.Out && !p.Dark {
		// A whisper of an edge, so white reads on the light wallpaper.
		box.Shadow(0, 1, 1, 0, ui.RGBA(40, 20, 90, 0.06))
	}
	if b.Failed {
		box.Background(p.Danger) // the one state that is marked
	}
	box.Children(func() {
		if len(b.Media) > 0 {
			pad := float32(3)
			if !hasText && b.Reply == nil {
				pad = 0
			}
			ui.Box(c).Padding(pad).Children(func() {
				if i := MediaGrid(c, b.Media, maxW-pad*2, rv); i >= 0 {
					a.OpenMedia = i
				}
			})
		}
		if b.File != nil {
			ui.Box(c).Padding(S1).Children(func() { FileCard(c, *b.File) })
		}
		if b.Reply != nil || hasText || b.Embed != nil {
			ui.Column(c).Padding(S2, S3, S2+1, S3).Gap(S15).Children(func() {
				if q := b.Reply; q != nil {
					qbg := p.BgInset
					if b.Out {
						qbg = ui.RGBA(30, 10, 80, 0.08)
						if p.Dark {
							qbg = ui.RGBA(0, 0, 0, 0.2)
						}
					}
					// A press goes to the message that was replied to.
					qb := ui.ButtonBase(c.Key("reply")).Gap(S2).Padding(S15, S2).Radius(RadiusSm + 2).Background(qbg).
						Justify(ui.Start).Label("Go to the message " + q.Sender + " wrote")
					qb.Children(func() {
						ui.Box(c).Width(2).Radius(1).Background(colors.Color(p, q.SenderID)).AlignSelf(ui.Stretch)
						ui.Column(c).Gap(1).MinWidth(0).Children(func() {
							ui.Row(c).Gap(S1).AlignItems(ui.Center).Children(func() {
								quoteAvatar(c, q, 14, colors)
								ui.Text(c, q.Sender).FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(colors.Color(p, q.SenderID)).SingleLine()
							})
							ui.Text(c, q.Body).FontSize(SizeSmall).TextColor(fg.Alpha(0.75)).MaxLines(2)
						})
					})
					if qb.Clicked() {
						a.JumpTo = q.ID
					}
				}
				if hasText {
					MessageBody(c, b.ID, b.Doc, rv, BodyStyle{Size: SizeBodyTouch - 0.5, Color: fg, OnAccent: b.Out})
				}
				if b.Embed != nil {
					LinkPreview(c, *b.Embed, maxW-S6)
				}
			})
		}
	})
}

// bubbleMeta is the time beside a bubble, with "Read" over it on the
// user's own messages, at the bubble's foot.
func bubbleMeta(c *ui.Context, b BubbleData) {
	p := Of(c)
	ui.Column(c).Gap(0).AlignItems(alignFor(!b.Out)).Shrink(0).Padding(0, 0, 2, 0).Children(func() {
		switch {
		case b.Failed:
			why := "Not sent"
			if b.Error != "" {
				why += ": " + b.Error
			}
			ui.Text(c, why).FontSize(SizeCaption - 1).TextColor(p.Danger).MaxWidth(160)
			return
		case b.Out && b.Read:
			ui.Text(c, "Read").FontSize(SizeCaption - 1).FontWeight(WeightMedium).TextColor(p.AccentStrong)
		case b.Out && b.Sent:
			ui.Text(c, "Sent").FontSize(SizeCaption - 1).TextColor(p.TextMuted)
		}
		ReadStack(c, b.ReadBy)
		if b.Last || len(b.Media) > 0 {
			t := b.Time
			if b.Edited {
				t = "edited · " + t
			}
			ui.Text(c, t).FontSize(SizeCaption - 1).FontFeatures("tnum").TextColor(p.TextMuted).SingleLine()
		}
	})
}

func bubbleReaction(c *ui.Context, r Reaction) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key("r:" + r.Key))
	bg, bd, fg := p.Raised, p.Wallpaper, p.TextMuted
	if r.Mine {
		bg, fg = p.AccentSubtle, p.AccentStrong
	}
	b.Height(24).PaddingX(S15).Gap(3).Radius(RadiusFull).AlignItems(ui.Center).
		Background(bg).Border(2, bd).Checked(r.Mine).Label(r.Key + " " + strconv.Itoa(r.Count))
	Pressable(b)
	b.Children(func() {
		ui.Text(c, r.Key).FontSize(13)
		if r.Count > 1 {
			ui.Text(c, strconv.Itoa(r.Count)).FontSize(SizeCaption).FontWeight(WeightSemibold).FontFeatures("tnum").TextColor(fg)
		}
	})
	return b
}

// DatePill is the day between messages, centered.
func DatePill(c *ui.Context, label string) {
	p := Of(c)
	ui.Row(c.Key("day:"+label)).Justify(ui.Center).Padding(S3, 0, S1, 0).Children(func() {
		pillText(c, label, 22, SizeCaption, WeightMedium, p.TextMuted, p.Text.Alpha(0.06)).PaddingX(S3)
	})
}

// UnreadDivider marks where unread messages begin.
func UnreadDivider(c *ui.Context, n int) {
	p := Of(c)
	label := "Unread messages"
	if n > 0 {
		label = CountText(n) + " unread messages"
	}
	ui.Row(c.Key("unread")).Padding(S3, S3, S1, S3).Gap(S2).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(p.Accent.Alpha(0.35))
		ui.Text(c, label).FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(p.AccentStrong).SingleLine()
		ui.Box(c).Height(1).Grow(1).Background(p.Accent.Alpha(0.35))
	})
}

// TypingBubble is an incoming bubble of three dots that breathe, while
// someone types.
func TypingBubble(c *ui.Context, who string, avatar AvatarSpec, colors *UserColors) {
	p := Of(c)
	ui.Row(c.Key("typing")).Padding(S3, S3, 0, S3).Gap(S2).AlignItems(ui.End).
		Transition(ui.ElementTransition{Duration: DurBase, ExitDuration: DurExit, Enter: &ui.Motion{Scale: 0.9, X: -6}, Exit: &ui.Motion{Scale: 0.9}}).
		Children(func() {
			av := avatar
			av.Size, av.Colors = bubbleAvatar, colors
			if av.Name == "" {
				av.Name = who
			}
			Avatar(c, av)
			tl, tr, br, bl := bubbleRadii(false, true)
			ui.Row(c).Height(36).PaddingX(S3+2).Gap(4).Radius(tl, tr, br, bl).Background(p.BubbleIn).AlignItems(ui.Center).
				Label(who + " is typing").Children(func() {
				for i := 0; i < 3; i++ {
					d := ui.Box(c.Key(i)).Size(7, 7).Radius(RadiusFull).Background(p.TextMuted)
					t := d.Loop("t", 1200*time.Millisecond, ui.Linear)
					v := pulse(t, float32(i)/4)
					d.Opacity(0.35+0.65*v).Translate(0, -2*v)
				}
			})
		})
}

// ChatHeader is a phone chat's bar: back (with the unread count of the
// other rooms, as LINE), the room's avatar and name with its status, call
// and more.
type ChatHeader struct {
	Title, Status string
	Avatar        AvatarSpec
	Unread        int
	Muted         bool
}

// ChatHeaderAction is what the header asked for.
type ChatHeaderAction struct {
	Back, Call, More, Info bool
}

// PhoneChatHeader builds the bar.
func PhoneChatHeader(c *ui.Context, h ChatHeader) (a ChatHeaderAction) {
	p := Of(c)
	ui.Row(c).Height(52).PaddingStart(S1).PaddingEnd(S2).Gap(S1).AlignItems(ui.Center).Shrink(0).
		Background(p.Wallpaper.Alpha(0.92)).Children(func() {
		back := ui.ButtonBase(c.Key("back")).Height(TouchTarget).PaddingX(S1).Gap(0).AlignItems(ui.Center).Label("Back")
		back.Children(func() {
			ui.Icon(c, IconChevronL).Size(24, 24).TextColor(p.Text)
			if h.Unread > 0 {
				ui.Text(c, CountText(h.Unread)).FontSize(SizeLabel).FontWeight(WeightMedium).FontFeatures("tnum").TextColor(p.Text)
			}
		})
		a.Back = back.Clicked()
		info := ui.ButtonBase(c.Key("info")).Gap(S2).AlignItems(ui.Center).Grow(1).MinWidth(0).Justify(ui.Start).Label(h.Title)
		info.Children(func() {
			av := h.Avatar
			av.Size = 34
			if av.Name == "" {
				av.Name = h.Title
			}
			Avatar(c, av)
			ui.Column(c).MinWidth(0).Gap(0).Children(func() {
				ui.Row(c).Gap(S1).AlignItems(ui.Center).Children(func() {
					ui.Text(c, h.Title).FontSize(SizeLabel + 1).FontWeight(WeightSemibold).SingleLine().Shrink(1)
					if h.Muted {
						ui.Icon(c, IconBellOff).Size(13, 13).TextColor(p.TextMuted)
					}
				})
				if h.Status != "" {
					ui.Text(c, h.Status).FontSize(SizeCaption).TextColor(p.TextMuted).SingleLine()
				}
			})
		})
		a.Info = info.Clicked()
		a.Call = IconButton(c, Ghost, IconPhone, "Call").Clicked()
		a.More = IconButton(c, Ghost, IconMore, "More").Clicked()
	})
	return a
}

// PhoneComposer is the phone's composer, as KakaoTalk's and LINE's: a +
// for attachments, a pill holding the text with the emoji button in it,
// and on the right the microphone, which turns into send once there is
// text.
func PhoneComposer(c *ui.Context, st *ComposerState, placeholder string, colors *UserColors) (r ComposerResult) {
	p := Of(c)
	ui.Column(c).Shrink(0).Background(p.Wallpaper).Children(func() {
		if q := st.Reply; q != nil {
			ui.Row(c).Gap(S2).Padding(S2, S3, 0, S4).AlignItems(ui.Center).
				Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).
				Children(func() {
					ui.Box(c).Width(2).Height(28).Radius(1).Background(colors.Color(p, q.SenderID))
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, "Reply to "+q.Sender).FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(colors.Color(p, q.SenderID)).SingleLine()
						ui.Text(c, q.Body).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
					})
					if IconButton(c, Ghost, IconX, "Cancel reply").Clicked() {
						st.Reply = nil
					}
				})
		}
		ui.Row(c).Padding(S2, S2, S2, S2).Gap(S2).AlignItems(ui.End).Children(func() {
			plusMenu(c, roundIcon(c, IconPlus, "Attach or create", ui.Transparent, p.Text), st)
			pill := ui.Row(c).Grow(1).MinWidth(0).MinHeight(40).PaddingStart(S4).PaddingEnd(S1).Gap(S1).
				Radius(20).Background(p.Raised).AlignItems(ui.End)
			if !p.Dark {
				pill.Border(1, p.Border)
			}
			pill.Children(func() {
				ta := ui.TextAreaBase(c, &st.Text).Grow(1).MinWidth(0).Lines(1, 6).Placeholder(placeholder).
					PlaceholderColor(p.TextMuted).FontSize(SizeBodyTouch).LineHeight(1.35).MarginY(10).Label(placeholder)
				ta.OnTextKey(func(k ui.TextKey) bool {
					if k.Key == ui.KeyEnter && k.Mods == 0 && !Touch(c) {
						st.send = true
						return true
					}
					return false
				})
				r.EmojiButton = roundIcon(c, IconSmile, "Emoji, GIFs and stickers", ui.Transparent, p.TextMuted)
				r.GIFButton = r.EmojiButton
				r.Emoji, r.Unified = r.EmojiButton.Clicked(), true
			})
			// Mic, or send once there is text: they swap with a quick
			// scale, as one button changing its mind.
			if hasText(st.Text) {
				s := roundIcon(c.Key("send"), IconSend, "Send", p.Accent, p.OnAccent)
				s.Transition(ui.ElementTransition{Duration: DurFast, ExitDuration: DurExit, Enter: &ui.Motion{Scale: 0.6, Rotate: -30}, Exit: &ui.Motion{Scale: 0.6}})
				if s.Clicked() {
					st.send = true
				}
			} else {
				m := roundIcon(c.Key("mic"), IconMic, "Record a voice message", ui.Transparent, p.Text)
				m.Transition(ui.ElementTransition{Duration: DurFast, ExitDuration: DurExit, Enter: &ui.Motion{Scale: 0.6}, Exit: &ui.Motion{Scale: 0.6}})
			}
		})
	})
	if st.send {
		st.send = false
		if hasText(st.Text) {
			r.Send = true
		}
	}
	st.takePlus(&r)
	return r
}

func roundIcon(c *ui.Context, icon *ui.SVG, label string, bg, fg ui.Color) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(label)).Size(40, 40).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).Label(label).Shrink(0)
	switch {
	case bg.A > 0 && b.Pressed():
		bg = p.AccentPressed
	case bg.A == 0 && b.Pressed():
		bg = p.Pressed
	case bg.A == 0 && b.Hovered():
		bg = p.Hover
	}
	b.Background(b.AnimateColor("bg", bg, Color))
	Pressable(b)
	b.Children(func() { ui.Icon(c, icon).Size(21, 21).TextColor(fg) })
	return b
}

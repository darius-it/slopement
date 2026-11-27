package kit

import (
	"strconv"
	"strings"
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
	TimeFull  string // "Sunday 7th October, 12:02": the tooltip of the time
	Body      string
	Reply     *Quote
	Thread    *ThreadChip // set on a thread's root: its replies, summed up
	Audio     *AudioData  // an audio message: its player
	Reactions []Reaction
	// First is set on the first of a group of consecutive messages from one
	// sender, which shows the avatar and the name.
	First bool
	// Highlight is set when the message mentions the user.
	Highlight bool
	// Flash is the blink of a message just jumped to (a pin, a quoted
	// reply): its color in at once; FlashOut while it fades out slowly.
	Flash, FlashOut bool
	Edited          bool
	// Unverified marks a message from a device not verified.
	Unverified bool
	// Pending is set while it is being sent: it shows as any sent message
	// does — only a failure (Failed, with its Error) is marked.
	Pending bool
	// Unsent: still sending while there is no connection. Its words are red, as
	// a failed message's, until it is delivered (then it is drawn as any other).
	Unsent bool
	Failed bool
	// CanRetry says a failed message can be sent again ("Try again" shows).
	CanRetry bool
	Error    string
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
	// Location is a shared place, a card.
	Location *LocationSpec
	// Notice (m.notice, from bots) is shown in the quiet color.
	Notice bool
	// Quick are the emoji the hover toolbar offers to react with at once; none
	// gives the default three.
	Quick []string
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
	// Editor, set while the message is being edited, draws the field in place
	// of its words (EditBox); the toolbar and menu are away meanwhile.
	Editor func()
}

// YouTubeSpec is a YouTube link's card.
type YouTubeSpec struct {
	ID, Title, Channel, Duration, Thumb string
}

// MessageAction is what the user did to a message.
type MessageAction struct {
	// At is where the message's row was in the last frame, in window DIPs: for
	// a phone's menu, which draws the message again where it is.
	At ui.Rect
	// Retry: "Try again" on a message that was not sent was pressed.
	Retry     bool
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
	// OpenThread: the replies chip under the message was pressed.
	OpenThread bool
	// Audio is what the message's player asked for.
	Audio AudioAction
}

// ThreadChip sums a thread up under its root: how many replies, who wrote
// them (their pictures), and when the last came.
type ThreadChip struct {
	Replies int
	Last    string // "12:30", "Mon"
	Avatars []AvatarSpec
}

// threadChip is the chip under a message that has a thread: a press opens
// it.
func threadChip(c *ui.Context, t ThreadChip) (pressed bool) {
	p := Of(c)
	label := "1 reply"
	if t.Replies != 1 {
		label = strconv.Itoa(t.Replies) + " replies"
	}
	b := ui.ButtonBase(c.Key("thread")).Gap(S2).Padding(S1, S2, S1, S1+2).Radius(RadiusSm).AlignSelf(ui.Start).
		AlignItems(ui.Center).Label("Open the thread: " + label)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	Pressable(b)
	b.Children(func() {
		for i, av := range t.Avatars {
			if i >= 3 {
				break
			}
			av.Size = 18
			Avatar(c, av)
		}
		ui.Text(c, label).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.AccentStrong).SingleLine()
		if t.Last != "" {
			ui.Text(c, "Last reply "+t.Last).FontSize(SizeCaption).TextColor(p.TextFaint).SingleLine()
		}
	})
	return b.Clicked()
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
	if !open || a.ProfileAct.Message || a.ProfileAct.Call || a.ProfileAct.Verify || a.ProfileAct.EditStatus || a.ProfileAct.EditNote || a.ProfileAct.Ignore || a.ProfileAct.Unignore {
		a.ProfileClosed = true
	}
}

// QuickReactions are offered in the hover toolbar when the app gives none of
// its own (MessageData.Quick: the ones used lately, then these).
var QuickReactions = []string{"👍", "❤️", "😂"}

// MessageGroupGap is the room above the first message of a group.
const MessageGroupGap = S3

// moreHalo is how far around the ⋯ button still counts as on it: a little,
// so the pointer can drift, not so much that it grabs the buttons beside.
const moreHalo = 4

// toolbarPinned is the message whose hover toolbar is held open for its ⋯
// menu (UI thread only, like the rest of the view).
var (
	toolbarPinned   string
	toolbarPinnedAt time.Time // when the pointer was last on its ⋯
)

// Message is a row of the timeline: flat, not a bubble, with the avatar
// and name on the first message of a group, the time in the gutter of
// the others while hovered, and a toolbar of actions while hovered.
func Message(c *ui.Context, m MessageData, colors *UserColors) (a MessageAction) {
	p := Of(c)
	a.OpenMedia, a.Vote = -1, -1
	row := ui.Row(c.Key(m.ID))
	// Only the pointer, not focus: a click on the text or an embed leaves
	// the keyboard focus in the row, which must not keep it highlighted.
	hov := row.Hovered()
	// The toolbar stays while its ⋯ menu is in use: the menu lives in the
	// toolbar's ⋯ button, and the pointer leaves the row to reach the menu,
	// so a toolbar that left with the hover would take the menu with it.
	// It is pinned once the pointer is on ⋯ and let go when the pointer
	// comes back to the row elsewhere, to another row, or a choice is made.
	if row.Hovered() && toolbarPinned != m.ID && toolbarPinned != "" {
		toolbarPinned = ""
	}
	if toolbarPinned == m.ID {
		hov = true
	}
	bg := ui.Transparent
	if m.Highlight {
		bg = p.AccentSubtle.Alpha(0.45)
	}
	if hov {
		bg = p.Hover.Over(bg.Over(p.Bg))
	}
	// The blink: the accent comes in fast and goes out slowly, over the
	// hover gray.
	anim := Color
	switch {
	case m.Flash:
		bg = p.Accent.Alpha(0.32).Over(p.Bg)
		anim = flashIn
	case m.FlashOut:
		anim = flashOut
	}
	top := rowPad()
	if m.First {
		top = groupGap()
	}
	row.Padding(top, S4, rowPad(), S4).Gap(S3).AlignItems(ui.Start).
		Background(row.AnimateColor("bg", bg, anim))
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
				ts := ui.Text(c, m.Time).FontSize(SizeCaption - 1).FontFeatures("tnum").TextColor(p.TextFaint).
					SingleLine().MarginY(3)
				if m.TimeFull != "" {
					ts.Tooltip(m.TimeFull)
				}
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
					ts := ui.Text(c, m.Time).FontSize(SizeCaption).FontFeatures("tnum").TextColor(p.TextMuted).SingleLine()
					if m.TimeFull != "" {
						ts.Tooltip(m.TimeFull) // the whole date, on hover
					}
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
			case m.Editor != nil:
				m.Editor()
			case m.Undecryptable:
				ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconLock).Size(14, 14).TextColor(p.TextMuted)
					ui.Text(c, "Waiting for this message. It will appear when the keys arrive.").
						FontSize(SizeBody).Italic().TextColor(p.TextMuted)
				})
			default:
				fg := p.Text
				if m.Failed || m.Unsent {
					fg = p.Danger
				}
				// A right click on the words: the message's menu, with Copy and
				// Select All under it (the text's own menu would hide it).
				textMenu := func(mm *ui.Menu) {
					if !m.Menu.Off {
						a.chose(MessageMenu(mm, m.Menu))
						mm.Separator()
					}
					mm.EditItems()
				}
				if m.Notice {
					fg = p.TextMuted // a bot's notice is quieter than a person's words
				}
				body := func() {
					if m.Doc != nil {
						// Selected with the rest of the timeline (the list is Selectable).
						MessageBody(c, m.ID, m.Doc, m.Reveals, BodyStyle{Color: fg, Shared: true, Menu: textMenu})
					} else if m.Body != "" {
						ContextMenu(c, ui.RichText(c, ui.Span{Text: m.Body, Color: fg}).FontSize(SizeBody).LineHeight(1.45), textMenu)
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
						why = m.Error // "Not sent: …" already, as the source words it
						if !strings.HasPrefix(why, "Not sent") {
							why = "Not sent: " + why
						}
					}
					ui.Row(c.Key("notsent:" + m.ID)).Gap(S2).AlignItems(ui.Center).Children(func() {
						ui.Text(c, why).FontSize(SizeCaption).TextColor(p.Danger)
						if m.CanRetry && Button(c.Key("retry:"+m.ID), Ghost, "Try again", IconRetry).Clicked() {
							a.Retry = true
						}
					})
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
				if m.Audio != nil {
					a.Audio = AudioCard(c, *m.Audio)
				}
				if m.File != nil && FileCard(c, *m.File).Clicked() {
					a.Download = true
				}
				if m.Location != nil {
					LocationCard(c, *m.Location)
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
			if m.Thread != nil && m.Thread.Replies > 0 && threadChip(c, *m.Thread) {
				a.OpenThread = true
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
		if hov && m.Editor == nil {
			ui.Row(c).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(-12).Left(-S4).
				Padding(2).Gap(1).Radius(RadiusSm+2).Background(p.Raised).Border(1, p.Border).
				Shadow(0, 2, 6, -2, p.Shadow).Transition(ui.ElementTransition{Duration: DurFast, Enter: &ui.Motion{}}).
				Children(func() {
					quick := m.Quick
					if len(quick) == 0 {
						quick = QuickReactions
					}
					for _, k := range quick {
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
					// Your own words: Edit beside Reply; with Shift held, Delete too.
					if !m.Menu.Off && m.Menu.Mine {
						if m.Menu.HasText && toolButton(c, "Edit", func(fg ui.Color) { ui.Icon(c, IconPencil).Size(16, 16).TextColor(fg) }).Clicked() {
							a.chose(MsgEdit)
						}
						if c.Modifiers()&ui.Shift != 0 &&
							toolButton(c, "Delete", func(ui.Color) { ui.Icon(c, IconTrash).Size(16, 16).TextColor(p.Danger) }).Clicked() {
							a.chose(MsgDelete)
						}
					}
					if !m.Menu.Off {
						// A halo of a few pixels around ⋯ counts as being on it, so a
						// pointer that drifts a little off the button on its way to
						// the menu does not lose the toolbar (and the menu with it).
						// Negative margin keeps the toolbar's layout as it was.
						halo := ui.Row(c.Key("more-halo")).Padding(moreHalo).Margin(-moreHalo)
						onMore := halo.Hovered()
						halo.Children(func() {
							more := toolButton(c, "More", func(fg ui.Color) { ui.Icon(c, IconMore).Size(16, 16).TextColor(fg) })
							MenuButton(c, more, func(mm *ui.Menu) {
								ch := MessageMenu(mm, m.Menu)
								if ch != MsgNone {
									toolbarPinned = ""
								}
								a.chose(ch)
							})
							onMore = onMore || more.Hovered()
						})
						if onMore {
							toolbarPinned, toolbarPinnedAt = m.ID, time.Now()
						} else if row.Hovered() && toolbarPinned == m.ID && time.Since(toolbarPinnedAt) > 2*time.Second {
							// Back on the row, well after leaving ⋯: no menu is
							// being reached for (crossing the row to the menu
							// takes a moment, and must not let go).
							toolbarPinned = ""
						}
					}
				})
		}
	})
	if !m.Menu.Off && m.Editor == nil {
		ContextMenu(c, row,func(mm *ui.Menu) { a.chose(MessageMenu(mm, m.Menu)) })
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

// reactionKey draws what a reaction is: its emoji, or — when the key is an
// mxc:// URI, a custom emoji — its picture (nothing yet while it loads).
func reactionKey(c *ui.Context, key string) {
	if strings.HasPrefix(key, "mxc://") {
		if EmojiImage != nil {
			if src := EmojiImage(key); src != nil {
				ui.Image(c, src).Size(16, 16).Fit(ui.Contain)
				return
			}
		}
		ui.Box(c).Size(16, 16) // its place, until the picture is there
		return
	}
	ui.Text(c, key).FontSize(13)
}

// reactionLabel is what a screen reader says of a reaction.
func reactionLabel(r Reaction) string {
	key := r.Key
	if strings.HasPrefix(key, "mxc://") {
		key = "Custom emoji"
	}
	return key + " " + strconv.Itoa(r.Count)
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
		Label(reactionLabel(r))
	Pressable(b)
	b.Children(func() {
		reactionKey(c, r.Key)
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

// CallEvent is a quiet line for a call's start or end: a phone, what happened
// (and for how long) and when.
func CallEvent(c *ui.Context, text, at string) {
	p := Of(c)
	ui.Row(c).Padding(S05, S4, S05, S4+36+S3).Gap(S1).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, IconPhone).Size(12, 12).TextColor(p.TextMuted)
		ui.Text(c, text).FontSize(SizeSmall).TextColor(p.TextMuted)
		if at != "" {
			ui.Text(c, "· "+at).FontSize(SizeSmall).TextColor(p.TextFaint)
		}
	})
}

// BrokenEvent is the tile of a message that could not be shown: a quiet line
// and, where the source can show it, a View source for the bug report. It
// reports a press on that.
func BrokenEvent(c *ui.Context, id string, canViewSource bool) (viewSource bool) {
	p := Of(c)
	ui.Row(c.Key("broken:"+id)).Padding(S1, S4, S1, S4+36+S3).Gap(S2).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, IconAlert).Size(14, 14).TextColor(p.TextMuted)
		ui.Text(c, "Couldn't show this message").FontSize(SizeSmall).TextColor(p.TextMuted)
		if canViewSource {
			b := ui.ButtonBase(c.Key("broken-src")).Height(22).PaddingX(S2).Radius(RadiusSm).AlignItems(ui.Center).Label("View source")
			if b.Hovered() {
				b.Background(p.Hover)
			}
			b.Children(func() { ui.Text(c, "View source").FontSize(SizeSmall).TextColor(p.AccentStrong) })
			viewSource = b.Clicked()
		}
	})
	return viewSource
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

// SeekingPill floats on the bottom edge of a timeline, where Jump to
// latest sits and in its look, while the app looks for a message far back
// in the history (a jump to an old pin): what it is doing, and a thin
// square line along its bottom that sweeps across, left to right, again
// and again — always forward, as Windows' busy line does. With less motion
// asked for, the line pulses in place instead. It takes no room; the app
// removes it once the message is found, and the loop's frames go with it.
func SeekingPill(c *ui.Context, text string) {
	p := Of(c)
	pill := ui.Row(c.Key("seeking")).Attach(ui.AnchorBottom, ui.AnchorBottom).Top(-S1).PassThrough().
		Height(controlHeight(c)).PaddingX(S3+S05).AlignItems(ui.Center).Radius(RadiusSm).Clip().
		Background(p.Raised).Border(1, p.BorderStrong).Transition(PopIn()).
		Label(text).Role(ui.RoleProgress)
	// Fast through the middle, easing at the ends, then a short rest off
	// the edge before the next sweep.
	t := pill.Loop("sweep", 1400*time.Millisecond, ui.Linear)
	still := c.Preferences().ReduceMotion
	line := p.Accent
	pill.Draw(func(pt *ui.Painter, r ui.Rect) {
		// Inside the border and its rounded corners: Clip on the element
		// clips its children, not its own drawing, so the line is clipped
		// here, to the inner shape.
		in := ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
		y := in.Y + in.H - 2
		pt.Clip(in, RadiusSm-1, func() {
			if still {
				pt.Fill(ui.Rect{X: in.X, Y: y, W: in.W, H: 2}, line.Alpha(0.3+0.6*ui.EaseInOut(1-abs32(2*t-1))), 0)
				return
			}
			w := in.W * 0.4
			x := in.X - w + (in.W+w)*ui.EaseInOut(min(t/0.85, 1))
			pt.Fill(ui.Rect{X: x, Y: y, W: w, H: 2}, line, 0)
		})
	})
	pill.Children(func() {
		ui.Text(c, text).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(p.TextMuted).SingleLine()
	})
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
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

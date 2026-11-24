package app

import (
	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// A phone's message menu, as Signal's: holding a message blurs the chat behind
// it, the message stays where it was (lifted, sharp), and its options extend
// downward from it in a card, on the side the message is on — in place of a
// sheet from the bottom that covers the screen and hides what it is about. If
// the card does not fit under the message, both move up together.
//
// The chat is blurred by phoneRoom (it asks msgPop.Open); the message is drawn
// again here by bubbleRowAs, at the row's own place, which its row reports each
// frame (msgPop.At), so it stays over the original as the layout moves (the
// keyboard going away, say).

// msgPopState is the open menu: whether it is, and where its message is.
type msgPopState struct {
	Open bool
	At   ui.Rect // the message's row in window DIPs, as of the last frame
}

const (
	msgMenuW      = 248
	msgMenuRowH   = 48
	msgMenuGap    = 8  // between the message and its card
	msgMenuMargin = 12 // kept clear of the screen's edges and the bars
)

// msgItem is one option of a message's menu.
type msgItem struct {
	icon   *ui.SVG
	title  string
	choice kit.MessageChoice
	danger bool
}

// messageItems are what the message's menu offers: what msgMenu says it can do.
func (a *App) messageItems(ev model.Event) []msgItem {
	st := a.msgMenu(ev)
	items := []msgItem{{kit.IconReply, "Reply", kit.MsgReply, false}}
	add := func(icon *ui.SVG, title string, ch kit.MessageChoice) {
		items = append(items, msgItem{icon, title, ch, false})
	}
	if st.HasText {
		add(kit.IconCopy, "Copy text", kit.MsgCopyText)
	}
	add(kit.IconLink, "Copy message link", kit.MsgCopyLink)
	if st.Sticker {
		add(kit.IconPlus, "Add to my emoji", kit.MsgAddToPack)
	}
	if st.Pinned {
		add(kit.IconPin, "Unpin", kit.MsgUnpin)
	} else {
		add(kit.IconPin, "Pin", kit.MsgPin)
	}
	if st.Forward {
		add(kit.IconForward, "Forward", kit.MsgForward)
	}
	if st.Source {
		add(kit.IconCode, "View source", kit.MsgViewSource)
	}
	if st.Mine {
		if st.HasText {
			add(kit.IconPencil, "Edit", kit.MsgEdit)
		}
		items = append(items, msgItem{kit.IconTrash, "Delete", kit.MsgDelete, true})
	}
	return items
}

// messageMenuChoice carries out an option. Reply is the same as the swipe's.
func (a *App) messageMenuChoice(ev model.Event, ch kit.MessageChoice) {
	if ch == kit.MsgReply {
		a.messageAct(a.msgRoom, ev, nil, kit.MessageAction{Reply: true, OpenMedia: -1, Vote: -1})
		return
	}
	a.messageChoice(a.msgRoom, ev, ch)
}

// msgMenuLayout is where the message and its card go: the message where it
// is, the card under it; both up together when the card would not fit above
// the keyboard or the home indicator, and a message too tall for the screen
// is cut short to leave room for its card.
func msgMenuLayout(at ui.Rect, menuH, screenH, safeTop, safeBottom float32) (top, bubbleH, menuTop float32) {
	topLimit := safeTop + msgMenuMargin
	botLimit := screenH - safeBottom - msgMenuMargin
	bubbleH = max(min(at.H, botLimit-topLimit-menuH-msgMenuGap), 0)
	top = at.Y
	if top+bubbleH+msgMenuGap+menuH > botLimit {
		top = botLimit - menuH - msgMenuGap - bubbleH
	}
	top = max(top, topLimit)
	return top, bubbleH, top + bubbleH + msgMenuGap
}

// messageMenuView draws the open menu over the blurred chat.
func (a *App) messageMenuView(c *ui.Context) {
	if !a.msgPop.Open || a.msgID == "" {
		return
	}
	var ev model.Event
	found := false
	for _, e := range a.src.Timeline(a.msgRoom) {
		if e.ID == a.msgID {
			ev, found = e, true
		}
	}
	r, roomOK := a.roomByID(a.msgRoom)
	if !found || !roomOK {
		a.msgPop.Open = false
		return
	}
	items := a.messageItems(ev)
	p := kit.Of(c)
	w, h := c.Size()
	safeTop, _, safeBottom, _ := c.SafeArea()
	out := ev.Sender == a.src.Me().ID

	at := a.msgPop.At
	if at.W <= 0 || at.H <= 0 { // its place is not known yet: the middle, until it is
		at = ui.Rect{X: 0, Y: h * 0.35, W: w, H: 56}
	}
	menuH := float32(len(items)*msgMenuRowH + len(items) - 1)
	top, bubbleH, menuTop := msgMenuLayout(at, menuH, h, safeTop, safeBottom)

	// The card is on the message's side: under its left edge for a message on
	// the left (past its avatar), under its right edge for the user's own.
	menuLeft := at.X + kit.S3 + 40
	origin := float32(0)
	if out {
		menuLeft = w - kit.S3 - msgMenuW
		origin = 1
	}
	menuLeft = min(max(menuLeft, kit.S3), w-kit.S3-msgMenuW)

	ui.DialogBase(c, &a.msgPop.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Scrim).Padding(0).
			Transition(ui.ElementTransition{Duration: kit.DurBase, ExitDuration: kit.DurExit, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
		panel.Size(w, h).Label("Message options")

		// A press anywhere else closes it.
		if ui.Box(c.Key("dismiss")).Absolute().Top(0).Left(0).Size(w, h).Clicked() {
			a.msgPop.Open = false
		}

		// The message, where it was, sharp over the blur.
		ui.Column(c.Key("lift")).Absolute().Top(top).Left(at.X).Width(at.W).MaxHeight(bubbleH).Clip().Children(func() {
			rows := a.rows(r.ID)
			for i := range rows {
				if rows[i].ev != nil && rows[i].ev.ID == a.msgID {
					a.bubbleRowAs(c, r, rows, i, true)
					break
				}
			}
		})

		// Its options, extending down from it.
		card := ui.Column(c.Key("options")).Absolute().Top(menuTop).Left(menuLeft).Width(msgMenuW).
			Radius(kit.RadiusLg).Background(p.Raised).Border(1, p.Border).Shadow(0, 12, 32, -6, p.Shadow).Clip().
			TransformOrigin(origin, 0).
			Transition(ui.ElementTransition{Duration: kit.DurFast, Ease: kit.EaseOut, ExitDuration: kit.DurExit,
				Enter: &ui.Motion{Scale: 0.9}, Exit: &ui.Motion{Scale: 0.95}})
		card.Children(func() {
			for i, it := range items {
				if i > 0 {
					ui.Box(c).Height(1).Background(p.Border)
				}
				fg := p.Text
				if it.danger {
					fg = p.Danger
				}
				b := ui.ButtonBase(c.Key("opt:" + it.title)).Height(msgMenuRowH).PaddingX(kit.S4).Gap(kit.S3).
					AlignItems(ui.Center).AlignSelf(ui.Stretch).Label(it.title)
				if b.Pressed() {
					b.Background(p.Pressed)
				}
				b.Children(func() {
					ui.Text(c, it.title).FontSize(kit.SizeBodyTouch).TextColor(fg).Grow(1).SingleLine()
					ui.Icon(c, it.icon).Size(18, 18).TextColor(fg)
				})
				if b.Clicked() {
					a.msgPop.Open = false
					a.messageMenuChoice(ev, it.choice)
				}
			}
		})
	})
}

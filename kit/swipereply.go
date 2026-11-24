package kit

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Swipe to reply, as iMessage, WhatsApp and Telegram have it: a finger drags
// a message sideways a little way, an arrow comes out beside it, a tick says
// "that far", and letting go from there replies; letting go before springs
// the message back with nothing done. An incoming message (on the left) is
// pulled to the right and an outgoing one (on the right) to the left, toward
// the middle of the screen, the way it has room to go. Only on a phone.
//
// It is a horizontal Draggable, which mygo's touch handling leaves the
// vertical scrolling of the list alongside: a finger going down scrolls the
// list, one going across drags the message.

const (
	// swipeReach is how far the message can be pulled; past it, it resists.
	swipeReach = 76
	// swipeFire is how far it must be pulled for letting go to reply.
	swipeFire = 56
	// swipeHyst is how far back the finger must come to take that back.
	swipeHyst = 12
)

// swipeBack is how the message returns: a quick spring with a hint of
// bounce, from the speed the finger let go at.
var swipeBack = ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.12}}

// replySwipe is one bubble's swipe, kept in its row.
type replySwipe struct {
	armed    bool // pulled past swipeFire: letting go replies
	dragging bool
}

type replySwipeKey struct{}

// swipeState is the swipe kept in row.
func swipeState(row ui.Element) *replySwipe {
	return ui.Local(row, replySwipeKey{}, func() replySwipe { return replySwipe{} })
}

// swipeToReply lets a finger pull col (the message with its name and
// reactions) sideways, and sets a.Reply when it is let go from past
// swipeFire. It returns how far the message is pulled, for the arrow beside
// it. Call it before col's children.
func swipeToReply(row, col ui.Element, out bool, a *MessageAction) float32 {
	st := swipeState(row)
	bounds := ui.Rect{W: swipeReach} // incoming: to the right
	if out {
		bounds = ui.Rect{X: -swipeReach, W: swipeReach} // outgoing: to the left
	}
	col.Draggable(nil, ui.DraggableOptions{
		Axis:      ui.Horizontal,
		Bounds:    &bounds,
		Snap:      func(ui.Point) ui.Point { return ui.Point{} }, // always back
		Animation: swipeBack,
	})
	x := float32(math.Abs(float64(col.DragOffset().X)))
	st.dragging = col.Dragging()
	switch {
	case st.dragging && x >= swipeFire && !st.armed:
		st.armed = true
		mygo.Haptics.Light() // "that far": the tick of a choice
	case st.dragging && x < swipeFire-swipeHyst && st.armed:
		st.armed = false
	}
	if _, _, ended := col.DragEnded(); ended {
		if st.armed {
			a.Reply = true
		}
		st.armed = false
	}
	return x
}

// replyCue is the arrow beside a message being pulled: it comes out on the
// side the message left, growing and fading in as it goes, and fills with the
// accent once it is far enough to reply.
func replyCue(c *ui.Context, row ui.Element, out bool, x float32) {
	if x < 4 {
		return
	}
	p := Of(c)
	armed := swipeState(row).armed
	// Fully there by swipeFire.
	t := min(max((x-8)/(swipeFire-8), 0), 1)
	bg, fg, border := p.BgInset, p.TextMuted, p.Border
	scale := 0.6 + 0.4*t
	if armed {
		bg, fg, border = p.Accent, p.OnAccent, p.Accent
		scale = 1.1
	}
	cue := ui.Box(c).Size(30, 30).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
		Background(bg).Border(1, border).Opacity(t).Scale(scale, scale).PassThrough()
	if out { // beyond the message's right edge, which has moved left
		cue.Attach(ui.AnchorRight, ui.AnchorLeft).Left(S2)
	} else {
		cue.Attach(ui.AnchorLeft, ui.AnchorRight).Left(-S2)
	}
	cue.Children(func() { ui.Icon(c, IconReply).Size(16, 16).TextColor(fg) })
}

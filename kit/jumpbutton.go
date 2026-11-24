package kit

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// jumpTransition is how the button comes and goes: it fades in from a little
// smaller (never from nothing) and leaves quicker than it came.
var jumpTransition = ui.ElementTransition{
	Duration:     DurFast,
	Ease:         EaseOut,
	ExitDuration: 100 * time.Millisecond,
	Enter:        &ui.Motion{Scale: 0.85},
	Exit:         &ui.Motion{Scale: 0.9},
}

// JumpToLatest is the round arrow that takes a phone's timeline back to the
// newest message: small, on the bottom right edge of the messages, just above
// the composer, as Perplexity's, Vivid's and Grok's chats have it (Mobbin). It
// floats over the messages, so it is the one thing here with a soft shadow.
// Its box is a fingertip's (44 DIPs); the disc drawn in it is smaller. Call it
// inside the column holding the timeline, after the list; it takes no room.
//
// unread is how many messages came while it was scrolled away (the room's own
// count, which only goes down once the end is in view), shown as a solid count
// on the arrow's top right; mentions is how many of them mention the user, and
// colors it with the accent.
func JumpToLatest(c *ui.Context, unread, mentions int) (clicked bool) {
	p := Of(c)
	label := "Jump to latest message"
	if unread > 0 {
		label += ", " + strconv.Itoa(unread) + " unread"
	}
	b := ui.ButtonBase(c.Key("jump-latest")).Size(TouchTarget, TouchTarget).AlignItems(ui.Center).Justify(ui.Center).
		Attach(ui.AnchorBottomRight, ui.AnchorBottomRight).Top(-S2).Left(-S2).
		Label(label).Transition(jumpTransition)
	bg := p.Raised
	if b.Pressed() {
		bg = p.Pressed.Over(p.Raised)
	}
	b.Children(func() {
		d := ui.Box(c).Size(36, 36).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
			Background(bg).Border(1, p.BorderStrong).Shadow(0, 4, 14, -4, p.Shadow).PassThrough()
		d.Children(func() { ui.Icon(c, IconArrowDown).Size(18, 18).TextColor(p.Text) })
		if unread > 0 {
			// On the disc's top right corner, ringed in the page's color.
			ui.Box(c).Attach(ui.AnchorTopRight, ui.AnchorCenter).Top(4).Left(-4).PassThrough().Children(func() {
				n, hot := unread, false
				if mentions > 0 {
					n, hot = mentions, true
				}
				SolidCount(c, n, hot, p.Wallpaper)
			})
		}
	})
	Pressable(b)
	return b.Clicked()
}

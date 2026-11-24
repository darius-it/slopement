package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Motion follows a few rules (see docs/design-language.md):
//   - only when it explains a change; never on keyboard-driven,
//     high-frequency UI (the Ctrl+K palette, switching rooms, list arrows);
//   - ease-out on enter, faster exits, 100–250 ms;
//   - nothing grows from nothing: enter from scale 0.97 + fade;
//   - popovers come from their trigger, dialogs stay centered.
// mygo already sends values to their targets at once under the desktop's
// Reduce Motion, keeping fades, which is what we want.

// Durations.
const (
	DurPress = 120 * time.Millisecond
	DurFast  = 150 * time.Millisecond
	DurBase  = 200 * time.Millisecond
	DurExit  = 100 * time.Millisecond
)

// EaseOut is a strong ease-out for UI that enters or responds.
var EaseOut = ui.CubicBezier(0.23, 1, 0.32, 1)

// EaseInOut is for things that move on screen.
var EaseInOut = ui.CubicBezier(0.77, 0, 0.175, 1)

// EaseDrawer is iOS's sheet curve.
var EaseDrawer = ui.CubicBezier(0.32, 0.72, 0, 1)

// Color is how hover and press colors change.
var Color = ui.Animation{Duration: DurPress, Ease: ui.EaseOut}

// The blink of a message jumped to: in at once, out slowly.
var (
	flashIn  = ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut}
	flashOut = ui.Animation{Duration: 1100 * time.Millisecond, Ease: ui.EaseOut}
)

// SheetSpring settles a sheet without bouncing; it keeps the finger's
// velocity when let go.
var SheetSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 350 * time.Millisecond}}

// PopIn is the transition of a popover or menu panel: it comes 4 DIPs from
// its trigger, from 0.97 of its size, fading in, and leaves faster.
func PopIn() ui.ElementTransition {
	return ui.ElementTransition{
		Duration:     DurFast,
		Ease:         EaseOut,
		ExitDuration: DurExit,
		Enter:        &ui.Motion{Y: -4, Scale: 0.97, Flip: true},
		Exit:         &ui.Motion{Y: -2, Scale: 0.98, Flip: true},
	}
}

// FadeIn is the transition of a dialog: centered, a little scale, a fade.
func FadeIn() ui.ElementTransition {
	return ui.ElementTransition{
		Duration:     DurBase,
		Ease:         EaseOut,
		ExitDuration: DurExit,
		Enter:        &ui.Motion{Scale: 0.97},
		Exit:         &ui.Motion{Scale: 0.98},
	}
}

// PushSpring is a phone's page push, for its router: a lively spring (a
// little bounce: the page overshoots its place by a few DIPs and settles back)
// that is mostly there in a third of a second. A spring, not a curve, so a push
// caught by a Back (or a Back caught by a push) turns around from where the
// page is, with the speed it had, instead of restarting, and a swipe let go
// hands its speed to it. Pushed and popped by the same spring. Keep the
// bounce small: the router lays the page by how far it has come, so a big
// overshoot shows the page under it at the edge.
var PushSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond, Bounce: 0.2}}

// Rise is how a message that arrives live comes in: 8 DIPs up, fading.
// Never used for history loading or the first build of a room.
func Rise() ui.ElementTransition {
	return ui.ElementTransition{
		Duration: 180 * time.Millisecond,
		Ease:     EaseOut,
		Enter:    &ui.Motion{Y: 8},
	}
}

// Pressable gives e the kit's press feedback: a 0.97 scale while pressed.
// Only for buttons, not rows (rows change color instead).
func Pressable(e ui.Element) ui.Element {
	s := float32(1)
	if e.Pressed() {
		s = 0.97
	}
	s = e.AnimateWith("press", s, DurPress, EaseOut)
	return e.Scale(s, s)
}

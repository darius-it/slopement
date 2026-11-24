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

// PushSpring is a phone's page push, for its router: a spring, so a push caught
// by a Back (or a Back caught by a push) turns around from where the page is,
// with the speed it had, instead of restarting. It has no bounce, and must not:
// the router holds a page's progress between 0 and 1, so a spring that goes
// past its end shows nothing of it, and only keeps the transition alive for a
// second after it looks over, during which the router builds both pages, the
// room's and the lists', in every frame (a bounce made frames drop for the whole
// tail). Without it the spring is mostly there in a third of a second and done
// within about half of one.
var PushSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 330 * time.Millisecond}}

// PopCurve is a phone's page pop: the page goes back, by a Back button or a
// finger's swipe let go past halfway. A curve of fixed length, not a spring:
// it ends when it should, with no tail, whatever the speed or the distance it
// was let go at (a swipe hands the router only where it let go, not how fast, so
// a spring from there starts from rest, stalls and creeps), and it comes in
// strong and eases out as UIKit's pop does.
var PopCurve = ui.Animation{Duration: 300 * time.Millisecond, Ease: EaseDrawer}

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

package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// SheetState is a bottom sheet's (or adaptive popover's) state, kept in
// the app's model. Open is what the app sets; the sheet stays built while
// it springs shut.
type SheetState struct {
	Open bool
	// alive: the sheet is built (open, or on its way shut); fresh: it has
	// just started opening, and starts below the window.
	alive, fresh bool
	dragging     bool    // a finger holds the sheet
	drag         float32 // where the finger has put it (0 = open)
	y            float32 // where it was drawn last frame
	h            float32 // the panel's height last frame
}

// The sheet's springs, after iOS's Control Center. They are springs, so
// they keep their velocity when the target changes: a sheet closed while
// it opens turns around smoothly, a sheet caught mid-flight stays under the
// finger, and a flick hands its speed over.
var (
	// sheetOpen flies the sheet in: quick, with a visible bounce.
	sheetOpen = ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.22}
	// sheetBack returns a dragged sheet to open: a lively bounce, like a
	// let-go Control Center.
	sheetBack = ui.SpringOptions{Duration: 340 * time.Millisecond, Bounce: 0.28}
	// sheetShut closes it: faster than it came, and no bounce.
	sheetShut = ui.SpringOptions{Duration: 240 * time.Millisecond}
)

// sheetProjection is how far ahead (in seconds of velocity) a released
// sheet is projected, as UIKit's normal deceleration (0.998/ms ≈ v·0.5 s);
// a bit less, so a gentle let-go doesn't close it.
const sheetProjection = 0.3

// rubberBand eases a drag past the open position: the further, the less it
// moves (UIScrollView's curve, c = 0.55), never a hard stop.
func rubberBand(over, dim float32) float32 {
	if dim <= 0 {
		return 0
	}
	return (1 - 1/(over*0.55/dim+1)) * dim
}

// SheetAlive says a sheet is still built: open, or on its way shut, so its
// owner keeps calling Sheet until it is gone.
func SheetAlive(st *SheetState) bool { return st.alive }

// SheetTitle is a sheet's or dialog's title row, with a close button.
func SheetTitle(c *ui.Context, title string, open *bool) {
	p := Of(c)
	ui.Row(c).Gap(S2).AlignItems(ui.Center).MinHeight(36).Children(func() {
		ui.Text(c, title).FontSize(SizeTitle - 1).FontWeight(WeightSemibold).TextColor(p.Text).Grow(1).Role(ui.RoleHeading)
		if IconButton(c, Ghost, IconX, "Close").Clicked() {
			*open = false
		}
	})
}

// Sheet shows fn in a sheet from the bottom of the window over a scrim,
// as phones do. One spring moves it, in DIPs from its resting place: up
// as it opens, down as it closes, after the finger while its grabber is
// dragged (with friction upwards), and on with the finger's speed when let
// go; a flick down (over 500 DIPs a second) or a drag past a third of it
// closes it. Every change retargets the same spring, so the sheet can be
// caught and turned around at any moment. The scrim fades with it. A tap
// on the scrim or Escape closes it too.
func Sheet(c *ui.Context, st *SheetState, title string, fn func()) {
	if st.Open && !st.alive {
		st.alive, st.fresh = true, true
	}
	if !st.alive {
		return
	}
	p := Of(c)
	keep := true
	ui.DialogBase(c, &keep, func(backdrop, panel ui.Element) {
		h := st.h
		if h <= 0 {
			_, h = c.Size() // before its first layout: below the window
		}
		shut := h + S6
		backdrop.Justify(ui.End).AlignItems(ui.Stretch)
		panel.Radius(RadiusLg+4, RadiusLg+4, 0, 0).Background(p.Raised).Padding(0, S4, S6, S4).Gap(S3).
			MaxHeightPercent(88).AlignSelf(ui.Stretch).Shadow(0, -8, 32, -12, p.Shadow)
		grab := ui.Box(c).Height(24).AlignSelf(ui.Stretch).AlignItems(ui.Center).Justify(ui.Center).
			Cursor(ui.CursorGrab).Label("Drag down to close").Children(func() {
			ui.Box(c).Size(36, 4).Radius(RadiusFull).Background(p.BorderStrong)
		})
		if st.fresh {
			// Start below the window, so the spring brings it up.
			panel.Jump("y", shut)
			st.fresh, st.drag, st.y, st.dragging = false, 0, shut, false
		}
		// The whole sheet drags, not just its grabber (a click on a button
		// in it moves nothing, so it stays a click).
		_, dyG, okG := grab.Dragged()
		_, dyP, okP := panel.Dragged()
		held := grab.Dragging() || panel.Dragging() || okG || okP
		var y float32
		switch {
		case held:
			if !st.dragging {
				// Caught: wherever it is, even mid-spring, it stays under
				// the finger from there.
				st.dragging, st.drag = true, st.y
			}
			if okG {
				st.drag += dyG
			} else if okP {
				st.drag += dyP
			}
			d := st.drag
			if d < 0 {
				d = -rubberBand(-d, h) // past open: resists, never a wall
			}
			y = panel.Jump("y", d)
		default:
			opts := sheetOpen
			released := false
			vy := float32(0)
			if _, v, ok := grab.DragEnded(); ok {
				vy, released = v, true
			} else if _, v, ok := panel.DragEnded(); ok {
				vy, released = v, true
			} else if st.dragging {
				released = true // let go without a fling
			}
			if released {
				st.dragging = false
				// Where the sheet would come to rest with this momentum:
				// past half way it closes, however slowly it was dragged;
				// a flick down closes it from anywhere, one up keeps it.
				if st.y+vy*sheetProjection > h/2 {
					st.Open = false
				}
				opts = sheetBack
				opts.Velocity = vy // the fling carries on
			}
			target := float32(0)
			if !st.Open {
				target = shut
				v := opts.Velocity
				opts = sheetShut
				opts.Velocity = v
			}
			y = panel.Spring("y", target, opts)
			if !st.Open && panel.Arrived("y") {
				st.alive = false // shut: gone next frame
			}
		}
		st.y = y
		panel.Translate(0, y)
		// The scrim follows the sheet's way up.
		shown := min(max(1-y/shut, 0), 1)
		backdrop.Background(p.Scrim.Alpha(float32(p.Scrim.A) / 255 * shown))
		if title != "" {
			SheetTitle(c, title, &st.Open)
		}
		fn()
		if b := panel.Bounds(); b.H > 0 {
			st.h = b.H
		}
	})
	if !keep {
		st.Open = false // the scrim or Escape: spring shut, don't vanish
	}
}

// Dialog shows fn in a dialog centered over a scrim. Dialogs stay centered
// and scale from 0.97, without moving. The title takes the width; the
// close button hugs the corner, 12 DIPs in, out of the content's padding
// (Relume, Grok on Mobbin), so it neither pushes the title nor floats
// inside the panel.
func Dialog(c *ui.Context, open *bool, title string, width float32, fn func()) {
	p := Of(c)
	if width == 0 {
		width = 400
	}
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Scrim).Padding(S4).
			Transition(ui.ElementTransition{Duration: DurBase, ExitDuration: DurExit, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
		panel.Width(width).MaxWidthPercent(100).Padding(S5).Gap(S5).Radius(RadiusLg).Background(p.Raised).
			Border(1, p.Border).Shadow(0, 16, 48, -8, p.Shadow).Transition(FadeIn())
		if title != "" {
			ui.Text(c, title).FontSize(SizeLabel + 1).FontWeight(WeightSemibold).TextColor(p.Text).
				Role(ui.RoleHeading).PaddingEnd(S6) // clear of the close button
		}
		fn()
		closeCorner(c, open)
	})
}

// closeCorner is a dialog's close button in its top-right corner.
func closeCorner(c *ui.Context, open *bool) {
	p := Of(c)
	x := ui.ButtonBase(c.Key("close")).Size(28, 28).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).
		Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(S3).Left(-S3).Label("Close")
	fg := p.TextMuted
	if x.Pressed() {
		x.Background(p.Pressed)
		fg = p.Text
	} else if x.Hovered() {
		x.Background(p.Hover)
		fg = p.Text
	}
	x.Children(func() { ui.Icon(c, IconX).Size(16, 16).TextColor(fg) })
	if x.Clicked() {
		*open = false
	}
}

// ConfirmSpec is a question to confirm an action.
type ConfirmSpec struct {
	Title, Body string
	// Confirm labels the action ("Leave"); Cancel defaults to "Cancel".
	Confirm, Cancel string
	// Danger colors the action for something that loses data or access.
	Danger bool
}

// Confirm asks a question and reports the answer once given: confirmed
// true, or false when cancelled (by its button, Escape or the scrim).
// Desktop: a small centered dialog, title and body close together, the
// buttons at its bottom right, the action last. Phones: a sheet whose
// full-width buttons stack, the action on top and Cancel under it
// (Bluesky, Revolut on Mobbin).
func Confirm(c *ui.Context, st *SheetState, s ConfirmSpec) (answered, confirmed bool) {
	p := Of(c)
	if s.Cancel == "" {
		s.Cancel = "Cancel"
	}
	v := Primary
	if s.Danger {
		v = Danger
	}
	wasOpen := st.Open
	body := func() {
		ui.Column(c).Gap(S15).Children(func() {
			ui.Text(c, s.Title).FontSize(SizeLabel + 1).FontWeight(WeightSemibold).TextColor(p.Text).Role(ui.RoleHeading).
				PaddingEnd(S6)
			if s.Body != "" {
				ui.Text(c, s.Body).FontSize(SizeBody).LineHeight(1.45).TextColor(p.TextMuted)
			}
		})
	}
	if Compact(c) {
		Sheet(c, st, "", func() {
			body()
			ui.Column(c).Gap(S2).Margin(S2, 0, 0, 0).Children(func() {
				if Button(c, v, s.Confirm, nil).Height(TouchTarget + 4).Radius(RadiusMd).Clicked() {
					answered, confirmed, st.Open = true, true, false
				}
				if Button(c, Secondary, s.Cancel, nil).Height(TouchTarget + 4).Radius(RadiusMd).Clicked() {
					answered, st.Open = true, false
				}
			})
		})
	} else {
		ui.DialogBase(c, &st.Open, func(backdrop, panel ui.Element) {
			backdrop.Background(p.Scrim).Padding(S4).
				Transition(ui.ElementTransition{Duration: DurBase, ExitDuration: DurExit, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
			panel.Width(380).MaxWidthPercent(100).Padding(S5).Gap(S5).Radius(RadiusLg).Background(p.Raised).
				Border(1, p.Border).Shadow(0, 16, 48, -8, p.Shadow).Transition(FadeIn()).Role(ui.RoleAlertDialog)
			body()
			ui.Row(c).Gap(S2).Justify(ui.End).Children(func() {
				// Enter takes the focused button: Cancel when the action
				// destroys something, the action otherwise.
				cancel := Button(c, Secondary, s.Cancel, nil)
				action := Button(c, v, s.Confirm, nil)
				if s.Danger {
					cancel.AutoFocus()
				} else {
					action.AutoFocus()
				}
				if cancel.Clicked() {
					answered, st.Open = true, false
				}
				if action.Clicked() {
					answered, confirmed, st.Open = true, true, false
				}
			})
			closeCorner(c, &st.Open)
		})
	}
	if wasOpen && !st.Open && !answered {
		answered = true // Escape, the scrim, the ×: a cancel
	}
	return answered, confirmed
}

// Adaptive shows fn in a popover under anchor on wide windows, and in a
// sheet on phones: the GIF picker, the mute menu, filters.
func Adaptive(c *ui.Context, anchor ui.Element, st *SheetState, title string, width float32, fn func()) {
	if Compact(c) {
		Sheet(c, st, title, fn)
		return
	}
	p := Of(c)
	ui.PopoverBase(c, anchor, &st.Open, func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Width(width).Padding(S3).Gap(S3).Radius(RadiusMd).Background(p.Raised).
			Border(1, p.Border).Shadow(0, 12, 32, -6, p.Shadow).Shadow(0, 1, 2, 0, p.Shadow).Transition(PopIn())
		fn()
	})
}

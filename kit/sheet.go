package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// SheetState is a bottom sheet's (or adaptive popover's) state, kept in
// the app's model.
type SheetState struct {
	Open bool
	drag float32
}

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
// as phones do: it springs up, follows the finger when its grabber is
// dragged, and closes on a fling down or a drag past a third of it, or on
// a tap of the scrim or Escape.
func Sheet(c *ui.Context, st *SheetState, title string, fn func()) {
	p := Of(c)
	ui.DialogBase(c, &st.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Scrim).Justify(ui.End).AlignItems(ui.Stretch).
			Transition(ui.ElementTransition{Duration: DurBase, ExitDuration: 180 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
		panel.Radius(RadiusLg+4, RadiusLg+4, 0, 0).Background(p.Raised).Padding(0, S4, S6, S4).Gap(S3).
			MaxHeightPercent(88).AlignSelf(ui.Stretch).
			Transition(ui.ElementTransition{
				Animation:    &SheetSpring,
				ExitDuration: 220 * time.Millisecond, ExitEase: EaseDrawer,
				Enter: &ui.Motion{Opacity: 1, YPercent: 100},
				Exit:  &ui.Motion{Opacity: 1, YPercent: 100},
			})
		grab := ui.Box(c).Height(20).AlignSelf(ui.Stretch).AlignItems(ui.Center).Justify(ui.Center).
			Cursor(ui.CursorGrab).Label("Drag down to close").Children(func() {
			ui.Box(c).Size(36, 4).Radius(RadiusFull).Background(p.BorderStrong)
		})
		var y float32
		if _, dy, ok := grab.Dragged(); ok {
			st.drag += dy
			if st.drag < 0 { // friction upwards, rather than a wall
				st.drag = st.drag * 0.4
			}
		}
		if _, vy, ok := grab.DragEnded(); ok {
			_, h := panel.Bounds().W, panel.Bounds().H
			if vy > 500 || st.drag > h/3 {
				st.Open = false
			}
			st.drag = 0
			y = panel.Spring("drag", 0, ui.SpringOptions{Duration: 350 * time.Millisecond, Velocity: vy})
		} else if grab.Dragging() {
			y = panel.Jump("drag", st.drag)
		} else {
			y = panel.AnimateTo("drag", 0, SheetSpring)
		}
		panel.Translate(0, y)
		if title != "" {
			SheetTitle(c, title, &st.Open)
		}
		fn()
	})
}

// Dialog shows fn in a dialog centered over a scrim. Dialogs stay centered
// and scale from 0.97, without moving.
func Dialog(c *ui.Context, open *bool, title string, width float32, fn func()) {
	p := Of(c)
	if width == 0 {
		width = 420
	}
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Scrim).Padding(S4).
			Transition(ui.ElementTransition{Duration: DurBase, ExitDuration: DurExit, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
		panel.Width(width).MaxWidthPercent(100).Padding(S5).Gap(S4).Radius(RadiusLg).Background(p.Raised).
			Border(1, p.Border).Shadow(0, 16, 48, -8, p.Shadow).Transition(FadeIn())
		if title != "" {
			SheetTitle(c, title, open)
		}
		fn()
	})
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

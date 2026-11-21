package kit

import (
	"github.com/egoist/mygo/ui"
)

// Toasts draws the window's toasts as a Sonner-like stack at the bottom:
// the newest whole in front, S5 above the window's bottom edge, the ones
// before it peeking above it, opaque so they never show through each
// other. They glide as the stack changes, the newest rises in, a click
// dismisses (as Sonner's does), and a leaving toast drops and fades. The
// timing, pausing on hover, and focus are mygo's (see ui.ToastViewportBase).
func Toasts(c *ui.Context) {
	p := Of(c)
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		for i, t := range toasts {
			// Depth 0 is the newest, drawn last, in front.
			depth := float32(len(toasts) - 1 - i)
			toast := ui.ToastBase(c, t)
			root := toast.Root
			root.Row().MaxWidth(440).Gap(S3).Padding(S15, S3).Radius(RadiusMd).
				Background(p.Raised).Border(1, p.Border).AlignItems(ui.Center).Label(t.Title)
			// Only the front toast casts a shadow: one on each stacked
			// card would paint the gaps between them muddy.
			if depth == 0 {
				root.Shadow(0, 8, 24, -8, p.Shadow)
			}
			// Attached anchors measure against the parent's padding box,
			// so the stack's margin from the window's edge goes in the
			// Top offset, not in the viewport's padding. Older toasts
			// sit S15 higher each, sprung so a new toast lifts them
			// smoothly.
			up := root.AnimateTo("stackY", depth*14, ui.Snappy)
			s := root.AnimateTo("stackS", 1-depth*0.05, ui.Snappy)
			root.Attach(ui.AnchorBottom, ui.AnchorBottom).Top(-S5-up).Scale(s, s)
			root.Transition(ui.ElementTransition{
				Duration: DurBase, Ease: EaseOut, ExitDuration: DurExit,
				Enter: &ui.Motion{Y: 8, Scale: 0.97},
				Exit:  &ui.Motion{Y: 12, Scale: 0.98},
			})
			root.Children(func() {
				// Clicking the card dismisses it, as Sonner's does; the
				// action button, drawn after, sits over it and takes its
				// own clicks.
				if ui.ButtonBase(c.Key("dismiss")).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).
					Label("Dismiss").Clicked() {
					c.CloseToast(t.ID)
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(S05).Children(func() {
					ui.Text(c, t.Title).FontSize(SizeBody).TextColor(p.Text)
					if t.Description != "" {
						ui.Text(c, t.Description).FontSize(SizeSmall).TextColor(p.TextMuted)
					}
				})
				if t.Action != "" {
					toast.ActionButton().Label(t.Action).Children(func() {
						ui.Text(c, t.Action).FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(p.AccentStrong)
					})
				}
			})
		}
	})
}

package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Chip is a filter chip: a pill that is selected or not. It returns the
// button; Clicked reports a press.
func Chip(c *ui.Context, label string, selected bool) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c)
	// No outlines: unselected chips are text until hovered, the selected
	// one is tinted.
	bg, fg := ui.Transparent, p.TextMuted
	switch {
	case selected:
		bg, fg = p.AccentSubtle, p.AccentStrong
	case b.Pressed():
		bg, fg = p.Pressed, p.Text
	case b.Hovered():
		bg, fg = p.Hover, p.Text
	}
	h := float32(26)
	if Touch(c) {
		h = 34
	}
	b.Height(h).PaddingX(S2 + 2).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
		Background(b.AnimateColor("bg", bg, Color)).
		Label(label).AccessSelected(selected)
	b.Children(func() {
		ui.Text(c, label).FontSize(SizeSmall).FontWeight(WeightMedium).
			TextColor(b.AnimateColor("fg", fg, Color)).SingleLine()
	})
	return b
}

// TokenChip is a removable filter token in a search field, as
// "from: @alice". It reports whether its remove button was pressed.
func TokenChip(c *ui.Context, key, value string) (removed bool) {
	p := Of(c)
	ui.Row(c).Height(24).PaddingStart(S2).PaddingEnd(2).Gap(2).Radius(RadiusSm).AlignItems(ui.Center).
		Background(p.AccentSubtle).Shrink(0).Children(func() {
		ui.RichText(c,
			ui.Span{Text: key + ": ", Color: p.AccentStrong, Weight: WeightMedium},
			ui.Span{Text: value, Color: p.Text},
		).FontSize(SizeSmall).SingleLine()
		x := ui.ButtonBase(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Label("Remove " + key + " filter")
		if x.Hovered() {
			x.Background(p.Hover)
		}
		x.Children(func() { ui.Icon(c, IconX).Size(12, 12).TextColor(p.AccentStrong) })
		removed = x.Clicked()
	})
	return removed
}

// tabSlide moves the tabs' indicator: it travels on screen, so a spring
// without bounce, about 280 ms, which keeps its velocity when clicked
// again on the way (Emil Kowalski: movement is interruptible, no
// overshoot in UI chrome).
var tabSlide = ui.SpringOptions{Duration: 280 * time.Millisecond}

const tabPad = 2

// Tabs is a segmented control, as GIFs / Stickers / Emoji. One tinted
// indicator slides under the labels to the chosen one; the labels'
// colors cross-fade as it arrives. Clicking changes *selected.
func Tabs(c *ui.Context, selected *int, labels ...string) ui.Element {
	p := Of(c)
	n := len(labels)
	s := ui.SegmentedBase(c, selected, n)
	track := s.Track.Padding(tabPad).Radius(RadiusSm + 2).Background(p.BgInset)
	trackW := track.Bounds().W // the last layout's width, to size the indicator
	track.Children(func() {
		// The indicator, first so it is under the labels. Each segment is
		// an equal share of the track, so the indicator is one share wide
		// and moves by its own width per step.
		ind := ui.Box(c.Key("indicator")).Absolute().Top(tabPad).Bottom(tabPad).Left(0).
			Radius(RadiusSm).Background(p.AccentSubtle).PassThrough()
		if trackW > 0 {
			ind.Width((trackW - 2*tabPad) / float32(n))
		} else {
			ind.WidthPercent(100 / float32(n))
		}
		at := ind.Spring("x", float32(*selected), tabSlide)
		ind.TranslatePercent(at*100, 0)
		for i, l := range labels {
			seg := s.Segment(i).Grow(1).Basis(0).Height(28).PaddingX(S3).Radius(RadiusSm).
				AlignItems(ui.Center).Justify(ui.Center).ZIndex(1) // over the indicator
			fg := p.TextMuted
			switch {
			case i == *selected:
				fg = p.AccentStrong
			case seg.Hovered():
				fg = p.Text
			}
			seg.Children(func() {
				ui.Text(c, l).FontSize(SizeSmall).FontWeight(WeightMedium).
					TextColor(seg.AnimateColor("fg", fg, ui.Animation{Duration: DurFast, Ease: EaseOut})).SingleLine()
			})
		}
	})
	return s.Track
}

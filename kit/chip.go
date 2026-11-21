package kit

import "github.com/egoist/mygo/ui"

// Chip is a filter chip: a pill that is selected or not. It returns the
// button; Clicked reports a press.
func Chip(c *ui.Context, label string, selected bool) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c)
	bg, fg, border := ui.Transparent, p.TextMuted, p.Border
	switch {
	case selected:
		bg, fg, border = p.AccentSubtle, p.AccentStrong, p.AccentSubtle
	case b.Pressed():
		bg, fg = p.Pressed, p.Text
	case b.Hovered():
		bg, fg = p.Hover, p.Text
	}
	h := float32(28)
	if Touch(c) {
		h = 34
	}
	b.Height(h).PaddingX(S3).Radius(RadiusFull).AlignItems(ui.Center).Shrink(0).
		Background(b.AnimateColor("bg", bg, Color)).Border(1, b.AnimateColor("bd", border, Color)).
		Label(label).Selected(selected)
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

// Tabs is a segmented control, as Emoji / GIFs / Stickers. Clicking
// changes *selected.
func Tabs(c *ui.Context, selected *int, labels ...string) ui.Element {
	p := Of(c)
	s := ui.SegmentedBase(c, selected, len(labels))
	s.Track.Padding(2).Gap(2).Radius(RadiusSm+2).Background(p.BgInset).Children(func() {
		for i, l := range labels {
			seg := s.Segment(i).Grow(1).Height(28).PaddingX(S3).Radius(RadiusSm).
				AlignItems(ui.Center).Justify(ui.Center)
			bg, fg := ui.Transparent, p.TextMuted
			if i == *selected {
				bg, fg = p.Raised, p.Text
				seg.Shadow(0, 1, 2, 0, p.Shadow)
			} else if seg.Hovered() {
				fg = p.Text
			}
			seg.Background(seg.AnimateColor("bg", bg, Color))
			seg.Children(func() {
				ui.Text(c, l).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(fg).SingleLine()
			})
		}
	})
	return s.Track
}

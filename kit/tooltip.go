package kit

import "github.com/egoist/mygo/ui"

// SideTooltip shows text beside anchor, to its right: the tooltip of an
// element on a vertical strip (the space rail), where one above it would
// cover the element above. It shows and goes as every tooltip does (the
// pointer resting on it, the keyboard focus, a press hiding it), in the
// theme's tooltip colors. The anchor should name itself with Label.
// Nothing on touch screens, which have no hover.
func SideTooltip(c *ui.Context, anchor ui.Element, text string) {
	if Touch(c) || text == "" {
		return
	}
	p := Of(c)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		fill, ink := c.Theme().Inverse, c.Theme().InverseText
		if fill.A == 0 { // the theme leaves them to the defaults: its text, turned over
			fill, ink = p.Text, p.Bg
		}
		tip.AttachTo(anchor, ui.AnchorRight, ui.AnchorLeft).Margin(0, 0, 0, 8)
		tip.Padding(4, 8).Radius(RadiusSm).Background(fill).TextColor(ink)
		tip.Children(func() {
			ui.Text(c, text).FontSize(SizeSmall).FontWeight(WeightMedium).SingleLine()
		})
	})
}

// IconButtonSide is IconButton with its tooltip to the right, for icon
// buttons on a vertical strip.
func IconButtonSide(c *ui.Context, v Variant, icon *ui.SVG, label string) ui.Element {
	b := iconButton(c, v, icon, label, false)
	SideTooltip(c, b, label)
	return b
}

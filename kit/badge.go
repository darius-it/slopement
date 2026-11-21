package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// CountText formats an unread count, capping it.
func CountText(n int) string {
	if n > 999 {
		return "999+"
	}
	return strconv.Itoa(n)
}

// pillText centers a short line of text in a pill of height h: a row that
// centers its child, and a line box as tall as the font, so the glyphs sit
// on the pill's middle rather than on a taller line's.
func pillText(c *ui.Context, text string, h, size float32, weight int, fg, bg ui.Color) ui.Element {
	box := ui.Row(c).Height(h).MinWidth(h).PaddingX(h / 3).Radius(RadiusFull).Background(bg).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
	box.Children(func() {
		ui.Text(c, text).FontSize(size).FontWeight(weight).FontFeatures("tnum").TextColor(fg).
			FixedLineHeight(size + 1).SingleLine()
	})
	return box
}

// Count is an unread badge: neutral for unread messages, the accent for
// mentions (highlights). Nothing is built for 0.
func Count(c *ui.Context, n int, mention bool) {
	if n <= 0 {
		return
	}
	p := Of(c)
	// Translucent, so it reads on any row: plain, hovered or selected.
	bg, fg := p.Text.Alpha(0.09), p.TextMuted
	if mention {
		bg, fg = p.Accent, p.OnAccent
	}
	pillText(c, CountText(n), 18, SizeCaption, WeightSemibold, fg, bg).Label(CountText(n) + " unread")
}

// Dot is an unread dot without a count.
func Dot(c *ui.Context, mention bool) ui.Element {
	p := Of(c)
	col := p.TextMuted
	if mention {
		col = p.Accent
	}
	return ui.Box(c).Size(8, 8).Radius(RadiusFull).Background(col).Shrink(0)
}

// Pill is a small neutral label, as "Admin".
func Pill(c *ui.Context, text string) ui.Element {
	p := Of(c)
	return pillText(c, text, 20, SizeCaption, WeightMedium, p.TextMuted, p.BgInset)
}

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

// Count is an unread badge: neutral for unread messages, the accent for
// mentions (highlights). Nothing is built for 0.
func Count(c *ui.Context, n int, mention bool) {
	if n <= 0 {
		return
	}
	p := Of(c)
	bg, fg := p.Text.Alpha(0.10).Over(p.BgSubtle), p.Text
	if mention {
		bg, fg = p.Accent, p.OnAccent
	}
	ui.Text(c, CountText(n)).FontSize(SizeCaption).FontWeight(WeightSemibold).FontFeatures("tnum").
		TextColor(fg).Background(bg).Radius(RadiusFull).Height(18).MinWidth(18).Padding(0, 6).
		TextAlign(ui.Center).SingleLine().Shrink(0).Label(CountText(n) + " unread")
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

// Pill is a small neutral label, as "Encrypted" or "Admin".
func Pill(c *ui.Context, text string) ui.Element {
	p := Of(c)
	return ui.Text(c, text).FontSize(SizeCaption).FontWeight(WeightMedium).TextColor(p.TextMuted).
		Padding(1, 6).Radius(RadiusFull).Background(p.BgInset).SingleLine()
}

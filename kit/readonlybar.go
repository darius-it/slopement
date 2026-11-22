package kit

import "github.com/egoist/mygo/ui"

// ReadOnlyBar stands where the composer would, in a room the user may read
// but not write in (an announcements room): a quiet bar with a lock, in the
// composer's place and width, so the room does not look like it takes
// messages. It is the composer's height, at least 44 DIP on touch screens.
func ReadOnlyBar(c *ui.Context, text string) {
	p := Of(c)
	h := float32(44)
	ui.Column(c).Padding(0, S4, S4, S4).Children(func() {
		ui.Row(c.Key("read-only")).Height(max(h, controlHeight(c)+S2)).Gap(S2).PaddingX(S4).Radius(RadiusLg).
			Background(p.BgInset).Border(1, p.Border).AlignItems(ui.Center).Justify(ui.Center).
			Role(ui.RoleStatus).Label(text).Children(func() {
			ui.Icon(c, IconLock).Size(15, 15).TextColor(p.TextFaint)
			ui.Text(c, text).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
		})
	})
}

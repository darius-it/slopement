package kit

import "github.com/egoist/mygo/ui"

// StatusCloud is a status as the small thought cloud of the chat list's mini
// pictures, but always open: the emoji and the words in one line, with the two
// dots that trail down to whoever thinks it (below and to the left). It is a
// button: a press asks to edit the status. It returns the cloud, whose Clicked
// the caller reads; the caller places it (it takes its own size).
func StatusCloud(c *ui.Context, s Status) ui.Element {
	p := Of(c)
	words := statusWords(s)
	cloud := ui.ButtonBase(c.Key("status-cloud")).Height(20).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Start).
		Border(1, p.Border).Shadow(0, 2, 6, -2, p.Shadow).Label("Edit your status: " + words)
	bg := p.Raised
	if cloud.Hovered() {
		bg = p.Hover
	}
	cloud.Background(bg)
	cloud.Children(func() {
		if s.Emoji != "" {
			ui.Box(c.Key("slot")).Size(18, 18).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, s.Emoji).FontSize(11)
			})
		}
		if s.Text != "" {
			ui.Text(c, s.Text).FontSize(SizeCaption).TextColor(p.Text).SingleLine().Shrink(1).MaxWidth(190).
				Margin(0, S2, 0, 2).AccessHidden(true) // the button's label reads it already
		}
		for i, d := range []struct{ x, y, s float32 }{{3, 22, 4}, {-1, 27, 3}} {
			ui.Box(c.Key(i)).Size(d.s, d.s).Radius(RadiusFull).Background(p.Raised).Border(1, p.Border).
				Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Left(d.x).Top(d.y)
		}
	})
	return cloud
}

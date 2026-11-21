package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// UserColorNames name the palette's colors for screen readers.
var UserColorNames = [8]string{"Red", "Orange", "Olive", "Green", "Teal", "Blue", "Purple", "Pink"}

// SwatchPicker chooses a username color: "Auto" (the hashed color, -1) or
// one of the palette's, all of which read well on both themes. It shows a
// preview of the name in the chosen color.
func SwatchPicker(c *ui.Context, selected *int, name, mxid string) {
	p := Of(c)
	ui.Column(c).Gap(S3).Children(func() {
		col := p.Users[HashIndex(mxid)]
		if *selected >= 0 {
			col = p.Users[*selected]
		}
		ui.Row(c).Gap(S2).Padding(S2, S3).Radius(RadiusSm).Background(p.BgInset).AlignItems(ui.Center).Children(func() {
			ui.Text(c, name).FontSize(SizeBody).FontWeight(WeightSemibold).TextColor(col)
			ui.Text(c, "Hey, this is how my name looks").FontSize(SizeBody).TextColor(p.Text).SingleLine()
		})
		ui.Row(c).Gap(S2).Wrap().Children(func() {
			swatch(c, -1, selected, p.Users[HashIndex(mxid)], "Auto")
			for i, u := range p.Users {
				swatch(c, i, selected, u, UserColorNames[i])
			}
		})
	})
}

func swatch(c *ui.Context, i int, selected *int, col ui.Color, label string) {
	p := Of(c)
	const s = 28
	b := ui.RadioBase(c.Key("sw"+strconv.Itoa(i)), selected, i).Size(s+6, s+6).Radius(RadiusFull).
		AlignItems(ui.Center).Justify(ui.Center).Label(label).Tooltip(label)
	ring := ui.Transparent
	if *selected == i {
		ring = p.Text
	} else if b.Hovered() {
		ring = p.BorderStrong
	}
	b.Border(2, b.AnimateColor("ring", ring, Color))
	Pressable(b)
	b.Children(func() {
		d := ui.Box(c).Size(s, s).Radius(RadiusFull).Background(col).AlignItems(ui.Center).Justify(ui.Center)
		if i < 0 {
			d.Children(func() {
				ui.Text(c, "A").FontSize(SizeSmall).FontWeight(WeightSemibold).TextColor(p.Bg)
			})
		}
	})
}

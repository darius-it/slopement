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

// ColorDots is a small row of the palette's colors for a name, with "Auto"
// first (-1): the choice made this frame, if any. For a card, where the
// full picker would be too much.
func ColorDots(c *ui.Context, selected int, mxid string) (chosen int, changed bool) {
	p := Of(c)
	chosen = selected
	ui.Row(c).Gap(S05).AlignItems(ui.Center).Children(func() {
		for i := -1; i < len(p.Users); i++ {
			col, label := p.Users[HashIndex(mxid)], "Automatic color"
			if i >= 0 {
				col, label = p.Users[i], UserColorNames[i]
			}
			b := ui.ButtonBase(c.Key("dot"+strconv.Itoa(i))).Size(20, 20).Radius(RadiusFull).
				AlignItems(ui.Center).Justify(ui.Center).Label(label).Tooltip(label).Checked(selected == i)
			ring := ui.Transparent
			if selected == i {
				ring = p.Text
			} else if b.Hovered() {
				ring = p.BorderStrong
			}
			b.Border(1.5, b.AnimateColor("ring", ring, Color))
			Pressable(b)
			b.Children(func() {
				d := ui.Box(c).Size(12, 12).Radius(RadiusFull).Background(col)
				if i < 0 {
					d.Border(1, p.Bg) // "Auto" is the hashed color with a hairline: it reads as the default
				}
			})
			if b.Clicked() {
				chosen, changed = i, true
			}
		}
	})
	return chosen, changed
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

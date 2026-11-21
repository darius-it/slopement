package kit

import (
	"github.com/egoist/mygo/ui"
)

// RailItemData is a space (or Home) in the rail.
type RailItemData struct {
	ID      string
	Name    string
	Avatar  AvatarSpec
	Icon    *ui.SVG // instead of an avatar, as Home's
	Unread  bool
	Mention int
}

// RailItem is a space of the rail: a 44 DIP rounded square with an
// indicator on the rail's edge, a short pip while it has unread messages
// and a tall bar while it is selected, which grows between them. It
// returns the button.
func RailItem(c *ui.Context, d RailItemData, selected bool) ui.Element {
	p := Of(c)
	const size = 44
	var b ui.Element
	ui.Row(c.Key(d.ID)).Width(RailWidth).Height(size).AlignItems(ui.Center).Children(func() {
		// The indicator, on the window's edge.
		h := float32(0)
		switch {
		case selected:
			h = 24
		case d.Unread || d.Mention > 0:
			h = 8
		}
		ind := ui.Box(c).Width(4).Radius(0, 4, 4, 0).Background(p.Text)
		hh := ind.AnimateWith("h", h, DurBase, EaseOut)
		ind.Height(hh).Opacity(min(hh/4, 1))
		ui.Box(c).Width((RailWidth - size) / 2 - 4)

		b = ui.ButtonBase(c).Size(size, size).Radius(RadiusMd + 2).AlignItems(ui.Center).Justify(ui.Center).
			Label(d.Name).Selected(selected).Tooltip(d.Name)
		Pressable(b)
		b.Children(func() {
			if d.Icon != nil {
				bg, fg := p.BgInset, p.TextMuted
				if selected {
					bg, fg = p.Accent, p.OnAccent
				} else if b.Hovered() {
					bg, fg = p.Hover.Over(p.BgInset), p.Text
				}
				ui.Box(c).Size(size, size).Radius(RadiusMd + 2).Background(b.AnimateColor("bg", bg, Color)).
					AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, d.Icon).Size(20, 20).TextColor(fg)
				})
			} else {
				a := d.Avatar
				a.Size, a.Square = size, true
				if a.Name == "" {
					a.Name, a.ID = d.Name, d.ID
				}
				Avatar(c, a)
				if selected {
					ui.Box(c).Size(size, size).Radius(a.Size*0.28).Border(2, p.Accent).
						Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).PassThrough()
				}
			}
			if d.Mention > 0 {
				ui.Box(c).Attach(ui.AnchorBottomRight, ui.AnchorCenter).Top(-6).Left(-6).
					Padding(2).Radius(RadiusFull).Background(p.BgSubtle).Children(func() {
					Count(c, d.Mention, true)
				})
			}
		})
	})
	return b
}

// RailDivider separates Home from the spaces.
func RailDivider(c *ui.Context) {
	p := Of(c)
	ui.Box(c).Width(24).Height(1).Background(p.Border).MarginY(S1).AlignSelf(ui.Center)
}

// SectionHeader is a collapsible header of the room list, as
// "Favourites", with an optional count and trailing action.
func SectionHeader(c *ui.Context, title string, open *bool, trailing func()) {
	p := Of(c)
	ui.Row(c.Key("section:"+title)).Height(28).PaddingX(S2).Gap(S1).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c).Gap(S1).AlignItems(ui.Center).Grow(1).Height(28).Radius(RadiusSm).
			Label(title).Expanded(*open)
		fg := p.TextMuted
		if b.Hovered() {
			fg = p.Text
		}
		b.Children(func() {
			ui.Text(c, title).FontSize(SizeCaption + 1).FontWeight(WeightSemibold).TextColor(fg).SingleLine()
			chev := ui.Icon(c, IconChevronD).Size(12, 12).TextColor(fg)
			r := float32(0)
			if !*open {
				r = -90
			}
			chev.Rotate(chev.AnimateWith("r", r, DurFast, EaseOut))
		})
		if b.Clicked() {
			*open = !*open
		}
		if trailing != nil {
			trailing()
		}
	})
}

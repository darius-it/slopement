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
			Label(d.Name).AccessSelected(selected).Tooltip(d.Name)
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
				// The bar on the rail's edge says which is selected; the
				// others are dimmed a little so it stands out without a ring.
				o := float32(1)
				if !selected && !b.Hovered() {
					o = 0.72
				}
				Avatar(c, a).Opacity(b.AnimateWith("o", o, DurFast, EaseOut))
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

// RailAddAction is what the rail's + chose.
type RailAddAction int

const (
	RailAddNone RailAddAction = iota
	RailAddCreate
	RailAddJoin
	RailAddExplore
)

// RailAdd is the rail's last tile, right under the spaces it adds to: a
// dashed tile with a +, which turns to the accent on hover and opens
// Create a space, Join a space and Explore. choice records the menu's
// pick for the app to read.
func RailAdd(c *ui.Context, choice *RailAddAction) ui.Element {
	p := Of(c)
	const size = 44
	var b ui.Element
	ui.Row(c.Key("rail-add")).Width(RailWidth).Height(size).Justify(ui.Center).Children(func() {
		b = ui.ButtonBase(c).Size(size, size).Radius(RadiusMd + 2).AlignItems(ui.Center).Justify(ui.Center).
			Label("Add a space").Tooltip("Add a space")
		bg, fg, bd := ui.Transparent, p.TextMuted, p.BorderStrong
		if b.Hovered() || b.FocusVisible() {
			bg, fg, bd = p.AccentSubtle, p.AccentStrong, p.Accent.Alpha(0.5)
		}
		b.Background(b.AnimateColor("bg", bg, Color)).Border(1.5, b.AnimateColor("bd", bd, Color)).BorderStyle(ui.BorderDashed)
		Pressable(b)
		b.Children(func() { ui.Icon(c, IconPlus).Size(20, 20).TextColor(fg) })
		MenuButton(c, b, func(m *ui.Menu) {
			if m.Item("Create a space").Chosen() {
				*choice = RailAddCreate
			}
			if m.Item("Join a space").Subtitle("With an address or invite link").Chosen() {
				*choice = RailAddJoin
			}
			m.Separator()
			if m.Item("Explore public spaces").Chosen() {
				*choice = RailAddExplore
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
		b := ui.ButtonBase(c).Gap(S1).AlignItems(ui.Center).Justify(ui.Start).Grow(1).Height(28).Radius(RadiusSm).
			Label(title).Expanded(*open)
		fg := p.TextMuted
		if b.Hovered() {
			fg = p.Text
		}
		b.Children(func() {
			ui.Text(c, title).FontSize(SizeCaption + 1).FontWeight(WeightMedium).TextColor(fg).SingleLine()
			// The chevron shows only while it means something: on hover,
			// on keyboard focus, or while the section is closed.
			show := b.Hovered() || b.FocusVisible() || !*open
			chev := ui.Icon(c, IconChevronD).Size(12, 12).TextColor(fg)
			r := float32(0)
			if !*open {
				r = -90
			}
			chev.Rotate(chev.AnimateWith("r", r, DurFast, EaseOut)).
				Opacity(chev.AnimateWith("o", boolf(show), DurFast, EaseOut))
		})
		if b.Clicked() {
			*open = !*open
		}
		if trailing != nil {
			trailing()
		}
	})
}

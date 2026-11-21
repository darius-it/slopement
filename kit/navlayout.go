package kit

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// NavLayout is how the app's places (Chats and Communities) are laid out,
// a setting under Appearance → Layout.
type NavLayout int

const (
	// NavRail: the rail's first item is Chats, then the communities, each
	// showing its own rooms. The desktop default.
	NavRail NavLayout = iota
	// NavTabs: Chats and Communities as two tabs (a switch on desktops,
	// the tab bar on phones). The phone default.
	NavTabs
	// NavInbox: one list, filters on top; communities unfold in place.
	NavInbox
)

// NavLayouts names the layouts, for the setting and the View menu, in the
// order of NavLayout.
var NavLayouts = []struct {
	Name, About string
}{
	{"Rail", "Chats first on the rail, then your communities"},
	{"Tabs", "Chats and Communities as two tabs"},
	{"One inbox", "Everything in one list, with filters"},
}

// DefaultNavLayout is the layout before the user picks one: the rail on
// desktops, tabs on phones.
func DefaultNavLayout() NavLayout {
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		return NavTabs
	}
	return NavRail
}

// Resolve returns the layout to show: the rail, which needs room, shows as
// tabs at phone widths.
func (l NavLayout) Resolve(compact bool) NavLayout {
	if l == NavRail && compact {
		return NavTabs
	}
	return l
}

// LayoutPicker chooses the navigation layout: a card per layout with a
// small diagram of it, its name and what it does; the chosen one ringed in
// the accent, the platform's default marked.
func LayoutPicker(c *ui.Context, sel *NavLayout) {
	def := DefaultNavLayout()
	ui.Row(c).Gap(S3).Wrap().Children(func() {
		for i, l := range NavLayouts {
			layoutCard(c, NavLayout(i), l.Name, l.About, NavLayout(i) == def, sel)
		}
	})
}

func layoutCard(c *ui.Context, l NavLayout, name, about string, isDefault bool, sel *NavLayout) {
	p := Of(c)
	b := ui.RadioBase(c.Key("layout"+name), sel, l).Width(176).Padding(S2).Gap(S2).Column().Radius(RadiusMd).
		Background(p.Raised).Label(name).Description(about)
	on := *sel == l
	bd := p.Border
	switch {
	case on:
		bd = p.Accent
	case b.Hovered():
		bd = p.BorderStrong
	}
	width := float32(1)
	if on {
		width = 2
	}
	b.Border(width, b.AnimateColor("bd", bd, Color))
	Pressable(b)
	b.Children(func() {
		layoutDiagram(c, l, on)
		ui.Column(c).Gap(1).Padding(0, S1, S1, S1).Children(func() {
			ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
				ui.Text(c, name).FontSize(SizeBody).FontWeight(WeightSemibold)
				if isDefault {
					Pill(c, "Default")
				}
			})
			ui.Text(c, about).FontSize(SizeCaption).TextColor(p.TextMuted)
		})
	})
}

// layoutDiagram is a layout's thumbnail, in boxes.
func layoutDiagram(c *ui.Context, l NavLayout, on bool) {
	p := Of(c)
	ink := p.Text.Alpha(0.14)
	hi := p.Text.Alpha(0.3)
	if on {
		hi = p.Accent.Alpha(0.6)
	}
	bar := func(w float32, col ui.Color) { ui.Box(c).Height(4).Width(w).Radius(2).Background(col) }
	list := func(n int) {
		for i := 0; i < n; i++ {
			ui.Row(c.Key(i)).Gap(3).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(RadiusFull).Background(ink)
				bar(28, ink)
			})
		}
	}
	ui.Row(c).Height(80).Radius(RadiusSm).Background(p.BgInset).Clip().AlignItems(ui.Stretch).Children(func() {
		switch l {
		case NavRail:
			ui.Column(c).Width(18).Gap(5).Padding(6, 4).Background(p.Text.Alpha(0.06)).Children(func() {
				ui.Box(c).Size(10, 10).Radius(3).Background(hi)
				ui.Box(c).Height(1).Background(p.Border)
				ui.Box(c).Size(10, 10).Radius(3).Background(ink)
				ui.Box(c).Size(10, 10).Radius(3).Background(ink)
			})
			ui.Column(c).Width(54).Gap(5).Padding(8, 5).Background(p.Text.Alpha(0.03)).Children(func() { list(4) })
			ui.Box(c).Grow(1)
		case NavTabs:
			ui.Column(c).Width(72).Gap(5).Padding(6, 5).Background(p.Text.Alpha(0.03)).Children(func() {
				ui.Row(c).Height(8).Radius(3).Background(ink).Children(func() {
					ui.Box(c).Grow(1).Radius(3).Background(hi)
					ui.Box(c).Grow(1)
				})
				list(3)
			})
			ui.Box(c).Grow(1)
		case NavInbox:
			ui.Column(c).Width(72).Gap(5).Padding(6, 5).Background(p.Text.Alpha(0.03)).Children(func() {
				ui.Row(c).Gap(3).Children(func() { bar(12, hi); bar(12, ink); bar(12, ink) })
				list(1)
				ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 8).Radius(2).Background(hi)
					bar(28, ink)
				})
				ui.Column(c).PaddingStart(11).Gap(4).Children(func() { bar(20, ink); bar(20, ink) })
			})
			ui.Box(c).Grow(1)
		}
	})
}

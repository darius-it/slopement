package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/internal/licenses"
	"slopement/kit"
)

// Settings → About: what version this is, what else it uses and who made it,
// and the open source software it is built from, with each license in full.
// The list is made from the real build (internal/licenses/gen).

func (a *App) aboutPage(c *ui.Context) {
	p := kit.Of(c)
	group(c, "slopement", "", func() {
		ui.Column(c).Padding(kit.S2).Gap(2).Children(func() {
			ui.Text(c, "slopement "+appVersion()).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium)
			ui.Text(c, "A super early alpha of a Matrix client. Not recommended for daily use yet.").FontSize(kit.SizeSmall).TextColor(p.TextMuted)
		})
	})
	group(c, "Also used", "Not part of the app's code, but part of what it does.", func() {
		for _, cr := range licenses.Credits {
			a.creditRow(c, cr)
		}
	})
	n := licenses.All()
	group(c, "Open source software", "slopement is built on the work of the people below. Thank you. Tap one for its license.", func() {
		for i := range n {
			a.noticeRow(c, &n[i])
		}
	})
}

// appVersion is the version the app was built as (the release tag without its "v").
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "(development build)"
}

// creditRow is something slopement uses that is not in go.mod: its name and
// license, what for, and a press opens its page.
func (a *App) creditRow(c *ui.Context, cr licenses.Credit) {
	p := kit.Of(c)
	b := ui.ButtonBase(c.Key("credit:" + cr.Name)).Padding(kit.S2).Gap(kit.S1).Radius(kit.RadiusSm).AlignItems(ui.Stretch).Label(cr.Name)
	b.Column()
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, cr.Name).FontSize(kit.SizeBody).FontWeight(kit.WeightMedium).Grow(1)
			ui.Text(c, cr.License).FontSize(kit.SizeCaption).TextColor(p.TextMuted)
		})
		ui.Text(c, cr.Note).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
	})
	if b.Clicked() {
		a.openURL = cr.URL
	}
}

// noticeRow is one piece of software: its name, version and license on a
// line; a press shows the license's full text and where it comes from.
func (a *App) noticeRow(c *ui.Context, n *licenses.Notice) {
	p := kit.Of(c)
	open := a.noticeOpen[n.Name]
	b := ui.ButtonBase(c.Key("notice:" + n.Name)).Padding(kit.S2).Radius(kit.RadiusSm).AlignItems(ui.Center).Gap(kit.S2).
		Label(n.Name + ", " + n.License).AccessSelected(open)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		ui.Text(c, n.Name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).Grow(1).SingleLine().Shrink(1)
		ui.Text(c, n.Version).FontSize(kit.SizeCaption).TextColor(p.TextFaint).SingleLine()
		ui.Text(c, n.License).FontSize(kit.SizeCaption).TextColor(p.TextMuted).SingleLine()
	})
	if b.Clicked() {
		if a.noticeOpen == nil {
			a.noticeOpen = map[string]bool{}
		}
		a.noticeOpen[n.Name] = !open
	}
	if !open {
		return
	}
	ui.Column(c.Key("notice-text:"+n.Name)).Padding(0, kit.S2, kit.S2, kit.S2).Gap(kit.S2).Children(func() {
		if kit.Button(c.Key("notice-link:"+n.Name), kit.Ghost, n.URL, nil).Clicked() {
			a.openURL = n.URL
		}
		ui.Text(c, n.Text).FontSize(kit.SizeCaption).TextColor(p.TextMuted).Selectable()
	})
}

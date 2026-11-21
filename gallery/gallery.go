// Package gallery is the design kit's gallery: every token and component
// of package kit in its states, light beside dark, and mock screens built
// from kit parts alone with fake data. Run it with `go run .`.
package gallery

import (
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

type pageGroup struct {
	title string
	pages []string
}

var groups = []pageGroup{
	{"Foundations", []string{"Colors", "Typography", "Spacing & radius", "Icons", "Motion"}},
	{"Components", []string{"Buttons", "Inputs & chips", "Avatars & badges", "Room list", "Timeline", "Composer & picker", "Menus & sheets", "Quick switcher", "Verification", "Calls", "Username colours"}},
	{"Screens", []string{"Desktop", "Phone"}},
}

// themeModes: side by side, the system's, light, dark.
var themeModes = []string{"Both", "System", "Light", "Dark"}

// App is the gallery's state.
type App struct {
	page      string
	themeMode int

	// Component state.
	name, query, search string
	chip                 int
	tokens               []string
	tab                  int
	sections             [4]bool
	room, space          string
	sheet, mute          kit.SheetState
	dialog               bool
	composer             kit.ComposerState
	picker               kit.PickerState
	switcher             kit.SwitcherState
	switcherItems        []kit.SwitcherItem
	call                 kit.CallState
	color                int
	colors               kit.UserColors
	verifyStep           int
	did                  string
	sent                 []kit.MessageData
	roomStates           map[string]*kit.RoomMenuState
	now                  time.Time
}

// New returns the gallery.
func New() *App {
	a := &App{
		page:          "Screens/Desktop",
		room:          "!r1",
		space:         "!design",
		sections:      [4]bool{true, true, true, true},
		tokens:        []string{"from: @mira", "has: image"},
		call:          kit.NewCall(),
		color:         -1,
		switcherItems: switcherItems(),
		roomStates:    map[string]*kit.RoomMenuState{},
		now:           time.Now(),
	}
	a.page = "Desktop"
	a.composer.Reply = &kit.Quote{Sender: "Mira", SenderID: "@mira:matrix.org", Body: "Pushed the new rail spacing."}
	for _, r := range rooms {
		a.roomStates[r.ID] = &kit.RoomMenuState{Muted: r.Muted, Favourite: r.Fav, Unread: r.Unread > 0 || r.Mention > 0}
	}
	return a
}

// View builds the gallery window.
func (a *App) View(c *ui.Context) {
	switch a.themeMode {
	case 2:
		c.SetTheme(kit.Theme(false))
	case 3:
		c.SetTheme(kit.Theme(true))
	default:
		kit.Apply(c)
	}
	a.now = time.Now()
	kit.SwitcherShortcut(c, &a.switcher)
	if kit.Compact(c) {
		a.compactView(c)
	} else {
		a.wideView(c)
	}
	if it, msgs := kit.Switcher(c, &a.switcher, a.switcherItems, &a.colors); it != nil {
		a.room = it.ID
		a.did = "Opened " + it.Name
		c.AddToast(ui.Toast{Title: "Opened " + it.Name})
	} else if msgs {
		a.did = "Search messages"
		c.AddToast(ui.Toast{Title: "Would search messages"})
	}
}

func (a *App) wideView(c *ui.Context) {
	p := kit.Of(c)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		// The sidebar of pages.
		ui.Scroll(c).Width(232).Shrink(0).Background(p.BgSubtle).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).
			Padding(kit.S4, kit.S2).Gap(kit.S1).Children(func() {
			ui.Row(c).Gap(kit.S2).Padding(kit.S1, kit.S2, kit.S3, kit.S2).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(22, 22).Radius(6).Background(p.Accent).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, "s").FontSize(14).FontWeight(kit.WeightSemibold).TextColor(p.OnAccent)
				})
				ui.Text(c, "slopement kit").FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
			})
			for _, g := range groups {
				ui.Text(c, g.title).FontSize(kit.SizeCaption).FontWeight(kit.WeightSemibold).TextColor(p.TextMuted).
					Padding(kit.S3, kit.S2, kit.S1, kit.S2)
				for _, pg := range g.pages {
					a.navItem(c, pg)
				}
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			a.toolbar(c)
			ui.Scroll(c).Grow(1).Padding(kit.S6, kit.S8, kit.S8, kit.S8).Gap(kit.S6).Children(func() {
				a.pageView(c)
			})
		})
	})
}

func (a *App) navItem(c *ui.Context, pg string) {
	p := kit.Of(c)
	sel := a.page == pg
	b := ui.ButtonBase(c.Key("nav:"+pg)).Height(30).PaddingX(kit.S2).Radius(kit.RadiusSm).AlignItems(ui.Center).
		Label(pg).Selected(sel)
	bg, fg := ui.Transparent, p.TextMuted
	switch {
	case sel:
		bg, fg = p.AccentSubtle, p.AccentStrong
	case b.Hovered():
		bg, fg = p.Hover, p.Text
	}
	b.Background(b.AnimateColor("bg", bg, kit.Color))
	b.Children(func() {
		w := kit.WeightRegular
		if sel {
			w = kit.WeightMedium
		}
		ui.Text(c, pg).FontSize(kit.SizeBody).FontWeight(w).TextColor(fg).SingleLine()
	})
	if b.Clicked() {
		a.page = pg
	}
}

func (a *App) toolbar(c *ui.Context) {
	p := kit.Of(c)
	ui.Row(c).Height(52).PaddingX(kit.S8).Gap(kit.S3).AlignItems(ui.Center).Shrink(0).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
		ui.Text(c, a.page).FontSize(kit.SizeTitle).FontWeight(kit.WeightSemibold).Grow(1).SingleLine()
		if a.did != "" {
			ui.Text(c, a.did).FontSize(kit.SizeSmall).TextColor(p.TextMuted).SingleLine().Shrink(1)
		}
		kit.Tabs(c.Key("theme"), &a.themeMode, themeModes...).Width(260)
		if kit.IconButton(c, kit.Ghost, kit.IconSearch, "Quick switcher ("+kit.CmdK()+")").Clicked() {
			a.switcher.Toggle()
		}
	})
}

func (a *App) compactView(c *ui.Context) {
	p := kit.Of(c)
	ui.Column(c).Fill().Children(func() {
		ui.Column(c).Padding(kit.S3, kit.S4, kit.S2, kit.S4).Gap(kit.S2).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
			ui.Text(c, a.page).FontSize(kit.SizeHeading).FontWeight(kit.WeightSemibold)
			ui.Scroll(c).Row().Gap(kit.S1 + 2).Scrollbars(false).Children(func() {
				for _, g := range groups {
					for _, pg := range g.pages {
						if kit.Chip(c.Key("nav:"+pg), pg, a.page == pg).Clicked() {
							a.page = pg
						}
					}
				}
			})
		})
		ui.Scroll(c).Grow(1).Padding(kit.S4).Gap(kit.S5).Children(func() { a.pageView(c) })
	})
}

// specimen shows fn on a card, twice (light and dark) in "Both" mode on
// wide windows. single builds it once in the window's theme: for overlays,
// which must not open twice.
func (a *App) specimen(c *ui.Context, title, desc string, single bool, fn func()) {
	p := kit.Of(c)
	ui.Column(c.Key("spec:"+title)).Gap(kit.S2).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
		}
		if desc != "" {
			ui.Text(c, desc).FontSize(kit.SizeSmall).TextColor(p.TextMuted).MaxWidth(720)
		}
		if a.themeMode == 0 && !single && !kit.Compact(c) {
			ui.Row(c).Gap(kit.S3).AlignItems(ui.Stretch).Children(func() {
				for _, dark := range []bool{false, true} {
					t := kit.Theme(dark)
					pp := &kit.Light
					if dark {
						pp = &kit.DarkPalette
					}
					ui.Themed(c.Key(dark), t).Grow(1).Basis(0).MinWidth(0).Padding(kit.S5).Gap(kit.S3).
						Radius(kit.RadiusMd + 2).Border(1, pp.Border).Children(fn)
				}
			})
			return
		}
		ui.Column(c).Padding(kit.S5).Gap(kit.S3).Radius(kit.RadiusMd + 2).Border(1, p.Border).Background(p.Bg).Children(fn)
	})
}

func (a *App) pageView(c *ui.Context) {
	switch a.page {
	case "Colors":
		a.colorsPage(c)
	case "Typography":
		a.typePage(c)
	case "Spacing & radius":
		a.spacingPage(c)
	case "Icons":
		a.iconsPage(c)
	case "Motion":
		a.motionPage(c)
	case "Buttons":
		a.buttonsPage(c)
	case "Inputs & chips":
		a.inputsPage(c)
	case "Avatars & badges":
		a.avatarsPage(c)
	case "Room list":
		a.roomListPage(c)
	case "Timeline":
		a.timelinePage(c)
	case "Composer & picker":
		a.composerPage(c)
	case "Menus & sheets":
		a.menusPage(c)
	case "Quick switcher":
		a.switcherPage(c)
	case "Verification":
		a.verifyPage(c)
	case "Calls":
		a.callsPage(c)
	case "Username colours":
		a.colorsUserPage(c)
	case "Desktop":
		a.desktopScreen(c)
	case "Phone":
		a.phoneScreen(c)
	}
}

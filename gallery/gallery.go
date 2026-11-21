// Package gallery is the design kit's gallery: every token and component
// of package kit in its states, light beside dark, and mock screens built
// from kit parts alone with fake data. Run it with `go run .`.
package gallery

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/format"
	"slopement/kit"
	"slopement/picker"
)

type pageGroup struct {
	title string
	pages []string
}

var groups = []pageGroup{
	{"Foundations", []string{"Colors", "Typography", "Spacing & radius", "Icons", "Motion"}},
	{"Components", []string{"Buttons", "Inputs & chips", "Avatars & badges", "Room list", "Timeline", "Formatting", "Media & embeds", "Composer & picker", "Polls", "Status & profile", "Menus & sheets", "Quick switcher", "Verification", "Calls", "Username colours"}},
	{"Screens", []string{"Desktop", "Phone", "Settings"}},
	{"Concepts", []string{"Navigation: overview", "A · Chats & Communities", "B · Chats on the rail", "C · One inbox"}},
}

// themeModes: side by side, the system's, light, dark.
var themeModes = []string{"Both", "Auto", "Light", "Dark"}

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
	dialog               kit.SheetState // the leave confirmation
	composer             kit.ComposerState
	phoneComposer        kit.ComposerState
	picker               kit.PickerState
	pick                 *picker.Controller
	switcher             kit.SwitcherState
	switcherItems        []kit.SwitcherItem
	call                 kit.CallState
	color                int
	colors               kit.UserColors
	did                  string
	sent                 []kit.MessageData
	sentBubbles          []kit.BubbleData
	roomStates           map[string]*kit.RoomMenuState
	reveals              kit.Reveals
	now                  time.Time

	// The formatting playground.
	fmtSrc string
	fmtDoc *format.Node
	fmtWas string

	social
	concept conceptState

	// gifURLs maps a sent GIF's media ID to its full GIF, whose animation
	// fills in once loaded.
	gifURLs map[string]string

	// win is the gallery's window, for dialogs; toast is a message to show.
	win   *mygo.Window
	toast string

	// MenuChanged, set by main, rebuilds the menu bar when what it checks
	// changes (the layout); menuLayout is the layout it was built with.
	MenuChanged func()
	menuLayout  kit.NavLayout

	// The media viewer.
	viewer      bool
	viewerItems []kit.MediaSpec
	viewerAt    int
}

// New returns the gallery, with a KLIPY picker of the environment's key.
func New() *App { return NewWith(picker.NewWithKey()) }

// NewWith returns the gallery with the given picker controller (tests give
// one without network).
func NewWith(pick *picker.Controller) *App {
	a := &App{
		page:          "Desktop",
		room:          "!r1",
		space:         "!design",
		sections:      [4]bool{true, true, true, true},
		tokens:        []string{"from: @mira", "has: image"},
		call:          kit.NewCall(),
		color:         -1,
		switcherItems: switcherItems(),
		roomStates:    map[string]*kit.RoomMenuState{},
		reveals:       kit.Reveals{},
		gifURLs:       map[string]string{},
		pick:          pick,
		now:           time.Now(),
		fmtSrc:        formattingSample,
	}
	a.social = newSocial(a.now)
	a.menuLayout = a.navLayout
	a.composer.Reply = &kit.Quote{Sender: "Mira", SenderID: "@mira:matrix.org", Body: "Pushed the new rail spacing."}
	for _, r := range rooms {
		a.roomStates[r.ID] = &kit.RoomMenuState{Muted: r.Muted, Favourite: r.Fav, Unread: r.Unread > 0 || r.Mention > 0}
	}
	return a
}

// Attach connects the gallery to its window: background loads (KLIPY)
// publish their results through it.
func (a *App) Attach(w *mygo.Window) {
	a.win = w
	a.pick.SetUpdate(w.Update)
}

// View builds the gallery window.
func (a *App) View(c *ui.Context) {
	switch a.themeMode {
	case 2:
		kit.ApplyTheme(c, false)
	case 3:
		kit.ApplyTheme(c, true)
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
	} else if msgs {
		a.did = "Search messages"
	}
	kit.MediaViewer(c, &a.viewer, a.viewerItems, &a.viewerAt)
	a.globalOverlays(c)
	if a.toast != "" {
		c.AddToast(ui.Toast{Title: a.toast})
		a.toast = ""
	}
	if a.navLayout != a.menuLayout {
		a.menuLayout = a.navLayout
		if a.MenuChanged != nil {
			a.MenuChanged() // View → Layout checks the new one
		}
	}
}

// openViewer shows items in the media viewer at i.
func (a *App) openViewer(items []kit.MediaSpec, i int) {
	a.viewerItems, a.viewerAt, a.viewer = items, i, true
}

// vibrant reports whether the sidebar shows the window's material: only
// while the theme follows the system's appearance, as the material does.
func (a *App) vibrant(c *ui.Context) bool {
	return c.Vibrancy() && a.themeMode <= 1
}

func (a *App) wideView(c *ui.Context) {
	p := kit.Of(c)
	bar := c.TitleBar() // the native window controls' room
	sideBg := p.BgSubtle
	if a.vibrant(c) {
		c.Root().Background(ui.Transparent)
		sideBg = ui.Transparent // the sidebar material shows through
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		// The sidebar of pages: under the traffic lights on macOS, its
		// empty parts drag the window.
		ui.Scroll(c).Width(232).Shrink(0).Background(sideBg).BorderWidth(0, 1, 0, 0).BorderColor(p.Border).
			Padding(max(bar.Bottom+kit.S3, kit.S4), kit.S2, kit.S4, kit.S2).Gap(kit.S1).DragWindow().Children(func() {
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
		// The content keeps the kit's opaque background, material or not.
		ui.Column(c).Grow(1).MinWidth(0).Background(p.Bg).Children(func() {
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
		Justify(ui.Start).Label(pg).AccessSelected(sel)
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

// toolbar is the content's title bar: it drags the window, and keeps clear
// of the native window buttons (Windows' and Linux's, at its right).
func (a *App) toolbar(c *ui.Context) {
	p := kit.Of(c)
	bar := c.TitleBar()
	ui.Row(c).Height(max(52, bar.Height)).PaddingStart(kit.S8).PaddingEnd(kit.S8+bar.Right).Gap(kit.S3).
		AlignItems(ui.Center).Shrink(0).DragWindow().
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
	bar := c.TitleBar() // a narrow desktop window keeps its controls up here
	ui.Column(c).Fill().Background(p.Bg).Children(func() {
		ui.Column(c).Padding(max(kit.S3, bar.Bottom+kit.S1), kit.S4+bar.Right, kit.S2, kit.S4).Gap(kit.S2).Shrink(0).
			DragWindow().BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
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

// specimenFit shows fn on cards that wrap their content, centered in
// them: for panels with a width of their own (dialogs, verification).
// "Both" puts light beside dark, wrapping onto a new line when the page
// is too narrow for the two.
func (a *App) specimenFit(c *ui.Context, title, desc string, fn func()) {
	p := kit.Of(c)
	card := func(e ui.Element, border ui.Color) {
		e.Padding(kit.S6).Radius(kit.RadiusLg).Border(1, border).AlignItems(ui.Center).Shrink(0).MaxWidthPercent(100).
			Children(fn)
	}
	ui.Column(c.Key("spec:"+title)).Gap(kit.S2).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(kit.SizeLabel).FontWeight(kit.WeightSemibold)
		}
		if desc != "" {
			ui.Text(c, desc).FontSize(kit.SizeSmall).TextColor(p.TextMuted).MaxWidth(720)
		}
		ui.Row(c).Gap(kit.S3).Wrap().AlignItems(ui.Start).Children(func() {
			if a.themeMode == 0 && !kit.Compact(c) {
				for _, dark := range []bool{false, true} {
					pp := &kit.Light
					if dark {
						pp = &kit.DarkPalette
					}
					card(ui.Themed(c.Key(dark), kit.Theme(dark)), pp.Border)
				}
				return
			}
			card(ui.Column(c).Background(p.Bg), p.Border)
		})
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
	case "Formatting":
		a.formattingPage(c)
	case "Media & embeds":
		a.mediaPage(c)
	case "Composer & picker":
		a.composerPage(c)
	case "Polls":
		a.pollsPage(c)
	case "Status & profile":
		a.statusPage(c)
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
	case "Navigation: overview":
		a.conceptsCompare(c)
	case "A · Chats & Communities":
		a.conceptA(c)
	case "B · Chats on the rail":
		a.conceptB(c)
	case "C · One inbox":
		a.conceptC(c)
	case "Settings":
		a.specimen(c, "Appearance → Layout", "How Chats and Communities are laid out. Until you choose, desktops use the rail and phones use tabs (marked Default). The rail needs room, so at phone widths it shows as tabs. The Desktop and Phone screens and View → Layout follow this choice.", true, func() {
			kit.LayoutPicker(c, &a.navLayout)
		})
	case "Desktop":
		a.desktopScreen(c)
	case "Phone":
		a.phoneScreen(c)
	}
}

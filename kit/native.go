package kit

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// menuIcons caches the kit icons made into menu icons (each is drawn once).
var menuIcons = map[*ui.SVG]ui.MenuIcon{}

// MenuIcon makes a kit icon a menu item's icon, as a template: menus show
// it in their text's color (white on the highlight), like ui.Icon. Nil if
// it can't be made, which leaves the item without an icon.
func MenuIcon(icon *ui.SVG) ui.MenuIcon {
	if m, ok := menuIcons[icon]; ok {
		return m
	}
	var m ui.MenuIcon
	if mi, err := mygo.NewMenuIconSVG(icon, true); err == nil {
		m = mi
	}
	menuIcons[icon] = m
	return m
}

// NativeMenus would make context menus and menu buttons the system's. It is
// off everywhere (decided 2026-11-21): the system's menus felt foreign
// beside the kit, so every popup menu is the kit's drawn one (MenuStyle),
// in its type, colors, radius and motion. Only the menu bar stays the
// system's (package platform).
var NativeMenus = false

var _ = runtime.GOOS

// ContextMenu gives e its context menu: on a right click (Control-click
// on macOS) and the menu key, the system's on desktops.
func ContextMenu(c *ui.Context, e ui.Element, build func(m *ui.Menu)) ui.Element {
	if NativeMenus {
		return e.ContextMenu(build)
	}
	return e.PopoverMenu(build, MenuStyle(c))
}

// MenuButton makes e open a menu below it, as a ⋯ button: the system's
// pop-up menu on desktops.
func MenuButton(c *ui.Context, e ui.Element, build func(m *ui.Menu)) ui.Element {
	if NativeMenus {
		return e.Menu(build)
	}
	return e.PopoverMenuButton(build, ui.PopoverMenuButtonOptions{Panel: MenuStyle(c).Panel})
}

// SidebarBg is the background of the app's sidebars: transparent over the
// window's material where it shows (macOS's sidebar vibrancy, Windows
// 11's Mica), the kit's BgSubtle elsewhere.
func SidebarBg(c *ui.Context) ui.Color {
	if c.Vibrancy() {
		return ui.Transparent
	}
	return Of(c).BgSubtle
}

// WindowChrome prepares the root for the platform's window: transparent
// over a window material, so the sidebar shows it while the content keeps
// its opaque Bg. Call it first in the root view, after the theme.
func WindowChrome(c *ui.Context) {
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
}

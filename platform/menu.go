package platform

import (
	"github.com/egoist/mygo"

	"slopement/kit"
)

// Actions are what the menu bar's items do. Nil ones are left out. They
// run on the UI thread through the update function given to MenuBar, so
// they may change the app's state directly.
type Actions struct {
	Settings      func()
	QuickSwitcher func()
	NewMessage    func()
	NewRoom       func()
	JoinRoom      func()
	ToggleSidebar func()
	// Appearance sets 0 the system's, 1 light, 2 dark.
	Appearance func(mode int)
	// Layout sets the navigation layout (kit.NavLayout); LayoutNow is the
	// one in use, checked in View → Layout.
	Layout    func(l kit.NavLayout)
	LayoutNow kit.NavLayout
	// Room actions apply to the room open.
	MarkRead     func()
	Favourite    func()
	Mute         func(d kit.MuteFor)
	RoomSettings func()
	Leave        func()
}

// MenuBar builds the app's menu bar: the standard menus with the
// platform's roles and shortcuts, and the app's own. update runs an
// action on the UI thread and redraws (Window.Update).
func MenuBar(a Actions, update func(func())) *mygo.Menu {
	do := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { update(fn) }
	}
	item := func(label, accel string, fn func()) *mygo.MenuItem {
		if fn == nil {
			return nil
		}
		return &mygo.MenuItem{Label: label, Accelerator: accel, Click: do(fn)}
	}
	items := func(list ...*mygo.MenuItem) []*mygo.MenuItem {
		out := list[:0]
		for _, it := range list {
			if it != nil {
				out = append(out, it)
			}
		}
		return out
	}

	var bar []*mygo.MenuItem
	mac := Current() == MacOS
	if mac {
		bar = append(bar, &mygo.MenuItem{Role: mygo.RoleAppMenu, Submenu: items(
			&mygo.MenuItem{Role: mygo.RoleAbout},
			mygo.Separator(),
			item("Settings…", "Cmd+,", a.Settings),
			mygo.Separator(),
			&mygo.MenuItem{Role: mygo.RoleServices},
			mygo.Separator(),
			&mygo.MenuItem{Role: mygo.RoleHide},
			&mygo.MenuItem{Role: mygo.RoleHideOthers},
			&mygo.MenuItem{Role: mygo.RoleUnhide},
			mygo.Separator(),
			&mygo.MenuItem{Role: mygo.RoleQuit},
		)})
	}
	file := items(
		item("New Message", "CmdOrCtrl+N", a.NewMessage),
		item("New Room…", "CmdOrCtrl+Shift+N", a.NewRoom),
		item("Join Room…", "CmdOrCtrl+Shift+J", a.JoinRoom),
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleClose},
	)
	if !mac {
		// Settings and Quit live in the File menu off macOS.
		file = append(file, mygo.Separator())
		if s := item("Settings", "Ctrl+,", a.Settings); s != nil {
			file = append(file, s)
		}
		file = append(file, &mygo.MenuItem{Role: mygo.RoleQuit})
	}
	bar = append(bar,
		&mygo.MenuItem{Label: "File", Submenu: file},
		&mygo.MenuItem{Role: mygo.RoleEditMenu},
	)

	view := items(
		item("Quick Switcher", "CmdOrCtrl+K", a.QuickSwitcher),
		item("Toggle Sidebar", "CmdOrCtrl+Shift+S", a.ToggleSidebar),
	)
	if a.Appearance != nil {
		var modes []*mygo.MenuItem
		for i, l := range []string{"Match System", "Light", "Dark"} {
			mode := i
			modes = append(modes, &mygo.MenuItem{Label: l, Type: mygo.MenuItemRadio, Checked: i == 0,
				Click: do(func() { a.Appearance(mode) })})
		}
		view = append(view, mygo.Separator(), &mygo.MenuItem{Label: "Appearance", Submenu: modes})
	}
	if a.Layout != nil {
		var layouts []*mygo.MenuItem
		for i, l := range kit.NavLayouts {
			nl := kit.NavLayout(i)
			layouts = append(layouts, &mygo.MenuItem{Label: l.Name, Type: mygo.MenuItemRadio, Checked: nl == a.LayoutNow,
				ToolTip: l.About, Click: do(func() { a.Layout(nl) })})
		}
		view = append(view, &mygo.MenuItem{Label: "Layout", Submenu: layouts})
	}
	view = append(view, mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleResetZoom},
		&mygo.MenuItem{Role: mygo.RoleZoomIn},
		&mygo.MenuItem{Role: mygo.RoleZoomOut},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleToggleFullScreen},
	)
	bar = append(bar, &mygo.MenuItem{Label: "View", Submenu: view})

	var mutes []*mygo.MenuItem
	if a.Mute != nil {
		for _, ch := range kit.MuteChoices {
			d := ch.For
			label := ch.Label
			if d != 0 {
				label = trimFor(label)
			}
			mutes = append(mutes, &mygo.MenuItem{Label: label, Click: do(func() { a.Mute(d) })})
		}
	}
	room := items(
		// No Escape: it closes dialogs and clears fields everywhere.
		item("Mark as Read", "Shift+Escape", a.MarkRead),
		item("Favourite", "CmdOrCtrl+D", a.Favourite),
	)
	if len(mutes) > 0 {
		room = append(room, &mygo.MenuItem{Label: "Mute", Submenu: mutes})
	}
	room = append(room, mygo.Separator())
	room = append(room, items(
		item("Room Settings…", "", a.RoomSettings),
		item("Leave Room…", "", a.Leave),
	)...)
	bar = append(bar, &mygo.MenuItem{Label: "Room", Submenu: room})

	bar = append(bar, &mygo.MenuItem{Role: mygo.RoleWindowMenu})
	if mac {
		bar = append(bar, &mygo.MenuItem{Role: mygo.RoleHelp, Submenu: []*mygo.MenuItem{
			{Label: "slopement Help", Click: func(*mygo.MenuItem, *mygo.Window) {}},
		}})
	}
	return mygo.NewMenu(bar)
}

// trimFor turns "For 2 hours" into "2 Hours", as menu items read.
func trimFor(s string) string {
	const p = "For "
	if len(s) > len(p) && s[:len(p)] == p {
		s = s[len(p):]
	}
	return s
}

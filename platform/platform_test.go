package platform

import (
	"testing"

	"github.com/egoist/mygo"

	"slopement/kit"
)

func TestWindowChrome(t *testing.T) {
	o := Window(mygo.WindowOptions{Title: "x"})
	switch Current() {
	case MacOS:
		if o.TitleBarStyle != mygo.TitleBarHidden || o.Vibrancy != mygo.VibrancySidebar || o.TrafficLightPosition == nil {
			t.Errorf("macOS chrome = %+v", o)
		}
	case Windows:
		if o.TitleBarStyle != mygo.TitleBarHidden || o.Vibrancy != mygo.VibrancyMica || !o.AutoHideMenuBar {
			t.Errorf("Windows chrome = %+v", o)
		}
	case Linux:
		if o.TitleBarStyle != mygo.TitleBarHidden || !o.AutoHideMenuBar {
			t.Errorf("Linux chrome = %+v", o)
		}
	}
	if o.Title != "x" {
		t.Error("options not kept")
	}
}

func TestMenuBar(t *testing.T) {
	ran := 0
	var muted kit.MuteFor = -1
	m := MenuBar(Actions{
		QuickSwitcher: func() { ran++ },
		Mute:          func(d kit.MuteFor) { muted = d },
		Leave:         func() {},
	}, func(fn func()) { fn() })
	items := m.Items()
	var labels []string
	for _, it := range items {
		l := it.Label
		if it.Role != "" {
			l = string(it.Role) // roles fill in their own labels
		}
		labels = append(labels, l)
	}
	want := []string{"File", "editMenu", "View", "Room", "windowMenu"}
	if Current() == MacOS {
		want = append([]string{"appMenu"}, append(want, "help")...)
	}
	if len(labels) != len(want) {
		t.Fatalf("menus = %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("menu %d = %q, want %q", i, labels[i], want[i])
		}
	}
	// View → Quick Switcher (⌘K) runs its action.
	var view, room *mygo.MenuItem
	for _, it := range items {
		switch it.Label {
		case "View":
			view = it
		case "Room":
			room = it
		}
	}
	if qs := view.Submenu[0]; qs.Label != "Quick Switcher" || qs.Accelerator != "CmdOrCtrl+K" {
		t.Fatalf("first View item = %+v", qs)
	} else {
		qs.Click(qs, nil)
	}
	if ran != 1 {
		t.Error("Quick Switcher did not run")
	}
	// Room → Mute → 2 Hours mutes for two hours.
	for _, it := range room.Submenu {
		if it.Label == "Mute" {
			two := it.Submenu[1]
			if two.Label != "2 hours" {
				t.Fatalf("mute item = %q", two.Label)
			}
			two.Click(two, nil)
		}
	}
	if muted != kit.MuteChoices[1].For {
		t.Errorf("muted = %v", muted)
	}
}

// View → Layout lists the three layouts, the one in use checked.
func TestLayoutMenu(t *testing.T) {
	var picked kit.NavLayout = -1
	m := MenuBar(Actions{Layout: func(l kit.NavLayout) { picked = l }, LayoutNow: kit.NavTabs}, func(fn func()) { fn() })
	for _, it := range m.Items() {
		if it.Label != "View" {
			continue
		}
		for _, sub := range it.Submenu {
			if sub.Label != "Layout" {
				continue
			}
			if len(sub.Submenu) != 3 || !sub.Submenu[1].Checked || sub.Submenu[0].Checked {
				t.Fatalf("layout items = %+v", sub.Submenu)
			}
			inbox := sub.Submenu[2]
			inbox.Click(inbox, nil)
			if picked != kit.NavInbox {
				t.Fatalf("picked = %v", picked)
			}
			return
		}
	}
	t.Fatal("no View → Layout")
}

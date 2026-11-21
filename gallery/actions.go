package gallery

import (
	"slopement/kit"
	"slopement/platform"
)

// Actions are the menu bar's actions on the gallery's demo state: the
// open room is the Desktop mock's.
func (a *App) Actions() platform.Actions {
	room := func() *kit.RoomMenuState { return a.roomStates[a.room] }
	return platform.Actions{
		QuickSwitcher: func() { a.switcher.Toggle() },
		NewMessage:    func() { a.did = "New message" },
		NewRoom:       func() { a.did = "New room" },
		JoinRoom:      func() { a.did = "Join room" },
		Settings:      func() { a.did = "Settings" },
		ToggleSidebar: func() { a.did = "Toggle sidebar" },
		Appearance: func(mode int) {
			a.themeMode = []int{1, 2, 3}[mode] // Auto, Light, Dark
		},
		Layout:    func(l kit.NavLayout) { a.navLayout = l },
		LayoutNow: a.navLayout,
		MarkRead:     func() { a.applyRoom(a.room, kit.RoomAction{MarkRead: true}) },
		Favourite:    func() { a.applyRoom(a.room, kit.RoomAction{ToggleFav: true}) },
		Mute:         func(d kit.MuteFor) { a.applyRoom(a.room, kit.RoomAction{Mute: &d}) },
		RoomSettings: func() { a.did = "Room settings" },
		Leave: func() {
			if room() != nil {
				a.dialog.Open = true
			}
		},
	}
}

package app

import (
	"github.com/egoist/mygo"

	"slopement/model"
)

// Desktop notifications: the source tells of messages that may notify
// (model.Notifier); the app shows one unless the user turned them off, the
// window is in front with that very room open, or it is a mention of them
// when mentions are off. One notification per room (a later one replaces
// the last), grouped by room; a click opens the room.

// showNotification puts a notification on screen: the system's, off the UI
// thread (the first asks the user for permission and waits for the answer).
// A variable, so tests see what would be shown.
var showNotification = func(n model.Notice, silent bool) {
	if !mygo.NotificationsSupported() {
		return
	}
	go func() {
		_ = mygo.NewNotification(mygo.NotificationOptions{
			ID: n.Room, Group: n.Room, Title: n.Title, Body: n.Body, Silent: silent,
		}).Show()
	}()
}

// listenForNotices has the source tell the app of messages that may notify.
func (a *App) listenForNotices() {
	if ns, ok := a.src.(model.Notifier); ok {
		ns.OnNotify(a.notice)
	}
}

// notice decides on one message that may notify. UI thread.
func (a *App) notice(n model.Notice) {
	switch {
	case a.detached:
		return
	case n.Mention && !n.DM && !a.prefs.notifyMentions:
		return
	case a.focused && a.view == viewRoom && a.room == n.Room:
		return // it is on screen in front of them
	}
	if a.prefs.notifyDesktop {
		showNotification(n, true) // silent: the sound is the app's own, below
	}
	if a.prefs.notifySound {
		playChime() // with or without the banner: "Sound" is its own switch
	}
}

// OpenFromNotification opens the room a notification was about (its ID):
// the host calls it when the user clicks one.
func (a *App) OpenFromNotification(id string) {
	if _, ok := a.src.Room(id); ok {
		a.community = ""
		a.open(id)
	}
}

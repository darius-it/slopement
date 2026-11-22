package app

import (
	"testing"

	"slopement/model"
)

// What gets shown: not with notifications off, not for the room that is on
// screen in a window in front, not for a mention when mentions are off —
// and a chat message, or a mention, otherwise, quietly if sound is off.
func TestWhichNoticesShow(t *testing.T) {
	var shown []model.Notice
	var silent []bool
	old := showNotification
	showNotification = func(n model.Notice, s bool) { shown, silent = append(shown, n), append(silent, s) }
	defer func() { showNotification = old }()

	a := testApp()
	a.open("!crit")
	dm := model.Notice{Room: "!crit", Title: "Kai", Body: "hi", DM: true}
	mention := model.Notice{Room: "!other", Title: "Club", Body: "Kai: @you", Mention: true}

	a.focused = true
	a.notice(dm) // the room in view, window in front
	if len(shown) != 0 {
		t.Fatal("notified for the room on screen")
	}
	a.focused = false
	a.notice(dm) // window behind
	if len(shown) != 1 {
		t.Fatal("no notification with the window behind")
	}
	a.focused = true
	a.notice(mention) // another room, window in front
	if len(shown) != 2 {
		t.Fatal("no notification for another room")
	}

	a.prefs.notifyMentions = false
	a.notice(mention)
	if len(shown) != 2 {
		t.Fatal("a mention notified with mentions off")
	}
	a.notice(model.Notice{Room: "!x", DM: true, Mention: true}) // a chat is not a mention
	if len(shown) != 3 {
		t.Fatal("a chat message did not notify with mentions off")
	}

	a.prefs.notifySound = false
	a.focused = false
	a.notice(dm)
	if !silent[len(silent)-1] {
		t.Fatal("sound off, but the notification was loud")
	}

	a.prefs.notifyDesktop = false
	before := len(shown)
	a.notice(dm)
	if len(shown) != before {
		t.Fatal("notified with desktop notifications off")
	}
}

// A click on a notification opens its room.
func TestNotificationClickOpensTheRoom(t *testing.T) {
	a := testApp()
	room, _ := longTimeline(a, 2)
	a.OpenFromNotification(room.ID)
	if a.room != room.ID || a.view != viewRoom {
		t.Fatalf("open %q view %v", a.room, a.view)
	}
}

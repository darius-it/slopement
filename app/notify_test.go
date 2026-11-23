package app

import (
	"testing"

	"slopement/model"
)

// chimes counts the sounds a test would have heard.
var chimes int

// What gets shown: not with notifications off, not for the room that is on
// screen in a window in front, not for a mention when mentions are off —
// and a chat message, or a mention, otherwise, quietly if sound is off.
func TestWhichNoticesShow(t *testing.T) {
	var shown []model.Notice
	var silent []bool
	old := showNotification
	showNotification = func(n model.Notice, s bool) { shown, silent = append(shown, n), append(silent, s) }
	oldChime := playChime
	playChime = func() { chimes++ }
	defer func() { showNotification, playChime = old, oldChime }()

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

	// The sound is the app's own chime; the banner itself is always silent.
	a.focused = false
	chimes = 0
	a.prefs.notifySound = true
	a.notice(dm)
	if chimes != 1 || !silent[len(silent)-1] {
		t.Fatalf("sound on: %d chimes, banner silent=%v", chimes, silent[len(silent)-1])
	}
	a.prefs.notifySound = false
	a.notice(dm)
	if chimes != 1 {
		t.Fatal("chimed with sound off")
	}

	// Banners off, sound on: the chime without a banner.
	a.prefs.notifyDesktop = false
	a.prefs.notifySound = true
	before := len(shown)
	a.notice(dm)
	if len(shown) != before {
		t.Fatal("notified with desktop notifications off")
	}
	if chimes != 2 {
		t.Fatal("no chime with banners off and sound on")
	}
	// Nothing at all for the room in front.
	a.focused = true
	a.open("!crit")
	a.notice(dm)
	if chimes != 2 {
		t.Fatal("chimed for the room on screen")
	}
}

// The chime is a real, short WAV that the audio package decodes.
func TestChimeIsAWAV(t *testing.T) {
	w := chimeWAV()
	if string(w[:4]) != "RIFF" || string(w[8:12]) != "WAVE" || len(w) < 8000 || len(w) > 100000 {
		t.Fatalf("not a short WAV: %d bytes", len(w))
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

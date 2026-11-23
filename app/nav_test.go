package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

func navApp(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 800, ui.TesterOptions{Dark: true})
	settle(tt)
	return a, tt
}

// Back and forward walk the places visited, as a browser does: a new place
// after going back drops the ones ahead.
func TestBackAndForward(t *testing.T) {
	a, tt := navApp(t)
	a.open("!crit")
	settle(tt)
	a.open("!dm-alex")
	settle(tt)
	a.view = viewPeople
	settle(tt)
	if a.canGoForward() || !a.canGoBack() {
		t.Fatalf("at the end: back %v forward %v", a.canGoBack(), a.canGoForward())
	}

	tt.Key(0, ui.KeyBack) // a mouse's back button
	settle(tt)
	if a.view != viewRoom || a.room != "!dm-alex" {
		t.Fatalf("back #1: view %v room %q", a.view, a.room)
	}
	tt.Key(0, ui.KeyBack)
	settle(tt)
	if a.room != "!crit" {
		t.Fatalf("back #2: room %q", a.room)
	}
	tt.Key(0, ui.KeyForward)
	settle(tt)
	if a.room != "!dm-alex" || a.view != viewRoom {
		t.Fatalf("forward: view %v room %q", a.view, a.room)
	}

	// A new place now: what was ahead (People) is gone.
	a.open("!crit")
	settle(tt)
	if a.canGoForward() {
		t.Fatal("the stops ahead were kept after a new place")
	}
	tt.Key(0, ui.KeyBack)
	settle(tt)
	if a.room != "!dm-alex" {
		t.Fatalf("back after the new place: room %q", a.room)
	}
}

// The top bar has Back and Forward arrows, disabled at the ends of the history.
func TestTopBarArrows(t *testing.T) {
	a, tt := navApp(t)
	tt.SetTitleBar(ui.TitleBar{Height: 32, Left: 80})
	a.open("!crit")
	settle(tt)
	a.open("!dm-alex")
	settle(tt)
	if err := tt.Click("Back"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.room != "!crit" {
		t.Fatalf("Back went to %q", a.room)
	}
	if err := tt.Click("Forward"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.room != "!dm-alex" {
		t.Fatalf("Forward went to %q", a.room)
	}
	snap(t, "app-toparrows", tt)
}

// Coming back to a chat from a space (Chats on the rail), or from Settings,
// asks for the keyboard in that chat's composer.
func TestReturningToAChatFocusesItsComposer(t *testing.T) {
	a, tt := navApp(t)
	a.open("!crit")
	settle(tt)
	cms := a.communities()
	if len(cms) == 0 {
		t.Skip("needs a community")
	}
	a.community = cms[0].ID // into a space
	settle(tt)
	a.focusRoom = ""
	a.community = "" // Chats again: the last open chat shows
	a.navTrack()
	if a.focusRoom != "!crit" {
		t.Fatalf("no focus asked for the chat after leaving the space: %q", a.focusRoom)
	}

	a.view = viewSettings
	settle(tt)
	a.focusRoom = ""
	a.view = viewRoom
	a.navTrack()
	if a.focusRoom != "!crit" {
		t.Fatalf("no focus asked for the chat after Settings: %q", a.focusRoom)
	}
	// Typed now, it goes to the composer.
	settle(tt)
	tt.Type("hi")
	settle(tt)
	if got := a.draft("!crit").Text; got != "hi" {
		t.Fatalf("the draft is %q", got)
	}
}

// A room that is gone is passed over, and the ends of the history do nothing.
func TestBackSkipsGoneRooms(t *testing.T) {
	a, tt := navApp(t)
	a.open("!crit")
	settle(tt)
	a.trail = navHistory{stops: []place{{view: viewRoom, room: "!gone"}, {view: viewRoom, room: "!crit"}}, at: 1}
	a.open("!dm-alex")
	settle(tt)
	tt.Key(0, ui.KeyBack) // to !crit
	settle(tt)
	tt.Key(0, ui.KeyBack) // !gone is skipped: nowhere further back
	settle(tt)
	if a.room != "!crit" {
		t.Fatalf("room = %q", a.room)
	}
	tt.Key(0, ui.KeyForward)
	tt.Key(0, ui.KeyForward) // the end
	settle(tt)
	if a.room != "!dm-alex" {
		t.Fatalf("room = %q", a.room)
	}
}

// Opening a room puts the keyboard in its composer (not on phones).
func TestOpeningARoomFocusesItsComposer(t *testing.T) {
	a, tt := navApp(t)
	a.open("!crit")
	settle(tt)
	if a.focusRoom != "" {
		t.Fatalf("the focus was not taken: %q", a.focusRoom)
	}
	tt.Type("hello")
	settle(tt)
	if got := a.draft("!crit").Text; got != "hello" {
		t.Fatalf("typing went elsewhere: the draft is %q", got)
	}
}

// People lists the home server's own people; other servers and bridges'
// puppets are one press away.
func TestOutsideHome(t *testing.T) {
	a, _ := navApp(t)
	_, home, _ := strings.Cut(a.src.Me().ID, ":")
	for _, tc := range []struct {
		local, server string
		outside       bool
	}{
		{"ada", home, false},
		{"Marathon", home, false},
		{"signalfan", home, false}, // a name, not a bridge's "signal_"
		{"ircfan", home, false},
		{"slackerjack", home, false},
		{"discordfan", home, false},
		{"_discord_123", home, true},
		{"whatsapp_4915100", home, true},
		{"telegram_77", home, true},
		{"signal_ab", home, true},
		{"bridgebot", home, true},
		{"_slackpuppet", home, true},
		{"irc_nick", home, true},
		{"googlechat_abc", home, true},
		{"alice", "other.example", true},
		{"ada", home + ".example", true},
	} {
		if got := a.outsideHome("@" + tc.local + ":" + tc.server); got != tc.outside {
			t.Errorf("outsideHome(@%s:%s) = %v, want %v", tc.local, tc.server, got, tc.outside)
		}
	}
}

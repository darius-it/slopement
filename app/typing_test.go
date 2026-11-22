package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

type typist struct {
	model.Source
	calls []string // "room:true", "room:false"
}

func (s *typist) SetTyping(room string, typing bool) {
	v := "false"
	if typing {
		v = "true"
	}
	s.calls = append(s.calls, room+":"+v)
}

func typingApp(t *testing.T) (*App, *typist, *ui.Tester, model.Room) {
	t.Helper()
	oldIdle, oldRefresh := typingIdle, typingRefresh
	typingIdle, typingRefresh = 80*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { typingIdle, typingRefresh = oldIdle, oldRefresh })
	a := testApp()
	room := a.src.Rooms()[0]
	ty := &typist{Source: a.src}
	a.src = ty
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	return a, ty, tt, room
}

// Writing in the composer tells the room the user is typing — once, then
// again after a while if they go on — and that they stopped, a few seconds
// after the last change; emptying the field stops it at once.
func TestTypingNoticesAreSent(t *testing.T) {
	a, ty, tt, room := typingApp(t)
	if len(ty.calls) != 0 {
		t.Fatalf("typing told with nothing typed: %v", ty.calls)
	}
	a.draft(room.ID).Text = "hel"
	settle(tt)
	if len(ty.calls) != 1 || ty.calls[0] != room.ID+":true" {
		t.Fatalf("after typing: %v", ty.calls)
	}
	a.draft(room.ID).Text = "hello"
	settle(tt)
	if len(ty.calls) != 1 {
		t.Fatalf("every keystroke is told: %v", ty.calls)
	}
	// Still typing, never pausing long enough to stop, past the refresh
	// time: told again, not just once.
	for i := 0; i < 6; i++ {
		time.Sleep(50 * time.Millisecond)
		a.draft(room.ID).Text = "hello there " + string(rune('a'+i))
		settle(tt)
	}
	if len(ty.calls) != 2 || ty.calls[1] != room.ID+":true" {
		t.Fatalf("a long stretch of typing: %v", ty.calls)
	}
	// Then quiet: stopped.
	time.Sleep(120 * time.Millisecond)
	settle(tt)
	if got := ty.calls[len(ty.calls)-1]; got != room.ID+":false" {
		t.Fatalf("after going quiet: %v", ty.calls)
	}

	// Emptied (or sent): stopped at once.
	a.draft(room.ID).Text = "again"
	settle(tt)
	n := len(ty.calls)
	a.draft(room.ID).Text = ""
	settle(tt)
	if len(ty.calls) != n+1 || ty.calls[n] != room.ID+":false" {
		t.Fatalf("emptying the field: %v", ty.calls[n-1:])
	}
}

// Leaving the room stops it; the setting off sends nothing, and stops what
// was going.
func TestTypingStopsWhenLeavingOrTurnedOff(t *testing.T) {
	a, ty, tt, room := typingApp(t)
	a.draft(room.ID).Text = "writing"
	settle(tt)
	other := a.src.Rooms()[1].ID
	a.open(other)
	settle(tt)
	if got := ty.calls[len(ty.calls)-1]; got != room.ID+":false" {
		t.Fatalf("leaving the room: %v", ty.calls)
	}

	a.open(room.ID)
	a.prefs.typing = false
	n := len(ty.calls)
	a.draft(room.ID).Text = "secret words"
	settle(tt)
	if len(ty.calls) != n {
		t.Fatalf("typing told with the setting off: %v", ty.calls[n:])
	}

	// Turned off mid-way: what was going is stopped.
	a.prefs.typing = true
	a.draft(room.ID).Text = "going again"
	settle(tt)
	if got := ty.calls[len(ty.calls)-1]; got != room.ID+":true" {
		t.Fatalf("typing again: %v", ty.calls)
	}
	a.prefs.typing = false
	settle(tt)
	if got := ty.calls[len(ty.calls)-1]; got != room.ID+":false" {
		t.Fatalf("turning it off did not stop it: %v", ty.calls)
	}
}

// The setting is a real switch again, not a "Soon".
func TestTypingSettingIsASwitch(t *testing.T) {
	a := testApp()
	a.view, a.settings = viewSettings, "Privacy"
	tt := ui.NewTester(a.View, 1100, 900)
	settle(tt)
	if !tt.HasText("Send typing notifications") {
		t.Fatal("no typing setting")
	}
	if _, soon := tt.Find("Send typing notifications, coming soon"); soon {
		t.Fatal("the typing setting is still marked as coming soon")
	}
	if _, ok := tt.Find("Send typing notifications"); !ok {
		t.Fatal("the switch is not there")
	}
}

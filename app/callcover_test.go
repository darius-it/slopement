package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// The room's timeline lays out rows whose boxes reach up over the call page (a
// list builds a few rows past its edge); a web view counts every element built
// after it as covering it, by its whole box, clipped or not, so unless the call
// area is ordered above the timeline those rows cut holes in the page: an
// invisible strip across its bottom that takes no clicks. This pins both halves:
// the rows do overlap the page's box, and the call area is ordered above.
func TestCallAreaIsOrderedAboveTheTimeline(t *testing.T) {
	if callAreaZIndex <= 0 {
		t.Fatal("the call area is not ordered above its siblings")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 1000, ui.TesterOptions{Dark: true})
	settle(tt)
	a.open("!crit")
	a.callSess = &callSession{room: "!crit", status: "Joining…"}
	a.inCall, a.callHeight = "!crit", 440
	settle(tt)
	grip, ok := tt.Find("Resize the call")
	if !ok {
		t.Fatal("no grip under the call")
	}
	top, bottom := grip.Y-440, grip.Y
	overlapping := 0
	for _, acc := range tt.AccessAll() {
		b := acc.Bounds
		if b.X >= 340 && b.W >= 800 && b.H > 0 && acc.Name != "" && b.Y > top && b.Y < bottom-2 {
			overlapping++
		}
	}
	if overlapping == 0 {
		t.Skip("the timeline no longer lays out rows over the call box; the z-order may not be needed")
	}
	t.Logf("%d timeline rows overlap the call box; the call area is ordered above them", overlapping)
}

// The call page takes every press inside its box: the grip under it (which
// resizes the page) must not reach up into it. Probed by pressing at points down
// the box's bottom edge and dragging: only the grip itself may move the height.
func TestGripDoesNotReachIntoTheCallPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	tt := ui.NewTesterWith(a.View, 1300, 1000, ui.TesterOptions{Dark: true})
	settle(tt)
	a.open("!crit")
	settle(tt)
	// A session with no page yet shows the same box the page does.
	a.callSess = &callSession{room: "!crit", status: "Joining…"}
	a.inCall = "!crit"
	a.callHeight = 440
	settle(tt)

	grip, ok := tt.Find("Resize the call")
	if !ok {
		t.Fatal("no grip under the call")
	}
	x := grip.X + grip.W/2
	t.Logf("grip box: y=%.0f h=%.0f", grip.Y, grip.H)
	for _, above := range []float32{2, 6, 10, 16, 24, 32, 44, 60, 90} {
		y := grip.Y - above // above the grip: inside the call box
		a.callHeight = 440
		tt.Move(x, y)
		cur := tt.Cursor()
		tt.Press(x, y)
		for i := 1; i <= 5; i++ {
			tt.Move(x, y+float32(i)*8)
		}
		tt.Release(x, y+40)
		settle(tt)
		moved := a.callHeight != 440
		t.Logf("%.0f px above the grip: cursor %v, height changed %v", above, cur, moved)
		if moved || cur == ui.CursorResizeNS {
			t.Errorf("the grip takes presses %.0f px above itself, inside the call box", above)
		}
	}
}

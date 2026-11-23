package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// In a call the call page sits above the room in a box of its own, with a thin
// grip under it, dragged to make the page bigger or smaller.
func TestCallPageHeightIsDraggable(t *testing.T) {
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
	r, _ := a.src.Room("!crit")
	a.joinCall(r)
	settle(tt)
	if a.callHeight != 440 {
		t.Fatalf("the call page starts at %v, want 440", a.callHeight)
	}

	grip := func() (x, y float32) {
		for _, acc := range tt.AccessAll() {
			if acc.Name == "Resize the call" {
				return acc.Bounds.X + acc.Bounds.W/2, acc.Bounds.Y + acc.Bounds.H/2
			}
		}
		t.Fatal("no grip under the call")
		return
	}
	drag := func(dy float32) {
		x, y := grip()
		tt.Press(x, y)
		const steps = 10
		for i := 1; i <= steps; i++ {
			tt.Move(x, y+dy*float32(i)/steps)
		}
		tt.Release(x, y+dy)
		settle(tt)
	}
	before := a.callHeight
	drag(120)
	if a.callHeight < before+60 {
		t.Fatalf("the page did not grow with the drag: %v after %v", a.callHeight, before)
	}
	big := a.callHeight
	drag(-250)
	if a.callHeight > big-100 {
		t.Fatalf("the page did not shrink: %v after %v", a.callHeight, big)
	}
	if a.callHeight < 120 {
		t.Fatalf("the page went under its least: %v", a.callHeight)
	}
}

package app

import (
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A rail item's tooltip goes out to its right, level with it — not above,
// where it would sit on the item above.
func TestRailTooltipBesideItem(t *testing.T) {
	a := testApp()
	name := "Add a space"
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)

	var item ui.Rect
	for _, info := range tt.AccessAll() {
		if info.Name == name { // a menu button, whatever its role is called
			item = info.Bounds
		}
	}
	if item.W == 0 {
		t.Fatalf("no rail item named %q", name)
	}
	pixel := func(x, y float32) color.Color { return tt.Image().At(int(x), int(y)) }
	right := [2]float32{item.X + item.W + 14, item.Y + item.H/2} // where a tip to the right sits
	above := [2]float32{item.X + item.W/2, item.Y - 10}          // where a tip above it would
	beforeRight, beforeAbove := pixel(right[0], right[1]), pixel(above[0], above[1])

	tt.Move(item.X+item.W/2, item.Y+item.H/2)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	snap(t, "rail-tooltip", tt)

	if pixel(right[0], right[1]) == beforeRight {
		t.Fatal("nothing shows to the right of the item after resting on it")
	}
	if pixel(above[0], above[1]) != beforeAbove {
		t.Fatal("something shows above the item: the tooltip is not to its side")
	}
}

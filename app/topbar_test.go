package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The place's name is in the middle of the window, whichever side the window's
// controls are on.
func TestTopBarNameIsCentred(t *testing.T) {
	for _, c := range []struct {
		os  string
		bar ui.TitleBar
	}{
		{"windows", ui.TitleBar{Height: 32, Right: 138}},
		{"macos", ui.TitleBar{Height: 32, Left: 80}},
	} {
		a := testApp()
		tt := ui.NewTester(a.View, 1100, 700)
		tt.SetTitleBar(c.bar)
		settle(tt)
		name := a.placeName()
		r, ok := tt.Find(name)
		if !ok || r.Y > 32 {
			t.Fatalf("%s: %q not found in the top bar (%+v)", c.os, name, r)
		}
		if mid := r.X + r.W/2; mid < 549 || mid > 551 {
			t.Errorf("%s: the name's middle is at %.1f, want 550", c.os, mid)
		}
		snap(t, "topbar-"+c.os, tt)
	}
}

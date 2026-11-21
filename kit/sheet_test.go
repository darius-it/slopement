package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSheetSpring opens a sheet, closes it mid-flight and catches it
// again: the spring turns around without jumping, and once shut the sheet
// is gone.
func TestSheetSpring(t *testing.T) {
	st := &SheetState{}
	view := func(c *ui.Context) {
		Apply(c)
		Sheet(c, st, "", func() { ui.Text(c, "Sheet body") })
	}
	tt := ui.NewTester(view, 390, 844)
	top := func() float32 {
		r, ok := tt.Find("Sheet body")
		if !ok {
			return -1
		}
		return r.Y
	}
	tt.Frame()
	if top() >= 0 {
		t.Fatal("shown while closed")
	}
	st.Open = true
	tt.Frame()
	start := top()
	tt.Advance(1200 * time.Millisecond)
	tt.Frame()
	rest := top()
	if !(start > rest && rest > 600) {
		t.Fatalf("it should rise from below to rest at the bottom: start %v rest %v", start, rest)
	}
	// Close it, and catch it again partway down.
	st.Open = false
	tt.Frame()
	tt.Advance(90 * time.Millisecond)
	tt.Frame()
	mid := top()
	if !(mid > rest) {
		t.Fatalf("not on its way down: %v", mid)
	}
	st.Open = true
	tt.Frame()
	tt.Advance(16 * time.Millisecond)
	tt.Frame()
	// Interruptible: it carries on a little, then turns; no jump back.
	if back := top(); back < rest-1 || back > mid+80 {
		t.Fatalf("jumped when caught: mid %v then %v", mid, back)
	}
	tt.Advance(1500 * time.Millisecond)
	tt.Frame()
	if r := top(); r < rest-1 || r > rest+1 {
		t.Fatalf("did not settle back: %v, want %v", r, rest)
	}
	// Shut: gone once it has arrived.
	st.Open = false
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.Frame()
	if top() >= 0 || st.alive {
		t.Fatal("still built after closing")
	}
}

// TestViewerClickOutside: a click on the picture keeps the media viewer
// open; one beside it closes it.
func TestViewerClickOutside(t *testing.T) {
	open, at := true, 0
	items := []MediaSpec{{ID: "m", Kind: MediaImage, W: 400, H: 300, Label: "Photo"}}
	view := func(c *ui.Context) {
		Apply(c)
		MediaViewer(c, &open, items, &at)
	}
	tt := ui.NewTester(view, 1000, 800)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	click := func(x, y float32) {
		tt.Move(x, y)
		tt.Frame()
		tt.Press(x, y)
		tt.Frame()
		tt.Release(x, y)
		tt.Frame()
	}
	click(500, 450) // the middle: the picture
	tt.Frame()
	if !open {
		t.Fatal("a click on the picture closed the viewer")
	}
	click(40, 450) // beside it
	tt.Frame()
	if open {
		t.Fatal("a click beside the picture didn't close the viewer")
	}
}

// TestSheetDrag drags a sheet by its body, as Control Center: a short,
// slow drag bounces back; a slow drag past half way closes it, with no
// flick needed; and a sheet caught mid-flight stays under the finger.
func TestSheetDrag(t *testing.T) {
	st := &SheetState{Open: true}
	view := func(c *ui.Context) {
		Apply(c)
		Sheet(c, st, "", func() {
			ui.Text(c, "Sheet body")
			ui.Box(c).Height(260)
		})
	}
	tt := ui.NewTester(view, 390, 844)
	top := func() float32 {
		r, ok := tt.Find("Sheet body")
		if !ok {
			return -1
		}
		return r.Y
	}
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	rest := top()
	// slowDrag moves the finger by dy in small, slow steps, then pauses
	// before letting go, so there's no fling.
	slowDrag := func(dy float32) {
		x, y := float32(195), rest+4
		tt.FingerDown(x, y)
		tt.Frame()
		for moved := float32(0); moved < dy; moved += 8 {
			tt.Advance(40 * time.Millisecond)
			tt.FingerMove(x, y+moved+8)
			tt.Frame()
		}
		tt.Advance(300 * time.Millisecond)
		tt.FingerMove(x, y+dy)
		tt.Frame()
		tt.FingerUp(x, y+dy)
		tt.Frame()
	}
	slowDrag(60)
	if !st.Open {
		t.Fatal("a short drag closed it")
	}
	tt.Advance(time.Second)
	tt.Frame()
	if r := top(); r < rest-1 || r > rest+1 {
		t.Fatalf("did not bounce back: %v, want %v", r, rest)
	}
	slowDrag(st.h/2 + 30)
	if st.Open {
		t.Fatalf("a slow drag past half way didn't close it (h %v)", st.h)
	}
	// Catch it mid-flight: reopen, and put a finger on it on the way up.
	tt.Advance(time.Second)
	tt.Frame()
	st.Open = true
	tt.Frame()
	tt.Advance(70 * time.Millisecond)
	tt.Frame()
	mid := top()
	tt.FingerDown(195, mid+4)
	tt.Frame()
	tt.Advance(16 * time.Millisecond)
	tt.FingerMove(195, mid+6)
	tt.Frame()
	if got := top(); got < mid-12 || got > mid+12 {
		t.Fatalf("jumped when caught: %v then %v", mid, got)
	}
	tt.FingerUp(195, mid+6)
	tt.Frame()
}

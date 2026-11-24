package app

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// near says a pixel is the color c, to within a few levels.
func near(img *image.RGBA, x, y int, c ui.Color) bool {
	o := img.PixOffset(x, y)
	d := func(a uint8, b uint8) float64 { return math.Abs(float64(a) - float64(b)) }
	return d(img.Pix[o], c.R)+d(img.Pix[o+1], c.G)+d(img.Pix[o+2], c.B) < 12
}

// roomEdge is where the room's page starts, from the left, in window DIPs: its
// header band is the wallpaper's color, the lists' the page's. 400 when it is
// gone (no wallpaper in the band).
func roomEdge(tt *ui.Tester) float32 {
	img := tt.Image()
	wp := kit.Light.Wallpaper
	for x := 0; x < img.Bounds().Dx(); x++ {
		if near(img, x, 4, wp) {
			return float32(x)
		}
	}
	return float32(img.Bounds().Dx())
}

// backFrames steps a back from a room in 16 ms frames and returns, for each, the
// room's edge and the lists' Search field's left edge.
func backFrames(t *testing.T, back func(a *App)) (room, lists []float32) {
	t.Helper()
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	back(a)
	for i := 0; i < 60; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		room = append(room, roomEdge(tt))
		if r, ok := tt.Find("Settings"); ok { // the tab bar is part of the lists' page
			lists = append(lists, r.X)
		}
	}
	return room, lists
}

// Going back from a room is one smooth motion: the room's edge moves out to
// the right and never turns back.
func TestPhoneBackIsOneSmoothMotion(t *testing.T) {
	room, _ := backFrames(t, func(a *App) { a.pushed = false })
	t.Logf("room edge by frame: %v", room)
	for i := 1; i < len(room); i++ {
		if room[i] < room[i-1]-0.5 {
			t.Errorf("the room turned back at frame %d: %v then %v", i, room[i-1], room[i])
			return
		}
	}
	if room[len(room)-1] < 399 {
		t.Errorf("the room did not leave: edge at %v", room[len(room)-1])
	}
}

// A swipe back that is let go carries on from where the finger left the room:
// mygo's iOS edge swipe starts the slide out from its very start (the finger's
// place is lost as Back is called), which made the room jump back to fill the
// screen and slide out again. The router's Back with nothing recorded is what
// that gives; the room must begin where it was (200 of 400 here) and only go
// on, the lists under it beginning where the finger had them.
func TestPhoneSwipeBackContinuesFromTheFinger(t *testing.T) {
	defer func(was bool) { swipeBackRestarts = was }(swipeBackRestarts)
	swipeBackRestarts = true
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)

	a.roomX, a.roomXPrev = 200, 200 // the finger had the room 200 in
	a.phoneNav.Back()               // the router's own Back, as the gesture's commit calls it
	var room, lists []float32
	for i := 0; i < 24; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		room = append(room, roomEdge(tt))
		if r, ok := tt.Find("Settings"); ok {
			lists = append(lists, r.X)
		}
	}
	t.Logf("room edge: %v", room)
	t.Logf("lists' Settings x: %v", lists)
	if room[0] < 190 {
		t.Errorf("the room jumped back to %v as the finger let go at 200", room[0])
	}
	for i := 1; i < len(room); i++ {
		if room[i] < room[i-1]-0.5 {
			t.Errorf("the room turned back at frame %d: %v then %v", i, room[i-1], room[i])
			break
		}
	}
	if room[len(room)-1] < 399 {
		t.Errorf("the room did not leave: %v", room[len(room)-1])
	}
	// The lists begin 30% of the way back from rest (300): 240 for a room at 200.
	if len(lists) == 0 || lists[0] < 230 || lists[0] > 262 {
		t.Errorf("the lists began at %v, want about 240", lists)
	}
	for i := 1; i < len(lists); i++ {
		if lists[i] < lists[i-1]-0.5 || lists[i] > 300.5 {
			t.Errorf("the lists did not settle once: frame %d %v then %v", i, lists[i-1], lists[i])
			break
		}
	}
}

// A transition that looks over is over: the router builds both its pages in
// every frame while one goes on, so a tail of tiny motion after the slide (as a
// spring with a bounce had, for about a second) is a second of dropped frames.
// After a push and a back, no frame is asked for 0.6 s later.
func TestPhoneSlideAsksForNoFramesOnceDone(t *testing.T) {
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	a := testApp()
	a.pushed = false
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	for _, step := range []struct {
		name string
		do   func()
	}{
		{"push", func() { a.open("!dm-alex") }},
		{"pop", func() { a.pushed = false }},
	} {
		step.do()
		for i := 0; i < 38; i++ { // 608 ms
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
		}
		if tt.FramePending() {
			t.Errorf("%s: still asking for frames 608 ms after it began", step.name)
		}
	}
}

// The lists under the room come in once: their Search field moves to its place
// and stays, without going past it and coming back.
func TestPhoneListSettlesWithoutOvershoot(t *testing.T) {
	_, lists := backFrames(t, func(a *App) { a.pushed = false })
	t.Logf("lists' Search x by frame: %v", lists)
	if len(lists) < 5 {
		t.Fatalf("the lists were seen in %d frames only", len(lists))
	}
	rest := lists[len(lists)-1]
	for i, x := range lists {
		if x > rest+0.75 {
			t.Errorf("the lists went past their place at frame %d: %v, rest %v", i, x, rest)
			return
		}
	}
}

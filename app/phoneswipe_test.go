package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// swipe drags a finger from (x, y) sideways by dx, in steps with the clock
// moving, and lets go.
func swipe(tt *ui.Tester, x, y, dx float32) {
	tt.FingerDown(x, y)
	tt.Advance(40 * time.Millisecond)
	tt.Frame()
	const steps = 8
	for i := 1; i <= steps; i++ {
		tt.FingerMove(x+dx*float32(i)/steps, y)
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.FingerUp(x+dx, y)
	tt.Frame()
}

// The card of a message's menu goes under the message; where it would not fit
// (a message near the bottom, the keyboard up), both go up together; a message
// too tall for the screen is cut to leave room for the card.
func TestMessageMenuLayout(t *testing.T) {
	const h, safeTop, safeBottom = 800, 50, 34
	const menuH = 4*msgMenuRowH + 3
	botLimit := float32(h - safeBottom - msgMenuMargin)

	// Room below: the message stays, the card is under it.
	top, bh, mt := msgMenuLayout(ui.Rect{Y: 200, H: 60}, menuH, h, safeTop, safeBottom)
	if top != 200 || bh != 60 || mt != 200+60+msgMenuGap {
		t.Errorf("room below: top %v height %v card %v", top, bh, mt)
	}
	// Near the bottom: both move up so the card ends above the bars.
	top, bh, mt = msgMenuLayout(ui.Rect{Y: 700, H: 60}, menuH, h, safeTop, safeBottom)
	if top >= 700 || mt+menuH != botLimit || bh != 60 {
		t.Errorf("near the bottom: top %v height %v card %v (ends %v, want %v)", top, bh, mt, mt+menuH, botLimit)
	}
	// Taller than the screen allows: cut, and still the card fits.
	top, bh, mt = msgMenuLayout(ui.Rect{Y: 100, H: 2000}, menuH, h, safeTop, safeBottom)
	if bh >= 2000 || top < safeTop || mt+menuH > botLimit+0.01 {
		t.Errorf("tall: top %v height %v card %v", top, bh, mt)
	}
}

// dragY drags a finger from (x, y) down by dy in 8 steps of step each, and lets
// go: a short step is a flick, a long one a slow drag.
func dragY(tt *ui.Tester, x, y, dy float32, step time.Duration) {
	tt.FingerDown(x, y)
	tt.Advance(40 * time.Millisecond)
	tt.Frame()
	const steps = 8
	for i := 1; i <= steps; i++ {
		tt.FingerMove(x, y+dy*float32(i)/steps)
		tt.Advance(step)
		tt.Frame()
	}
	tt.FingerUp(x, y+dy)
	tt.Frame()
}

// On a phone, a picture in the viewer is taken away by a flick down (or up): it
// follows the finger and, past a third of the screen (the throw counted), closes
// the viewer; a short slow drag springs it back and leaves it open.
func TestPhoneFlickToCloseViewer(t *testing.T) {
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	fast, slow := 16*time.Millisecond, 120*time.Millisecond
	for _, tc := range []struct {
		name  string
		dy    float32
		step  time.Duration
		close bool
	}{
		{"down far", 330, fast, true},
		{"up far", -330, fast, true},
		{"down short flick", 60, fast, true},
		{"down short slow", 60, slow, false},
	} {
		a := testApp()
		a.pushed = true
		a.viewerItems = []kit.MediaSpec{{ID: "pic", Kind: kit.MediaImage, W: 400, H: 300, Label: "a picture"}}
		a.viewerAt, a.viewer = 0, true
		tt := ui.NewTester(a.View, 400, 800)
		settle(tt)
		if !a.viewer {
			t.Fatalf("%s: the viewer is not open", tc.name)
		}
		dragY(tt, 200, 400, tc.dy, tc.step)
		if tc.name == "down far" {
			snap(t, "viewer-flick", tt)
		}
		settle(tt)
		settle(tt)
		if a.viewer == tc.close {
			t.Errorf("%s: viewer open %v, want %v", tc.name, a.viewer, !tc.close)
		}
	}
}

// The arrow beside a message being pulled, before and at the distance that
// replies (a snapshot with SNAP_DIR).
func TestPhoneSwipeCueRenders(t *testing.T) {
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	for _, tc := range []struct {
		name, text string
		dx         float32
	}{{"in-part", "hey! are we still on for", 34}, {"in-armed", "hey! are we still on for", 80}, {"out-armed", "yes!! 10:00 works", -80}} {
		a := testApp()
		a.open("!dm-alex")
		a.pushed = true
		tt := ui.NewTester(a.View, 400, 800)
		settle(tt)
		var r ui.Rect
		for _, s := range tt.Texts() {
			if len(s) >= len(tc.text) && s[:len(tc.text)] == tc.text {
				r, _ = tt.Find(s)
				break
			}
		}
		x, y := r.X+r.W/2, r.Y+r.H/2
		tt.FingerDown(x, y)
		tt.Advance(40 * time.Millisecond)
		tt.Frame()
		for i := 1; i <= 10; i++ {
			tt.FingerMove(x+tc.dx*float32(i)/10, y)
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
		}
		snap(t, "swipe-"+tc.name, tt)
	}
}

// On a phone a message pulled sideways far enough replies, as iMessage's and
// WhatsApp's do: an incoming message (on the left) to the right, one of the
// user's own (on the right) to the left. Not far enough, or the wrong way,
// does nothing.
func TestPhoneSwipeToReply(t *testing.T) {
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	for _, tc := range []struct {
		name, text string
		dx         float32
		want       bool
	}{
		{"incoming, right, far", "hey! are we still on for", 110, true},
		{"incoming, right, short", "hey! are we still on for", 24, false},
		{"incoming, left", "hey! are we still on for", -110, false},
		{"own, left, far", "yes!! 10:00 works", -110, true},
		{"own, right", "yes!! 10:00 works", 110, false},
	} {
		a := testApp()
		a.open("!dm-alex")
		a.pushed = true
		tt := ui.NewTester(a.View, 400, 800)
		settle(tt)
		var r ui.Rect
		found := false
		for _, s := range tt.Texts() {
			if len(s) >= len(tc.text) && s[:len(tc.text)] == tc.text {
				r, found = tt.Find(s)
				break
			}
		}
		if !found {
			t.Fatalf("%s: no message %q (texts: %v)", tc.name, tc.text, tt.Texts())
		}
		swipe(tt, r.X+r.W/2, r.Y+r.H/2, tc.dx)
		settle(tt)
		_, replying := a.replyTo["!dm-alex"]
		if replying != tc.want {
			t.Errorf("%s: replying %v, want %v", tc.name, replying, tc.want)
		}
	}
}

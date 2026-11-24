package kit

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// trace runs an animation of a value from `from` to `to` in a Tester, in 16 ms
// frames, and returns the value shown after each, and whether an animation
// frame was still being asked for (the animation still going on).
func trace(t *testing.T, a ui.Animation, from, to float32, frames int) (vals []float32, alive []bool) {
	t.Helper()
	target, last := from, from
	tt := ui.NewTester(func(c *ui.Context) {
		last = ui.Box(c.Key("b")).Size(10, 10).AnimateTo("v", target, a)
	}, 100, 100)
	tt.Frame()
	target = to
	for i := 0; i < frames; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		vals = append(vals, last)
		alive = append(alive, tt.FramePending())
	}
	return vals, alive
}

// settledAt is the first frame after which the animation asks for no more
// frames: while it does, the router builds both its pages every frame.
func settledAt(alive []bool) int {
	for i := len(alive) - 1; i >= 0; i-- {
		if alive[i] {
			return i + 1
		}
	}
	return 0
}

// A page push is over soon after it looks over: its spring may not go on for
// seconds past its end. A bounce past the end is clamped away by the router
// (the page cannot go further than gone), so it shows nothing and only keeps
// both pages being built, frame after frame; the spring therefore has none.
func TestPushSpringEndsWhenItLooksOver(t *testing.T) {
	for _, from := range []float32{0, 0.4, 0.6} {
		v, alive := trace(t, PushSpring, from, 1, 90)
		peak := float32(0)
		for _, x := range v {
			peak = max(peak, x)
		}
		end := settledAt(alive)
		t.Logf("from %.1f: peak %.4f, asks for frames until frame %d (%d ms)", from, peak, end, end*16)
		if peak > 1.001 {
			t.Errorf("from %.1f: goes past its end to %v (the router clamps it: it only costs frames)", from, peak)
		}
		// 90% of the way by 0.25 s, and no frames asked for after 0.7 s.
		if v[15] < from+0.9*(1-from) {
			t.Errorf("from %.1f: only %.3f of the way at 0.25 s", from, (v[15]-from)/(1-from))
		}
		if end*16 > 700 {
			t.Errorf("from %.1f: still animating at %d ms", from, end*16)
		}
		// Never turns back, never speeds up after it begins to slow.
		for i := 1; i < len(v); i++ {
			if v[i] < v[i-1]-1e-4 {
				t.Errorf("from %.1f: turns back at frame %d", from, i)
				break
			}
		}
		_ = math.Abs
	}
}

// A page pop is a fixed-length curve: exactly done in its time, with no tail.
func TestPopCurveIsDoneInItsTime(t *testing.T) {
	for _, from := range []float32{0.3, 0.6, 0.9} {
		v, alive := trace(t, PopCurve, from, 1, 40)
		end := settledAt(alive)
		t.Logf("from %.1f: asks for frames until frame %d (%d ms)", from, end, end*16)
		if end*16 > 360 {
			t.Errorf("from %.1f: still animating at %d ms", from, end*16)
		}
		if v[len(v)-1] != 1 {
			t.Errorf("from %.1f: ends at %v", from, v[len(v)-1])
		}
		for i := 1; i < len(v); i++ {
			if v[i] < v[i-1]-1e-6 || v[i] > 1+1e-6 {
				t.Errorf("from %.1f: not monotonic into 1 at frame %d: %v then %v", from, i, v[i-1], v[i])
				break
			}
		}
	}
}

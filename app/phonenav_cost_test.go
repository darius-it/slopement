package app

import (
	"os"
	"sort"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// frameCosts builds n frames, each after a 16 ms clock step, and returns how
// long each took to build and lay out, in ms.
func frameCosts(tt *ui.Tester, n int) []float64 {
	var out []float64
	for i := 0; i < n; i++ {
		tt.Advance(16 * time.Millisecond)
		t0 := time.Now()
		tt.Frame()
		out = append(out, float64(time.Since(t0).Microseconds())/1000)
	}
	return out
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

// A diagnostic (SLOPEMENT_COST=1): what a frame of a push or a pop costs next to
// a frame of a room at rest. The router builds both pages in every frame of a
// slide, so a slide frame costs about the room plus the lists: on a phone, a
// few times slower than here, a slide frame over 16 ms drops frames.
func TestPhoneSlideFrameCost(t *testing.T) {
	if os.Getenv("SLOPEMENT_COST") == "" {
		t.Skip("a diagnostic: set SLOPEMENT_COST=1")
	}
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	rest := frameCosts(tt, 20)
	a.pushed = false
	pop := frameCosts(tt, 16)
	lists := frameCosts(tt, 20)
	a.open("!dm-alex")
	push := frameCosts(tt, 16)
	t.Logf("room at rest:  median %.2f ms (max %.2f)", median(rest), maxOf(rest))
	t.Logf("pop (both pages): median %.2f ms (max %.2f)", median(pop), maxOf(pop))
	t.Logf("lists at rest: median %.2f ms (max %.2f)", median(lists), maxOf(lists))
	t.Logf("push (both pages): median %.2f ms (max %.2f)", median(push), maxOf(push))
	t.Logf("pop frames: %.1f", pop)
}

// BenchmarkPhoneSlide pushes and pops a room again and again, for a profile of
// what a slide frame spends its time on:
//
//	go test -tags goolm ./app -run XXX -bench PhoneSlide -cpuprofile <scratch>/cpu.prof
func BenchmarkPhoneSlide(b *testing.B) {
	defer kit.SetPhoneOS(kit.SetPhoneOS(true))
	a := testApp()
	a.open("!dm-alex")
	a.pushed = true
	tt := ui.NewTester(a.View, 400, 800)
	settle(tt)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.pushed = false
		for j := 0; j < 20; j++ {
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
		}
		a.open("!dm-alex")
		for j := 0; j < 20; j++ {
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
		}
	}
}

func maxOf(v []float64) float64 {
	m := 0.0
	for _, x := range v {
		m = max(m, x)
	}
	return m
}

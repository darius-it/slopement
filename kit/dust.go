package kit

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Spoilers dissolve when revealed: a small burst of dust where the veil
// was, drifting up and out and fading, while the content fades in. Subtle
// (tiny dots, ~0.7 s) but a little fun, as Telegram's spoilers are. Reduce
// Motion keeps the fade and drops the dust.

// dustDuration is how long the dust takes to settle.
const dustDuration = 700 * time.Millisecond

// dustBurst is one reveal's dust: where it starts, in the drawing element's
// coordinates, and over how wide an area.
type dustBurst struct {
	x, y   float32 // the center, local to the element
	w, h   float32 // the area the dust starts in
	n      int     // how many grains
	seed   uint32
	grains ui.Color
}

// rnd is a small deterministic generator: the same burst draws the same
// grains every frame.
type rnd uint32

func (r *rnd) next() float32 {
	*r ^= *r << 13
	*r ^= *r >> 17
	*r ^= *r << 5
	return float32(*r%10000) / 10000
}

// drawDust paints a burst at progress t (0 to 1) in r, the element's box
// in window DIPs.
func drawDust(pt *ui.Painter, r ui.Rect, b dustBurst, t float32) {
	if t >= 1 {
		return
	}
	g := rnd(b.seed | 1)
	ease := 1 - float32(math.Pow(float64(1-t), 3)) // ease-out: fast, then drifting
	fade := float32(math.Pow(float64(1-t), 1.6))
	for i := 0; i < b.n; i++ {
		sx := (g.next() - 0.5) * b.w
		sy := (g.next() - 0.5) * b.h
		ang := g.next() * 2 * math.Pi
		dist := 8 + g.next()*26
		size := 1.2 + g.next()*1.8
		delay := g.next() * 0.25 // grains leave one after another
		lt := min(max((ease-delay)/(1-delay), 0), 1)
		x := r.X + b.x + sx + float32(math.Cos(float64(ang)))*dist*lt
		y := r.Y + b.y + sy + float32(math.Sin(float64(ang)))*dist*lt - 14*lt // a drift up
		a := fade * (0.55 + 0.45*g.next())
		if a <= 0.01 {
			continue
		}
		pt.Fill(ui.Rect{X: x - size/2, Y: y - size/2, W: size, H: size}, b.grains.Alpha(a), size/2)
	}
}

// dustState is the bursts drawing over an element, by key.
type dustState map[string]dustBurst

// reveals' bursts, by the element they draw over: kept while they play.
var dusts = map[string]dustState{}

// startDust starts a burst over the element of id, starting progress at 0.
func startDust(c *ui.Context, e ui.Element, id, key string, b dustBurst) {
	if c.Preferences().ReduceMotion {
		return
	}
	if dusts[id] == nil {
		dusts[id] = dustState{}
	}
	b.seed = uint32(time.Now().UnixNano())
	dusts[id][key] = b
	e.Jump("dust:"+key, 0)
}

// playDust draws the bursts of id over e, advancing them, and forgets
// those done. Call it on the element after building its children.
func playDust(e ui.Element, id string) {
	bs := dusts[id]
	if len(bs) == 0 {
		return
	}
	prog := map[string]float32{}
	for key := range bs {
		t := e.AnimateTo("dust:"+key, 1, ui.Animation{Duration: dustDuration, Ease: ui.Linear, NonMotion: false})
		if t >= 1 {
			delete(bs, key)
			continue
		}
		prog[key] = t
	}
	if len(bs) == 0 {
		delete(dusts, id)
		return
	}
	snapshot := make([]struct {
		b dustBurst
		t float32
	}, 0, len(bs))
	for key, b := range bs {
		snapshot = append(snapshot, struct {
			b dustBurst
			t float32
		}{b, prog[key]})
	}
	e.DrawOver(func(pt *ui.Painter, r ui.Rect) {
		for _, s := range snapshot {
			drawDust(pt, r, s.b, s.t)
		}
	})
}

package app

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// avatarSource gives every person a profile picture and serves it as a
// small solid PNG, or fails.
type avatarSource struct {
	model.Source
	calls int
	fail  bool
	sides []int
}

func (s *avatarSource) Person(id string) (model.Person, bool) {
	p, ok := s.Source.Person(id)
	p.Avatar = "mxc://example.org/" + id
	return p, ok
}

func (s *avatarSource) LoadMedia(m model.Media, done func([]byte, error)) {
	s.calls++
	s.sides = append(s.sides, m.Square)
	if s.fail {
		done(nil, errors.New("no such media"))
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 160, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	done(buf.Bytes(), nil)
}

// A profile picture is fetched once, as a thumbnail, and then shows; a
// failed one is not asked for again every frame.
func TestAvatarLoadsOnceAndNeverStorms(t *testing.T) {
	a := testApp()
	src := &avatarSource{Source: a.src}
	a.src = src
	a.runUI = func(f func()) { f() }

	p, _ := a.src.Person("@someone:example.org")
	if a.avatar(p, 36).Image != nil && src.calls == 0 {
		t.Fatal("a picture before anything was fetched")
	}
	for i := 0; i < 5; i++ {
		if a.avatar(p, 36).Image == nil {
			t.Fatal("the picture did not arrive")
		}
	}
	if src.calls != 1 || src.sides[0] != avatarSide {
		t.Fatalf("fetches = %d, sides = %v: want one thumbnail request of %d", src.calls, src.sides, avatarSide)
	}

	bad := &avatarSource{Source: testApp().src, fail: true}
	a2 := testApp()
	a2.src = bad
	a2.runUI = func(f func()) { f() }
	q, _ := a2.src.Person("@other:example.org")
	for i := 0; i < 10; i++ {
		if a2.avatar(q, 36).Image != nil {
			t.Fatal("a picture from a failed fetch")
		}
	}
	if bad.calls != 1 {
		t.Fatalf("a failed picture was asked for %d times", bad.calls)
	}
}

// The reply quote shows who is quoted with their picture beside the name
// (a snapshot with SNAP_DIR).
func TestReplyQuoteShowsPicture(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 8)
	src := &avatarSource{Source: timeline{a.src, tl}}
	a.src = src
	a.runUI = func(f func()) { f() }
	a.open(room.ID)
	for i := 0; i < 3; i++ { // the first frame asks, the next show
		tt := ui.NewTester(a.View, 1000, 600)
		settle(tt)
		if i == 2 {
			snap(t, "reply-avatar", tt)
		}
	}
	if q := a.quote(room.ID, "$2"); q == nil || q.Avatar.Image == nil {
		t.Fatal("the quote carries no picture")
	}
}

package app

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strconv"
	"testing"
	"time"

	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

func gifBytes() []byte {
	pal := color.Palette{color.RGBA{200, 30, 30, 255}, color.Black}
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 8, 8), pal))
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	_ = gif.EncodeAll(&b, g)
	return b.Bytes()
}

// originalsSource serves the originals of profile pictures: a GIF, a PNG, a
// failure, and counts what was asked.
type originalsSource struct {
	model.Source
	asked map[string]int
	files map[string][]byte
	fail  map[string]bool
}

func (s *originalsSource) Person(id string) (model.Person, bool) {
	p, ok := s.Source.Person(id)
	p.Avatar = "mxc://example.org/" + id
	return p, ok
}

func (s *originalsSource) LoadMedia(m model.Media, done func([]byte, error)) {
	s.asked[m.URL+"#"+strconv.Itoa(m.Square)]++
	if s.fail[m.URL] {
		done(nil, errors.New("offline"))
		return
	}
	done(s.files[m.URL], nil)
}

// A GIF profile picture's original is fetched the first time it would play,
// not before; the animation is kept; a picture that is no GIF is not asked for
// again; a failure may be asked again.
func TestAvatarAnimationLoadsLazily(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var pngB bytes.Buffer
	_ = png.Encode(&pngB, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	m := mock.New(time.Now())
	m.Delay = 0
	src := &originalsSource{Source: m, asked: map[string]int{},
		files: map[string][]byte{"mxc://example.org/@gif": gifBytes(), "mxc://example.org/@png": pngB.Bytes()},
		fail:  map[string]bool{"mxc://example.org/@bad": true}}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }

	gifURL, pngURL, badURL := "mxc://example.org/@gif", "mxc://example.org/@png", "mxc://example.org/@bad"
	original := func(u string) int { return src.asked[u+"#0"] }

	// Drawing avatars asks for no originals.
	for _, id := range []string{"@gif", "@png", "@bad"} {
		p, _ := src.Person(id)
		spec := a.avatar(p, 36)
		if spec.WantAnim == nil || spec.Anim != nil || spec.Animate {
			t.Fatalf("%s: spec = %+v", id, spec)
		}
	}
	if original(gifURL)+original(pngURL)+original(badURL) != 0 {
		t.Fatal("an original was fetched before anything would play")
	}

	// Asked to play: the GIF is fetched once and kept as a player.
	a.wantAvatarAnim(gifURL)
	a.wantAvatarAnim(gifURL)
	if original(gifURL) != 1 || a.avatarAnims[gifURL] == nil {
		t.Fatalf("gif: fetched %d times, player %v", original(gifURL), a.avatarAnims[gifURL])
	}
	p, _ := src.Person("@gif")
	if a.avatar(p, 36).Anim == nil {
		t.Fatal("the picture's spec does not carry its animation")
	}

	// A PNG is noted as no GIF and never asked for again.
	a.wantAvatarAnim(pngURL)
	a.wantAvatarAnim(pngURL)
	if original(pngURL) != 1 || a.avatarAnims[pngURL] != nil {
		t.Fatalf("png: fetched %d times", original(pngURL))
	}
	if _, known := a.avatarAnims[pngURL]; !known {
		t.Fatal("a still is not remembered as one")
	}

	// A failure is not remembered: it is asked for again.
	a.wantAvatarAnim(badURL)
	a.wantAvatarAnim(badURL)
	if original(badURL) != 2 {
		t.Fatalf("failure: fetched %d times, want it asked again", original(badURL))
	}

	// The setting and the profile card.
	a.prefs.animatedAvatars = true
	if !a.avatar(p, 36).Animate {
		t.Fatal("the setting does not make avatars animate")
	}
	a.prefs.animatedAvatars = false
	if a.avatar(p, 36).Animate || !a.profile("@gif").Avatar.Animate {
		t.Fatal("off: avatars animate; or the profile card's does not")
	}
}

// At most avatarAnimMax players are kept; the oldest is closed.
func TestAvatarAnimationsAreBounded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	files := map[string][]byte{}
	for i := 0; i < avatarAnimMax+5; i++ {
		files["mxc://example.org/"+strconv.Itoa(i)] = gifBytes()
	}
	src := &originalsSource{Source: m, asked: map[string]int{}, files: files}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }
	for i := 0; i < avatarAnimMax+5; i++ {
		a.wantAvatarAnim("mxc://example.org/" + strconv.Itoa(i))
	}
	if len(a.avatarAnims) != avatarAnimMax || len(a.avatarAnimOrder) != avatarAnimMax {
		t.Fatalf("%d players kept", len(a.avatarAnims))
	}
	if _, ok := a.avatarAnims["mxc://example.org/0"]; ok {
		t.Fatal("the oldest is kept")
	}
	if _, ok := a.avatarAnims["mxc://example.org/"+strconv.Itoa(avatarAnimMax+4)]; !ok {
		t.Fatal("the newest is gone")
	}
}

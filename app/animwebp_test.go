package app

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// A one-frame animated WebP (the sample Modernizr tests WebP support with).
const animatedWebP = "UklGRlIAAABXRUJQVlA4WAoAAAASAAAAAAAAAAAAQU5JTQYAAAD/////AABBTk1GJgAAAAAAAAAAAAAAAAAAAGQAAABWUDhMDQAAAC8AAAAQBxAREYiI/gcA"
const stillWebP = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"

type webpSource struct {
	model.Source
	files map[string][]byte
}

func (s *webpSource) Timeline(room string) []model.Event {
	if room != "!crit" {
		return s.Source.Timeline(room)
	}
	at := time.Now().Add(-time.Minute)
	var evs []model.Event
	for i, u := range []string{"mxc://x/anim", "mxc://x/still"} {
		evs = append(evs, model.Event{ID: "$w" + string(rune('a'+i)), Room: room, Time: at, Kind: model.EventMedia,
			Sender: "@x:example.org", Body: "sticker", Media: []model.Media{{Kind: model.MediaImage, URL: u,
				Mime: "image/webp", W: 1, H: 1, Name: "sticker", Sticker: true}}})
	}
	return evs
}

func (s *webpSource) LoadMedia(m model.Media, done func([]byte, error)) { done(s.files[m.URL], nil) }

// An animated WebP plays like a GIF; a still one stays a picture.
func TestAnimatedWebPPlays(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	anim, _ := base64.StdEncoding.DecodeString(animatedWebP)
	still, _ := base64.StdEncoding.DecodeString(stillWebP)
	m := mock.New(time.Now())
	m.Delay = 0
	src := &webpSource{Source: m, files: map[string][]byte{"mxc://x/anim": anim, "mxc://x/still": still}}
	a := New(src, picker.New(nil))
	a.runUI = func(f func()) { f() }
	a.nav = kit.NavRail
	a.open("!crit")
	tt := ui.NewTesterWith(a.View, 1100, 800, ui.TesterOptions{Dark: true})
	settle(tt)
	settle(tt)

	evs := src.Timeline("!crit")
	_, animMedia, _ := a.content(evs[0])
	_, stillMedia, _ := a.content(evs[1])
	if len(animMedia) != 1 || animMedia[0].Kind != kit.MediaGIF || animMedia[0].Player == nil {
		t.Fatalf("an animated WebP does not play: %+v", animMedia)
	}
	if animMedia[0].Still == nil {
		t.Fatal("an animated WebP has no first frame for a still")
	}
	if len(stillMedia) != 1 || stillMedia[0].Kind == kit.MediaGIF || stillMedia[0].Player != nil || stillMedia[0].Still == nil {
		t.Fatalf("a still WebP was made to play, or lost its picture: %+v", stillMedia)
	}
}

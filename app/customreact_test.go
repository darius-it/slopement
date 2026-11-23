package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// reactSource records custom emoji reactions.
type reactSource struct {
	packSource
	got []string
}

func (r *reactSource) ReactCustom(room, event, url, shortcode string) {
	r.got = append(r.got, room+"|"+event+"|"+url+"|"+shortcode)
}

// A custom emoji from a pack reacts with its mxc:// URI and shortcode; an
// account that cannot says so.
func TestReactWithPackEmoji(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	src := &reactSource{packSource: packSource{Source: m}}
	a := New(src, picker.New(nil))
	e, ok := a.findPackEmoji("!r", &kit.PackPick{Pack: "!r|cats", Tile: "wave"})
	if !ok {
		t.Fatal("no such emoji in the packs")
	}
	a.reactCustom("!r", "$ev", e)
	if len(src.got) != 1 || src.got[0] != "!r|$ev|mxc://x/wave|wave" {
		t.Fatalf("reactions = %v", src.got)
	}

	plain := New(mock.New(time.Now()), picker.New(nil))
	plain.src = &struct{ model.Source }{Source: mock.New(time.Now())}
	plain.reactCustom("!r", "$ev", e)
	if plain.toast == "" {
		t.Fatal("an account that cannot react with custom emoji did not say so")
	}
}

// A reaction whose key is an mxc:// URI shows as a picture, not as its address,
// and is named for a screen reader.
func TestCustomEmojiReactionShowsAPicture(t *testing.T) {
	kit.EmojiImage = func(mxc string) ui.ImageSource { return nil } // not loaded: an empty place
	defer func() { kit.EmojiImage = nil }()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	m.ReactCustom("!crit", m.Timeline("!crit")[len(m.Timeline("!crit"))-1].ID, "mxc://x/wave", "wave")
	a := New(m, picker.New(nil))
	a.open("!crit")
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	if tt.HasText("mxc://x/wave") {
		t.Fatal("the address of the emoji is shown as text")
	}
	found := false
	for _, acc := range tt.AccessAll() {
		if acc.Name == "Custom emoji 1" {
			found = true
		}
	}
	if !found {
		t.Fatal("no reaction named \"Custom emoji 1\"")
	}
}

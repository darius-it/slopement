package app

import (
	"testing"
	"time"

	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// packSource offers a user pack and a room pack, and records what is done
// with them.
type packSource struct {
	model.Source
	stickers []model.Sticker
	added    []model.PackEmoji
}

func (p *packSource) EmojiPacks(room string) []model.EmojiPack {
	return []model.EmojiPack{
		{ID: "user", Name: "My emoji", Mine: true, Emoji: []model.PackEmoji{{Shortcode: "mine", URL: "mxc://x/mine", Emoticon: true}}},
		{ID: "!r|cats", Name: "Cats", Room: room, Emoji: []model.PackEmoji{
			{Shortcode: "wave", Body: "waving", URL: "mxc://x/wave", Mime: "image/png", W: 64, H: 64, Emoticon: true},
			{Shortcode: "big", URL: "mxc://x/big", Mime: "image/webp", Sticker: true},
		}},
	}
}
func (p *packSource) AddToPack(e model.PackEmoji, done func(string)) {
	p.added = append(p.added, e)
	done("")
}
func (p *packSource) SendSticker(room string, s model.Sticker, replyTo string) {
	p.stickers = append(p.stickers, s)
}

func packApp(t *testing.T) (*App, *packSource) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	src := &packSource{Source: m}
	return New(src, picker.New(nil)), src
}

// The packs on offer become sections of the emoji tab (emoticons) and the
// sticker tab (stickers), each image in the one it may be used as.
func TestPacksFillThePicker(t *testing.T) {
	a, _ := packApp(t)
	m := &kit.PickerModel{}
	a.fillPacks("!r", m)
	if len(m.EmojiPacks) != 2 || m.EmojiPacks[1].Name != "Cats" || len(m.EmojiPacks[1].Tiles) != 1 || m.EmojiPacks[1].Tiles[0].ID != "wave" {
		t.Fatalf("emoji packs = %+v", m.EmojiPacks)
	}
	if len(m.StickerPacks) != 1 || m.StickerPacks[0].Tiles[0].ID != "big" || m.StickerPacks[0].Mine {
		t.Fatalf("sticker packs = %+v", m.StickerPacks)
	}
}

// ":wa" suggests the packs' custom emoji before the usual ones, and taking
// one writes its ":shortcode:".
func TestCustomEmojiSuggestions(t *testing.T) {
	a, _ := packApp(t)
	st := &kit.ComposerState{Text: "hello :wa"}
	got := a.suggestions("!r", st)
	if len(got) == 0 || got[0].Insert != ":wave:" || got[0].Image == nil || got[0].Sub != "Cats" {
		t.Fatalf("suggestions = %+v", got)
	}
	if len(got) > suggestMax {
		t.Fatalf("%d suggestions", len(got))
	}
	a.applySuggestion("!r", st, got[0])
	if st.Text != "hello :wave: " { // a space after it, to go on typing
		t.Fatalf("text = %q", st.Text)
	}
	// A sticker-only image is not an emoji.
	if got := a.suggestions("!r", &kit.ComposerState{Text: ":big"}); len(got) > 0 && got[0].Insert == ":big:" {
		t.Fatal("suggested a sticker-only image as an emoji")
	}
}

// A sticker anyone sent can be added to the user's own pack from its menu.
func TestAddStickerFromMessageMenu(t *testing.T) {
	a, src := packApp(t)
	ev := model.Event{ID: "$s", Room: "!r", Sender: "@ada:example.org", Kind: model.EventMedia, Body: "Dancing Cat",
		Media: []model.Media{{Sticker: true, URL: "mxc://x/dance", Name: "Dancing Cat", Mime: "image/gif", W: 90, H: 90}}}
	if !a.msgMenu(ev).Sticker {
		t.Fatal("the menu does not offer it")
	}
	if plain := a.msgMenu(model.Event{ID: "$p", Room: "!r", Kind: model.EventMedia, Media: []model.Media{{URL: "mxc://x/pic"}}}); plain.Sticker {
		t.Fatal("a picture is offered as a sticker")
	}
	a.messageChoice("!r", ev, kit.MsgAddToPack)
	if len(src.added) != 1 || src.added[0].URL != "mxc://x/dance" || src.added[0].Shortcode != "dancing_cat" || a.toast != "Added to your emoji" {
		t.Fatalf("added = %+v, toast %q", src.added, a.toast)
	}
}

// Choosing from a pack: an emoji is typed in as its shortcode, a sticker is
// sent by its mxc url (no upload), and "add" puts it in the user's pack.
func TestPackChoices(t *testing.T) {
	a, src := packApp(t)
	st := &kit.ComposerState{Text: "hi "}
	a.packAct("!r", st, kit.PickerResult{CustomEmoji: &kit.PackPick{Pack: "!r|cats", Tile: "wave"}})
	if st.Text != "hi :wave:" {
		t.Fatalf("text = %q", st.Text)
	}
	a.picker.Open = true
	a.packAct("!r", st, kit.PickerResult{PackSticker: &kit.PackPick{Pack: "!r|cats", Tile: "big"}})
	if len(src.stickers) != 1 || src.stickers[0].URL != "mxc://x/big" || len(src.stickers[0].Data) != 0 || a.picker.Open {
		t.Fatalf("stickers = %+v, open %v", src.stickers, a.picker.Open)
	}
	a.packAct("!r", st, kit.PickerResult{AddToPack: &kit.PackPick{Pack: "!r|cats", Tile: "wave"}})
	if len(src.added) != 1 || src.added[0].URL != "mxc://x/wave" || a.toast != "Added to your emoji" {
		t.Fatalf("added = %+v, toast %q", src.added, a.toast)
	}
	a.packAct("!r", st, kit.PickerResult{CustomEmoji: &kit.PackPick{Pack: "nope", Tile: "x"}})
	if st.Text != "hi :wave:" {
		t.Fatal("an unknown pick changed the text")
	}
}

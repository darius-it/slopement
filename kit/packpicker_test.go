package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func packModel() *PickerModel {
	return &PickerModel{
		EmojiPacks: []PickerPack{
			{ID: "user", Name: "My emoji", Mine: true, Tiles: []PackTile{{ID: "mine", Label: "mine"}}},
			{ID: "cats", Name: "Cats", Tiles: []PackTile{{ID: "wave", Label: "waving"}, {ID: "purr", Label: "purring"}}},
		},
		StickerPacks: []PickerPack{{ID: "cats", Name: "Cat stickers", Tiles: []PackTile{{ID: "big", Label: "big cat"}}}},
	}
}

// The emoji tab shows each pack under its name, a search narrows the packs'
// images by name, and choosing one says which.
func TestPickerShowsPacks(t *testing.T) {
	m := packModel()
	st := &PickerState{Tab: TabEmoji}
	st.Open = true
	var res PickerResult
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c)
		if r := Picker(c, ui.Button(c.Key("anchor"), "e"), st, m); r.CustomEmoji != nil || r.AddToPack != nil || r.PackSticker != nil {
			res = r
		}
	}, 900, 700)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if !tt.HasText("My emoji") || !tt.HasText("Cats") {
		t.Fatal("the packs' names are not shown")
	}
	tt.Click("waving")
	tt.Frame()
	if res.CustomEmoji == nil || res.CustomEmoji.Pack != "cats" || res.CustomEmoji.Tile != "wave" {
		t.Fatalf("choice = %+v", res.CustomEmoji)
	}
	if st.Open {
		t.Fatal("the picker stays open after a choice")
	}
}

func TestPickerPackSearchAndStickers(t *testing.T) {
	m := packModel()
	st := &PickerState{Tab: TabEmoji, Query: "pur"}
	st.Open = true
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c)
		Picker(c, ui.Button(c.Key("anchor"), "e"), st, m)
	}, 900, 700)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	buttons := map[string]bool{}
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton {
			buttons[a.Name] = true
		}
	}
	if !buttons["purring"] || buttons["waving"] || buttons["mine"] {
		t.Fatalf("search \"pur\" shows %v", buttons)
	}
	st.Tab, st.Query = TabStickers, ""
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if !tt.HasText("Cat stickers") {
		t.Fatal("the sticker pack is not shown on the sticker tab")
	}
}

package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Custom emoji and sticker packs in the picker (MSC2545): sections of
// pictures, each named by the pack. The picker draws them under the recently
// used emoji, and before trending stickers.

// PackTile is one image of a pack.
type PackTile struct {
	ID    string // the shortcode
	Label string // what it is called, for screen readers and the footer
	// Image gives the picture once it has arrived (nil meanwhile, a quiet
	// placeholder). It is only asked for while the tile is near the view,
	// so a big pack doesn't fetch everything at once.
	Image func() ui.ImageSource
}

// PickerPack is a pack as a section.
type PickerPack struct {
	ID    string
	Name  string
	Mine  bool // the user's own: no "add to my emoji" on its tiles
	Tiles []PackTile
}

// PackPick says which tile of which pack the user chose.
type PackPick struct {
	Pack, Tile string
}

// packSections draws the packs' tiles: all of them, or the ones whose names
// contain query. emoji says they are small, in rows of nine; stickers are
// bigger, four across.
func packSections(c *ui.Context, st *PickerState, packs []PickerPack, query string, sticker bool, r *PickerResult) {
	cols, size := 9, float32(40)
	if sticker {
		cols, size = 4, 88
	}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, pk := range packs {
		tiles := pk.Tiles
		if query != "" {
			tiles = nil
			for _, t := range pk.Tiles {
				if strings.Contains(strings.ToLower(t.ID), query) || strings.Contains(strings.ToLower(t.Label), query) {
					tiles = append(tiles, t)
				}
			}
		}
		if len(tiles) == 0 {
			continue
		}
		sectionLabel(c, pk.Name)
		ui.Grid(c.Key("pack-" + pk.ID)).Columns(cols).Gap(2).Children(func() {
			for _, t := range tiles {
				packCell(c, st, pk, t, size, r)
			}
		})
	}
}

func packCell(c *ui.Context, st *PickerState, pk PickerPack, t PackTile, size float32, r *PickerResult) {
	p := Of(c)
	b := ui.ButtonBase(c.Key("pk:" + pk.ID + ":" + t.ID)).Height(size).Radius(RadiusSm).Padding(S05).Label(t.Label)
	hov := b.Hovered() || b.FocusVisible()
	if hov {
		b.Background(p.Hover)
		st.hoverName = t.ID
	}
	b.Children(func() {
		var src ui.ImageSource
		if t.Image != nil && b.InViewMargin(200) {
			src = t.Image()
		}
		if src != nil {
			ui.Image(c, src).Fill().Fit(ui.Contain)
		} else {
			ui.Box(c).Fill().Radius(RadiusSm).Background(p.BgInset)
		}
		if hov && !pk.Mine {
			// Anything seen can be added to the user's own pack.
			add := ui.ButtonBase(c.Key("add")).Size(20, 20).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorTopRight, ui.AnchorTopRight).Background(ui.RGBA(10, 8, 20, 0.55)).Label("Add to my emoji")
			add.Children(func() { ui.Icon(c, IconPlus).Size(12, 12).TextColor(ui.Hex("#FFFFFF")) })
			Pressable(add)
			if add.Clicked() {
				r.AddToPack = &PackPick{pk.ID, t.ID}
			}
		}
	})
	Pressable(b)
	if b.Clicked() && r.AddToPack == nil {
		if size > 40 {
			r.PackSticker = &PackPick{pk.ID, t.ID}
		} else {
			r.CustomEmoji = &PackPick{pk.ID, t.ID}
		}
	}
}

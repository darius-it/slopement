package kit

import (
	"github.com/egoist/mygo/ui"
)

// GIFTile is a GIF result or a category of the picker.
type GIFTile struct {
	ID    string
	Label string // a category's name, or the GIF's title for screen readers
	// Preview is the small still or animated rendition; nil while loading,
	// which shows a quiet placeholder.
	Preview *ui.Bitmap
	W, H    float32
	// Category tiles show their Label over the picture and search for it.
	Category bool
}

// PickerTabs are the picker's kinds of content: emoji are local, the rest
// come from Klipy (GIFs, stickers, clips, memes).
var PickerTabs = []string{"Emoji", "GIFs", "Stickers", "Clips", "Memes"}

var pickerPlaceholders = []string{"Search emoji", "Search GIFs", "Search stickers", "Search clips", "Search memes"}

// PickerState is the emoji / GIF / sticker / clip / meme picker's state.
// Tab indexes PickerTabs.
type PickerState struct {
	SheetState
	Tab   int
	Query string
}

// PickerResult is what the picker chose.
type PickerResult struct {
	Emoji    string
	GIF      *GIFTile
	Category string // a category tapped: search for it
}

// CommonEmoji is a first page of emoji, until the full set is wired up.
var CommonEmoji = []string{"😀", "😂", "🥲", "😊", "😍", "🤔", "😴", "😮", "😢", "😡", "👍", "👎", "👏", "🙏", "💪", "🎉", "❤️", "🔥", "✨", "👀", "✅", "❌", "💯", "🤝"}

// Picker is the composer's picker: tabs for emoji, GIFs and stickers, a
// search field, and a grid. It opens as a popover from its button on wide
// windows and as a sheet on phones (Discord's and Telegram's layout).
func Picker(c *ui.Context, anchor ui.Element, st *PickerState, gifs []GIFTile) (r PickerResult) {
	Adaptive(c, anchor, &st.SheetState, "", 400, func() {
		p := Of(c)
		Tabs(c, &st.Tab, PickerTabs...)
		placeholder := pickerPlaceholders[st.Tab]
		SearchField(c.Key("picker-search"), &st.Query, placeholder, "").AutoFocus()
		ui.Scroll(c).Height(320).Children(func() {
			switch st.Tab {
			case 0:
				ui.Grid(c).Columns(8).Gap(2).Children(func() {
					for _, e := range CommonEmoji {
						b := ui.ButtonBase(c.Key(e)).Height(40).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label(e)
						if b.Hovered() {
							b.Background(p.Hover)
						}
						b.Children(func() { ui.Text(c, e).FontSize(22) })
						if b.Clicked() {
							r.Emoji = e
						}
					}
				})
			default:
				ui.Grid(c).Columns(2).Gap(S1 + 2).Children(func() {
					for i := range gifs {
						g := &gifs[i]
						if GIFCell(c, g).Clicked() {
							if g.Category {
								r.Category = g.Label
								st.Query = g.Label
							} else {
								r.GIF = g
							}
						}
					}
				})
			}
		})
		if st.Tab != 0 {
			ui.Row(c).Gap(S1).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Sent items are uploaded to your server.").FontSize(SizeCaption).TextColor(p.TextMuted).Grow(1)
				ui.Text(c, "Powered by KLIPY").FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(p.TextMuted)
			})
		}
	})
	if r.Emoji != "" || r.GIF != nil {
		st.Open = false
	}
	return r
}

// GIFCell is one tile of the GIF grid.
func GIFCell(c *ui.Context, g *GIFTile) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(g.ID)).Height(96).Radius(RadiusSm + 2).Clip().Label(g.Label)
	Pressable(b)
	b.Children(func() {
		if g.Preview != nil {
			ui.Image(c, g.Preview).Fill().Fit(ui.Cover)
		} else {
			// Placeholder: the tile's own quiet tint, never a spinner.
			ui.Box(c).Fill().Background(p.Users[HashIndex(g.ID)].Alpha(0.18).Over(p.BgInset))
		}
		if g.Category {
			ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Background(ui.RGBA(0, 0, 0, 0.32)).
				AlignItems(ui.Center).Justify(ui.Center).PassThrough().Children(func() {
				ui.Text(c, g.Label).FontSize(SizeBody).FontWeight(WeightSemibold).TextColor(ui.Hex("#FFFFFF"))
			})
		}
		if b.Hovered() || b.FocusVisible() {
			ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Radius(RadiusSm + 2).
				Border(2, p.Accent).PassThrough()
		}
	})
	return b
}

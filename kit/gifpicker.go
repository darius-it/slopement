package kit

import (
	"github.com/egoist/mygo/ui"
)

// Picker tabs, in Discord's order.
const (
	TabGIFs = iota
	TabStickers
	TabEmoji
)

// PickerTabs are the picker's kinds of content.
var PickerTabs = []string{"GIFs", "Stickers", "Emoji"}

var pickerPlaceholders = []string{"Search KLIPY", "Search stickers", "Find the perfect emoji"}

// Special tiles of the GIF tab's front page.
const (
	TileItem = iota
	TileCategory
	TileFavorites
	TileTrending
)

// PickerTile is a GIF, a sticker, or a category of the front page.
type PickerTile struct {
	ID    string
	Label string // a category's name, or the item's title for screen readers
	Kind  int    // TileItem, TileCategory, TileFavorites, TileTrending
	// Player animates the tile once loaded; Still shows meanwhile. Both nil
	// show a quiet placeholder.
	Player *ui.VideoPlayer
	Still  ui.ImageSource
	// W and H are the item's size, for its aspect ratio.
	W, H     float32
	Favorite bool
	// Query is what a category tile searches.
	Query string
}

// PickerModel is what the picker shows; a controller (package picker)
// fills it from KLIPY and the user's favorites and recents.
type PickerModel struct {
	// GIF tab front page: favorites and trending tiles, then categories.
	Front []PickerTile
	// Results is the grid of a search, a category, trending or the
	// favorites, titled Title.
	Results []PickerTile
	Title   string
	// RecentStickers lead the sticker tab.
	RecentStickers []PickerTile
	// RecentEmoji lead the emoji tab.
	RecentEmoji []string
	Loading     bool
	HasMore     bool
	// Notice explains an empty or failed state ("Add a KLIPY key…").
	Notice string
}

// PickerState is the picker's state, kept by the app.
type PickerState struct {
	SheetState
	Tab   int
	Query string
	// Browse is the GIF page open from the front page: a category's
	// query, "favorites" or "trending"; empty for the front page.
	Browse string
	hover  string // the emoji under the pointer, for the footer
}

// PickerResult is what the user did in the picker.
type PickerResult struct {
	Emoji     string
	Item      *PickerTile // a GIF or sticker to send
	ToggleFav *PickerTile // its star toggled
	More      bool        // the end of the results came into view
	// ImportDiscord asks to import favorite GIFs exported from Discord.
	ImportDiscord bool
}

// Picker is the composer's picker: GIFs, stickers and emoji. It opens as a
// popover from its button on wide windows and as a sheet on phones
// (Discord's layout, KakaoTalk's sheet).
func Picker(c *ui.Context, anchor ui.Element, st *PickerState, m *PickerModel) (r PickerResult) {
	if st.Tab < 0 || st.Tab >= len(PickerTabs) {
		st.Tab = TabGIFs
	}
	Adaptive(c, anchor, &st.SheetState, "", 420, func() {
		p := Of(c)
		tab := st.Tab
		Tabs(c.Key("picker-tabs"), &st.Tab, PickerTabs...)
		if st.Tab != tab {
			st.Query, st.Browse = "", ""
		}
		ui.Row(c).Gap(S1).AlignItems(ui.Center).Children(func() {
			if st.Tab == TabGIFs && st.Browse != "" && st.Query == "" {
				if IconButton(c, Ghost, IconChevronL, "Back").Clicked() {
					st.Browse = ""
				}
			}
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				SearchField(c.Key("picker-search"), &st.Query, pickerPlaceholders[st.Tab], "").AutoFocus()
			})
		})
		h := float32(360)
		if Compact(c) {
			_, wh := c.Size()
			h = wh * 0.5
		}
		ui.Scroll(c.Key("picker-body")).Height(h).Gap(S2).Children(func() {
			switch st.Tab {
			case TabEmoji:
				emojiTab(c, st, m, &r)
			case TabStickers:
				stickerTab(c, st, m, &r)
			default:
				gifTab(c, st, m, &r)
			}
		})
		// The footer: the emoji under the pointer, or KLIPY's credit.
		ui.Row(c).Height(24).Gap(S2).AlignItems(ui.Center).Children(func() {
			if st.Tab == TabEmoji {
				if st.hover != "" {
					ui.Text(c, st.hover).FontSize(18)
					ui.Text(c, ":"+EmojiName(st.hover)+":").FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
				}
				return
			}
			if m.Notice != "" {
				ui.Text(c, m.Notice).FontSize(SizeCaption).TextColor(p.TextMuted).Grow(1).SingleLine()
			} else {
				ui.Spacer(c)
			}
			ui.Text(c, "Powered by KLIPY").FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(p.TextFaint)
		})
	})
	if r.Emoji != "" || r.Item != nil {
		st.Open = false
	}
	return r
}

func sectionLabel(c *ui.Context, text string) {
	p := Of(c)
	ui.Text(c, text).FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(p.TextMuted).Padding(S1, S05, 0, S05)
}

func emojiTab(c *ui.Context, st *PickerState, m *PickerModel, r *PickerResult) {
	if e := EmojiGrid(c, st.Query, m.RecentEmoji, &st.hover, 9); e != "" {
		r.Emoji = e
	}
}

// EmojiGrid is the emoji to choose from: those matching query, or recently
// used then every category. hover gets the emoji under the pointer, for a
// footer naming it. It returns the one clicked.
func EmojiGrid(c *ui.Context, query string, recent []string, hover *string, cols int) (chosen string) {
	*hover = ""
	grid := func(key string, list []Emoji) {
		ui.Grid(c.Key(key)).Columns(cols).Gap(2).Children(func() {
			for _, e := range list {
				if emojiCell(c, e, hover) {
					chosen = e.Char
				}
			}
		})
	}
	if query != "" {
		found := SearchEmoji(query)
		if len(found) == 0 {
			emptyNote(c, "No emoji match “"+query+"”")
			return
		}
		grid("found", found)
		return
	}
	if len(recent) == 0 {
		recent = DefaultRecentEmoji
	}
	sectionLabel(c, "Recently used")
	list := make([]Emoji, 0, len(recent))
	for _, e := range recent {
		list = append(list, Emoji{e, EmojiName(e)})
	}
	grid("recent", list)
	for _, cat := range EmojiCategories {
		sectionLabel(c, cat.Name)
		grid(cat.Name, cat.Emojis)
	}
	return chosen
}

func emojiCell(c *ui.Context, e Emoji, hover *string) (clicked bool) {
	p := Of(c)
	b := ui.ButtonBase(c.Key(e.Char)).Height(40).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Label(e.Name)
	if b.Hovered() || b.FocusVisible() {
		b.Background(p.Hover)
		*hover = e.Char
	}
	b.Children(func() { ui.Text(c, e.Char).FontSize(24) })
	Pressable(b)
	return b.Clicked()
}

// EmojiPickerState is a standalone emoji picker's state.
type EmojiPickerState struct {
	Open  bool
	Query string
	hover string
}

// EmojiPopover is the whole emoji picker in a popover under anchor: a
// search, recently used and every category, and the hovered emoji's name.
// It closes on a choice, which it returns.
func EmojiPopover(c *ui.Context, anchor ui.Element, st *EmojiPickerState, recent []string) (chosen string) {
	p := Of(c)
	ui.PopoverBase(c, anchor, &st.Open, func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Width(360).Padding(S3).Gap(S2).Radius(RadiusMd).Background(p.Raised).Border(1, p.Border).
			Shadow(0, 12, 32, -6, p.Shadow).Transition(PopIn())
		SearchField(c.Key("emoji-search"), &st.Query, "Find the perfect emoji", "").AutoFocus()
		ui.Scroll(c.Key("emoji-body")).Height(280).Children(func() {
			chosen = EmojiGrid(c, st.Query, recent, &st.hover, 8)
		})
		ui.Row(c).Height(24).Gap(S2).AlignItems(ui.Center).Children(func() {
			if st.hover != "" {
				ui.Text(c, st.hover).FontSize(18)
				ui.Text(c, ":"+EmojiName(st.hover)+":").FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
			}
		})
	})
	if chosen != "" {
		st.Open, st.Query = false, ""
	}
	return chosen
}

func emptyNote(c *ui.Context, text string) {
	p := Of(c)
	ui.Column(c).Padding(S8, S4).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(SizeSmall).TextColor(p.TextMuted).TextAlign(ui.Center)
	})
}

func gifTab(c *ui.Context, st *PickerState, m *PickerModel, r *PickerResult) {
	if st.Query == "" && st.Browse == "" {
		// The front page: favorites, trending, then KLIPY's categories.
		ui.Grid(c.Key("front")).Columns(2).Gap(S1 + 2).Children(func() {
			for i := range m.Front {
				t := &m.Front[i]
				if categoryTile(c, t).Clicked() {
					switch t.Kind {
					case TileFavorites:
						st.Browse = "favorites"
					case TileTrending:
						st.Browse = "trending"
					default:
						st.Browse = t.Query
					}
				}
			}
		})
		if len(m.Front) == 0 {
			loadingOrNote(c, m)
		}
		return
	}
	if st.Browse == "favorites" && st.Query == "" {
		// The favorites, and bringing them over from Discord.
		ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Children(func() { sectionLabel(c, "Favorites") })
			imp := ui.ButtonBase(c.Key("import")).Height(26).PaddingX(S2).Gap(S1).Radius(RadiusSm).AlignItems(ui.Center).
				Label("Import favorites from Discord").Tooltip("A JSON file from discord-gif-exporter")
			p := Of(c)
			if imp.Hovered() {
				imp.Background(p.Hover)
			}
			imp.Children(func() {
				ui.Icon(c, IconDownload).Size(14, 14).TextColor(p.AccentStrong)
				ui.Text(c, "Import from Discord").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.AccentStrong)
			})
			r.ImportDiscord = imp.Clicked()
		})
	} else if m.Title != "" && st.Query == "" {
		sectionLabel(c, m.Title)
	}
	results(c, m, r, 2, false)
}

func stickerTab(c *ui.Context, st *PickerState, m *PickerModel, r *PickerResult) {
	if st.Query == "" && len(m.RecentStickers) > 0 {
		sectionLabel(c, "Recently used")
		ui.Grid(c.Key("recent-stickers")).Columns(4).Gap(S1).Children(func() {
			for i := range m.RecentStickers {
				if itemTile(c, &m.RecentStickers[i], r, true).Clicked() {
					r.Item = &m.RecentStickers[i]
				}
			}
		})
		sectionLabel(c, "Trending")
	}
	results(c, m, r, 4, true)
}

func loadingOrNote(c *ui.Context, m *PickerModel) {
	if m.Loading {
		emptyNote(c, "Loading…")
	} else if m.Notice != "" {
		emptyNote(c, m.Notice)
	}
}

func results(c *ui.Context, m *PickerModel, r *PickerResult, cols int, sticker bool) {
	if len(m.Results) == 0 {
		if !m.Loading && m.Notice == "" {
			emptyNote(c, "Nothing found. Try another word.")
		} else {
			loadingOrNote(c, m)
		}
		return
	}
	ui.Grid(c.Key("results")).Columns(cols).Gap(S1).Children(func() {
		for i := range m.Results {
			t := &m.Results[i]
			if itemTile(c, t, r, sticker).Clicked() {
				r.Item = t
			}
		}
	})
	if m.HasMore {
		// Scrolled near the end, the next page loads.
		end := ui.Box(c.Key("more")).Height(1)
		if end.InViewMargin(200) && !m.Loading {
			r.More = true
		}
	}
}

// tileMedia shows a tile's animation, its still, or a placeholder.
func tileMedia(c *ui.Context, t *PickerTile, fit ui.Fit, tint bool) {
	p := Of(c)
	switch {
	case t.Player != nil:
		ui.Video(c, t.Player).Fill().Fit(fit)
	case t.Still != nil:
		ui.Image(c, t.Still).Fill().Fit(fit)
	case tint:
		ui.Box(c).Fill().Background(p.Users[HashIndex(t.ID)].Alpha(0.16).Over(p.BgInset))
	}
}

// categoryTile is a tile of the GIF front page: a picture under a dark
// veil with its name, as Discord's.
func categoryTile(c *ui.Context, t *PickerTile) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(t.ID)).Height(88).Radius(RadiusSm + 2).Clip().Label(t.Label).Background(p.BgInset)
	Pressable(b)
	hasPic := t.Player != nil || t.Still != nil
	b.Children(func() {
		fg := ui.Hex("#FFFFFF")
		if hasPic {
			tileMedia(c, t, ui.Cover, false)
		} else {
			// Favorites before any are starred: a quiet tinted tile.
			ui.Box(c).Fill().Background(p.AccentSubtle)
			fg = p.AccentStrong
		}
		veil := float32(0.5)
		if b.Hovered() {
			veil = 0.36
		}
		if !hasPic {
			veil = 0
		}
		ui.Row(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Gap(S15).
			Background(ui.RGBA(10, 8, 20, b.AnimateWith("veil", veil, DurFast, EaseOut))).
			AlignItems(ui.Center).Justify(ui.Center).PassThrough().Children(func() {
			switch t.Kind {
			case TileFavorites:
				ui.Icon(c, IconStar).Size(16, 16).TextColor(fg)
			case TileTrending:
				ui.Icon(c, IconTrending).Size(16, 16).TextColor(fg)
			}
			ui.Text(c, t.Label).FontSize(SizeBody).FontWeight(WeightSemibold).TextColor(fg).SingleLine()
		})
	})
	return b
}

// itemTile is a GIF or sticker of a grid, with a star on hover to add it
// to the favorites (GIFs).
func itemTile(c *ui.Context, t *PickerTile, r *PickerResult, sticker bool) ui.Element {
	p := Of(c)
	h := float32(110)
	if sticker {
		h = 88
	}
	b := ui.ButtonBase(c.Key(t.ID)).Height(h).Radius(RadiusSm + 2).Clip().Label(t.Label)
	if !sticker {
		b.Background(p.BgInset)
	} else if b.Hovered() {
		b.Background(p.Hover)
	}
	Pressable(b)
	hov := b.Hovered() || b.FocusWithin()
	b.Children(func() {
		fit := ui.Cover
		if sticker {
			fit = ui.Contain
		}
		tileMedia(c, t, fit, !sticker)
		if !sticker && (hov || t.Favorite || Touch(c)) {
			s := ui.ButtonBase(c.Key("fav")).Size(28, 28).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(4).Left(-4).
				Background(ui.RGBA(10, 8, 20, 0.45)).Label("Favorite").Checked(t.Favorite)
			col := ui.Hex("#FFFFFF")
			if t.Favorite {
				col = p.Warning
			}
			s.Children(func() { ui.Icon(c, IconStar).Size(15, 15).TextColor(col) })
			Pressable(s)
			if s.Clicked() {
				r.ToggleFav = t
			}
		}
		if hov && !sticker {
			ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Radius(RadiusSm + 2).
				Border(2, p.Accent).PassThrough()
		}
	})
	return b
}

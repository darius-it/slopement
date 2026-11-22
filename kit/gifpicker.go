package kit

import (
	"strconv"
	"time"

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
	TileFolders // the user's own folders (Settings → GIFs)
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
	// Want starts loading the tile's animation; the tile calls it once it
	// is near the view, so a long page doesn't load all at once. Failed
	// says the load did not work: the tile shows a retry mark and a tap
	// asks again (PickerResult.Retry).
	Want   func()
	Drop   func() // takes the request back: the tile scrolled out of view
	Failed bool
	// Gone: the file is not there any more (an expired link): it says so,
	// is not retried and sends nothing. The star still takes it away.
	Gone bool
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
	// EmojiPacks and StickerPacks are the custom packs on offer, sections
	// of the emoji and sticker tabs.
	EmojiPacks, StickerPacks []PickerPack
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
	// hoverName is the shortcode of the pack image under the pointer.
	hoverName string
	// importHelp is the "where do I get this file" popover being open.
	importHelp bool
}

// PickerResult is what the user did in the picker.
type PickerResult struct {
	Emoji     string
	Item      *PickerTile // a GIF or sticker to send
	ToggleFav *PickerTile // its star toggled
	Retry     *PickerTile // a tile that failed to load, tapped
	// From the packs: a custom emoji to put in the message, a sticker to
	// send, an image to add to the user's own pack.
	CustomEmoji, PackSticker, AddToPack *PackPick
	More                                bool // the end of the results came into view
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
				} else if st.hoverName != "" {
					ui.Text(c, ":"+st.hoverName+":").FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine()
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
	if r.Emoji != "" || r.Item != nil || r.CustomEmoji != nil || r.PackSticker != nil {
		st.Open = false
	}
	return r
}

func sectionLabel(c *ui.Context, text string) {
	p := Of(c)
	ui.Text(c, text).FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(p.TextMuted).Padding(S1, S05, 0, S05)
}

func emojiTab(c *ui.Context, st *PickerState, m *PickerModel, r *PickerResult) {
	st.hoverName = ""
	// The packs' emoji lead (as a server's do in Discord), then the usual set.
	packSections(c, st, m.EmojiPacks, st.Query, false, r)
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

// discordExporterURL is the userscript that exports Discord's favorite GIFs.
const discordExporterURL = "https://github.com/darius-it/discord-gif-exporter"

// importHelpPopover explains where the file to import comes from.
func importHelpPopover(c *ui.Context, anchor ui.Element, st *PickerState) {
	p := Of(c)
	ui.PopoverBase(c, anchor, &st.importHelp, func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Width(300).Padding(S3).Gap(S2).Radius(RadiusMd).Background(p.Raised).Border(1, p.Border).
			Shadow(0, 12, 32, -6, p.Shadow).Transition(PopIn())
		ui.Text(c, "Bring your Discord favorites").FontSize(SizeBody).FontWeight(WeightSemibold)
		ui.Text(c, "In your browser, my Tampermonkey userscript exports them to a file: install it, open Discord, open its GIF picker and press “Export GIFs”. Then choose that file here.").
			FontSize(SizeSmall).TextColor(p.TextMuted)
		ui.Link(c, "github.com/darius-it/discord-gif-exporter", discordExporterURL).FontSize(SizeSmall).TextColor(p.AccentStrong)
	})
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
					case TileFolders:
						st.Browse = "folders"
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
		ui.Row(c).Gap(S05).AlignItems(ui.Center).Children(func() {
			// The label without sectionLabel's top padding, which would sit
			// it below the buttons' middle.
			ui.Text(c, "Favorites").FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(Of(c).TextMuted).Padding(0, S05).Grow(1)
			imp := ui.ButtonBase(c.Key("import")).Height(26).PaddingX(S2).Gap(S1).Radius(RadiusSm).AlignItems(ui.Center).
				Label("Import favorites from Discord").Tooltip("A JSON file from discord-gif-exporter")
			p := Of(c)
			if imp.Hovered() {
				imp.Background(p.Hover)
			}
			imp.Children(func() {
				ui.Icon(c, IconDownload).Size(14, 14).TextColor(p.AccentStrong)
				ui.Text(c, "Import from Discord").FontSize(SizeCaption).FontWeight(WeightMedium).TextColor(p.AccentStrong)
			})
			r.ImportDiscord = imp.Clicked()
			// As small as the import button beside it (26 tall), so the row
			// keeps its height.
			help := ui.ButtonBase(c.Key("import-help")).Size(26, 26).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).
				Label("How to export your Discord favorites").Tooltip("Where to get this file")
			if help.Hovered() {
				help.Background(p.Hover)
			}
			help.Children(func() { ui.Icon(c, IconHelp).Size(14, 14).TextColor(p.TextMuted) })
			Pressable(help)
			if help.Clicked() {
				st.importHelp = !st.importHelp
			}
			importHelpPopover(c, help, st)
		})
	} else if m.Title != "" && st.Query == "" {
		sectionLabel(c, m.Title)
	}
	results(c, m, r, 2, false)
}

func stickerTab(c *ui.Context, st *PickerState, m *PickerModel, r *PickerResult) {
	st.hoverName = ""
	packSections(c, st, m.StickerPacks, st.Query, true, r)
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
	if sticker {
		ui.Grid(c.Key("results")).Columns(cols).Gap(S1).Children(func() {
			for i := range m.Results {
				t := &m.Results[i]
				if itemTile(c, t, r, sticker).Clicked() {
					r.Item = t
				}
			}
		})
	} else {
		staggered(c, m, r, cols)
	}
	if m.HasMore {
		// Scrolled near the end, the next page loads.
		end := ui.Box(c.Key("more")).Height(1)
		if end.InViewMargin(200) && !m.Loading {
			r.More = true
		}
	}
}

// staggered lays GIFs out in columns whose rows do not line up: every GIF
// keeps its own shape (nothing is cropped) and goes under the shortest
// column, so the columns end about level. The order down each column is the
// order of the results.
func staggered(c *ui.Context, m *PickerModel, r *PickerResult, cols int) {
	heights := make([]float32, cols) // each column's height in widths
	assigned := make([][]int, cols)
	for i := range m.Results {
		t := &m.Results[i]
		ratio := tileRatio(t)
		short := 0
		for k := 1; k < cols; k++ {
			if heights[k] < heights[short] {
				short = k
			}
		}
		assigned[short] = append(assigned[short], i)
		heights[short] += 1 / ratio
	}
	ui.Row(c.Key("results")).Gap(S1).AlignItems(ui.Start).Children(func() {
		for k := range assigned {
			ui.Column(c.Key("col" + strconv.Itoa(k))).Grow(1).Basis(0).MinWidth(0).Gap(S1).Children(func() {
				for _, i := range assigned[k] {
					t := &m.Results[i]
					if itemTile(c, t, r, false).Clicked() {
						r.Item = t
					}
				}
			})
		}
	})
}

// tileRatio is a GIF's width over height, kept within what a tile can
// reasonably be (a very tall or very wide one is not stretched out).
func tileRatio(t *PickerTile) float32 {
	w, h := t.W, t.H
	if t.Player != nil {
		if pw, ph := t.Player.Size(); pw > 0 && ph > 0 {
			w, h = float32(pw), float32(ph)
		}
	}
	if w <= 0 || h <= 0 {
		return 4.0 / 3
	}
	return min(max(w/h, 0.5), 2.5)
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
	if t.Failed && t.Player == nil {
		// It could not load: a quiet mark that a tap tries again, or, for an
		// expired link that will never load, that it is gone.
		ui.Column(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).AlignItems(ui.Center).Justify(ui.Center).Gap(S1).PassThrough().Children(func() {
			if t.Gone {
				ui.Icon(c, IconX).Size(18, 18).TextColor(p.TextFaint)
				ui.Text(c, "No longer available").FontSize(SizeCaption).TextColor(p.TextFaint).TextAlign(ui.Center)
				return
			}
			ui.Icon(c, IconRetry).Size(18, 18).TextColor(p.TextFaint)
			ui.Text(c, "Tap to retry").FontSize(SizeCaption).TextColor(p.TextFaint)
		})
	}
}

// skeleton is a tile's loading state: a soft band of light sweeping across
// it. It moves only while the tile loads and shows (so an idle picker draws
// nothing), and stands still, a faint wash, when the system asks for less
// motion.
func skeleton(c *ui.Context) {
	p := Of(c)
	band := p.TextFaint.Alpha(0.2)
	clear := band.Alpha(0)
	b := ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).PassThrough()
	if c.Preferences().ReduceMotion {
		b.Background(p.TextFaint.Alpha(0.08))
		return
	}
	// The band's center travels from before the left edge to past the right
	// one every 1.4 s; stops are kept inside 0..1 and in order.
	x := -0.3 + 1.6*b.Loop("wave", 1400*time.Millisecond, ui.Linear)
	at := func(v float32) float32 { return min(max(v, 0), 1) }
	b.LinearGradient(ui.LinearGradient{Angle: 100, Stops: []ui.GradientStop{
		{Color: clear, At: 0}, {Color: clear, At: at(x - 0.25)}, {Color: band, At: at(x)}, {Color: clear, At: at(x + 0.25)}, {Color: clear, At: 1},
	}})
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
			case TileFolders:
				ui.Icon(c, IconFolder).Size(16, 16).TextColor(fg)
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
	b := ui.ButtonBase(c.Key(t.ID)).Radius(RadiusSm + 2).Clip().Label(t.Label)
	if sticker {
		b.Height(88)
	} else {
		// A GIF is its own shape, the width of its column.
		b.AspectRatio(tileRatio(t))
	}
	if !sticker {
		b.Background(p.BgInset)
	} else if b.Hovered() {
		b.Background(p.Hover)
	}
	Pressable(b)
	hov := b.Hovered() || b.FocusWithin()
	// Only what shows loads: a tile in the view asks for its animation (a
	// failed one waits its turn to try again), one scrolled out of it
	// takes its request back, so a page of favorites doesn't load whole.
	shown := b.InView()
	loading := false
	if t.Player == nil && t.Want != nil {
		if shown {
			t.Want()
			loading = !t.Failed
		} else if t.Drop != nil {
			t.Drop()
		}
	}
	defer func() {
		if t.Failed && !t.Gone && t.Player == nil && b.Clicked() {
			r.Retry = t
		}
	}()
	b.Children(func() {
		fit := ui.Cover
		if sticker {
			fit = ui.Contain
		}
		tileMedia(c, t, fit, !sticker)
		if loading && t.Still == nil {
			skeleton(c)
		}
		// The star is a pointer-hover affordance (touch keeps it), not a
		// permanent badge; a favorite is filled gold. Keyboard focus alone
		// doesn't show it: a click leaves focus behind.
		if !sticker && (b.Hovered() || Touch(c)) {
			s := ui.ButtonBase(c.Key("fav")).Size(28, 28).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(4).Left(-4).
				Background(ui.RGBA(10, 8, 20, 0.45)).Label("Favorite").Checked(t.Favorite)
			col, ic := ui.Hex("#FFFFFF"), IconStar
			if t.Favorite {
				col, ic = p.Warning, IconStarFilled
			}
			s.Children(func() { ui.Icon(c, ic).Size(15, 15).TextColor(col) })
			Pressable(s)
			if s.Clicked() {
				r.ToggleFav = t
			}
		}
		if hov && !sticker {
			ui.Box(c).Fill().Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Radius(RadiusSm+2).
				Border(2, p.Accent).PassThrough()
		}
	})
	return b
}

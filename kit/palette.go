package kit

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// PaletteKind is what a quick switcher item opens.
type PaletteKind int

const (
	KindRoom PaletteKind = iota
	KindPerson
	KindSpace
)

// SwitcherItem is a room, DM or space the quick switcher finds.
type SwitcherItem struct {
	ID      string
	Name    string
	Sub     string // the space it is in, or the MXID of a person
	Alias   string // also matched: #room:server, @user:server
	Kind    PaletteKind
	Avatar  AvatarSpec
	Unread  int
	Mention int
	// Recent is the rank of when it was last opened: 0 the latest.
	Recent int
}

// SwitcherState is the quick switcher's state.
type SwitcherState struct {
	Open  bool
	Query string
	Scope int // 0 all, 1 people, 2 rooms, 3 spaces
	sel   int
	moved bool
	enter bool
	// results reuses its storage from frame to frame.
	results []scored
}

type scored struct {
	i     int
	score int
}

var scopeLabels = []string{"All", "People", "Rooms", "Spaces"}

// Toggle opens the switcher, empty, or closes it.
func (st *SwitcherState) Toggle() {
	st.Open = !st.Open
	if st.Open {
		st.Query, st.sel = "", 0
	}
}

// SwitcherShortcut opens and closes the switcher with Cmd+K (Ctrl+K).
func SwitcherShortcut(c *ui.Context, st *SwitcherState) {
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		st.Toggle()
	}
}

func (st *SwitcherState) rank(items []SwitcherItem) {
	st.results = st.results[:0]
	for i := range items {
		it := &items[i]
		switch st.Scope {
		case 1:
			if it.Kind != KindPerson {
				continue
			}
		case 2:
			if it.Kind != KindRoom {
				continue
			}
		case 3:
			if it.Kind != KindSpace {
				continue
			}
		}
		s := 0
		if st.Query != "" {
			s = max(FuzzyScore(st.Query, it.Name), FuzzyScore(st.Query, it.Alias)-4)
			if s < 0 {
				continue
			}
		}
		// Mentions, then unread, then recency.
		if it.Mention > 0 {
			s += 15
		} else if it.Unread > 0 {
			s += 5
		}
		s += max(0, 12-it.Recent)
		st.results = append(st.results, scored{i, s})
	}
	slices.SortStableFunc(st.results, func(a, b scored) int { return b.score - a.score })
	if len(st.results) > 50 {
		st.results = st.results[:50]
	}
}

// Switcher is the quick switcher (Cmd/Ctrl+K): a field over the rooms,
// people and spaces, the arrows choosing and Enter opening. It has no
// open or close animation: it is used too often for one to be anything
// but a delay. It reports the item chosen, or that messages should be
// searched for the query.
func Switcher(c *ui.Context, st *SwitcherState, items []SwitcherItem, colors *UserColors) (chosen *SwitcherItem, searchMessages bool) {
	if !st.Open {
		return nil, false
	}
	p := Of(c)
	st.rank(items)
	n := len(st.results)
	if st.Query != "" {
		n++ // the last row searches messages
	}
	if st.sel >= n {
		st.sel = max(0, n-1)
	}
	compact := Compact(c)
	ui.DialogBase(c, &st.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Scrim.Alpha(0.18)).Justify(ui.Start).PaddingX(S3)
		if compact {
			backdrop.Padding(S3)
		} else {
			_, h := c.Size()
			backdrop.Padding(h*0.14, S3, S3, S3)
		}
		panel.Width(600).MaxWidthPercent(100).Radius(RadiusLg).Background(p.Raised).Border(1, p.Border).
			Shadow(0, 24, 64, -12, p.Shadow).Shadow(0, 2, 6, 0, p.Shadow).Clip().Label("Quick switcher")
		// The field.
		ui.Row(c).Height(52).PaddingX(S4).Gap(S3).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconSearch).Size(18, 18).TextColor(p.TextMuted)
			in := ui.TextInputBase(c, &st.Query).Grow(1).Placeholder("Jump to a room or person…").
				PlaceholderColor(p.TextMuted).FontSize(SizeLabel + 1).Label("Jump to").AutoFocus()
			if in.Changed() {
				st.sel = 0
			}
			in.OnTextKey(func(k ui.TextKey) bool {
				switch k.Key {
				case ui.KeyDown:
					st.sel = (st.sel + 1) % max(n, 1)
					st.moved = true
				case ui.KeyUp:
					st.sel = (st.sel - 1 + max(n, 1)) % max(n, 1)
					st.moved = true
				case ui.KeyEnter:
					st.enter = true
				case ui.KeyTab:
					d := 1
					if k.Mods&ui.Shift != 0 {
						d = len(scopeLabels) - 1
					}
					st.Scope = (st.Scope + d) % len(scopeLabels)
					st.sel = 0
				default:
					return false
				}
				return true
			})
			if !compact {
				Kbd(c, "esc")
			} else if IconButton(c, Ghost, IconX, "Close").Clicked() {
				st.Open = false
			}
		})
		ui.Row(c).PaddingX(S4).Gap(S1 + 2).PaddingY(S05).Children(func() {
			for i, l := range scopeLabels {
				if Chip(c.Key("scope"+l), l, st.Scope == i).Clicked() {
					st.Scope, st.sel = i, 0
				}
			}
		})
		ui.Box(c).Height(1).Background(p.Border).MarginY(S2)
		// The results.
		ui.Scroll(c).MaxHeight(380).PaddingX(S2).PaddingY(0).Gap(1).Children(func() {
			title := "Recent"
			if st.Query != "" {
				title = "Best matches"
			}
			if len(st.results) == 0 && st.Query != "" {
				ui.Text(c, "No rooms or people match").FontSize(SizeSmall).TextColor(p.TextMuted).Padding(S2, S3)
			} else {
				ui.Text(c, title).FontSize(SizeCaption).FontWeight(WeightSemibold).TextColor(p.TextMuted).Padding(S1, S2)
			}
			for row, r := range st.results {
				it := &items[r.i]
				if paletteRow(c, it, row == st.sel, st.moved, colors).Clicked() {
					chosen = it
				}
				if row == st.sel && st.enter {
					chosen = it
				}
			}
			if st.Query != "" {
				i := len(st.results)
				b := ui.ButtonBase(c.Key("search-messages")).Height(40).PaddingX(S2).Gap(S3).Radius(RadiusSm).AlignItems(ui.Center)
				if i == st.sel || b.Hovered() {
					b.Background(p.Hover)
				}
				if i == st.sel && st.moved {
					b.ScrollIntoView()
				}
				b.Children(func() {
					ui.Box(c).Size(28, 28).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Icon(c, IconSearch).Size(16, 16).TextColor(p.TextMuted)
					})
					ui.RichText(c,
						ui.Span{Text: "Search messages for ", Color: p.TextMuted},
						ui.Span{Text: "“" + st.Query + "”", Color: p.Text, Weight: WeightMedium},
					).FontSize(SizeBody).SingleLine()
				})
				if b.Clicked() || (i == st.sel && st.enter) {
					searchMessages = true
				}
			}
		})
		// The footer of keys, as on desktops.
		if !compact {
			ui.Row(c).Height(36).PaddingX(S4).Gap(S4).AlignItems(ui.Center).Margin(S2, 0, 0, 0).
				Background(p.BgSubtle).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Children(func() {
				hint(c, "↑↓", "Navigate")
				hint(c, "↵", "Open")
				hint(c, "tab", "Filter")
				ui.Spacer(c)
				hint(c, "esc", "Close")
			})
		} else {
			ui.Box(c).Height(S2)
		}
	})
	st.moved, st.enter = false, false
	if chosen != nil || searchMessages {
		st.Open = false
	}
	return chosen, searchMessages
}

func hint(c *ui.Context, keys, label string) {
	p := Of(c)
	ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
		Kbd(c, keys)
		ui.Text(c, label).FontSize(SizeCaption).TextColor(p.TextMuted)
	})
}

func paletteRow(c *ui.Context, it *SwitcherItem, selected, moved bool, colors *UserColors) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(it.ID)).Height(44).PaddingX(S2).Gap(S3).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Start).
		Label(it.Name).AccessSelected(selected)
	switch {
	case selected:
		b.Background(p.AccentSubtle)
	case b.Hovered():
		b.Background(p.Hover)
	}
	if selected && moved {
		b.ScrollIntoView()
	}
	b.Children(func() {
		a := it.Avatar
		a.Size, a.Colors = 28, colors
		a.Square = it.Kind == KindSpace
		if a.Name == "" {
			a.Name, a.ID = it.Name, it.ID
		}
		Avatar(c, a)
		ui.Row(c).Grow(1).MinWidth(0).Gap(S2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, it.Name).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(p.Text).SingleLine().Shrink(1)
			if it.Sub != "" {
				ui.Text(c, it.Sub).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine().Shrink(2).MinWidth(0)
			}
		})
		switch {
		case it.Mention > 0:
			Count(c, it.Mention, true)
		case it.Unread > 0:
			Count(c, it.Unread, false)
		}
		if selected {
			ui.Text(c, "↵").FontSize(SizeSmall).TextColor(p.TextMuted)
		}
	})
	return b
}

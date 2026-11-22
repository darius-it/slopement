package kit

import "github.com/egoist/mygo/ui"

// SuggestItem is one line of the composer's suggestions: a person to
// mention or an emoji to complete. Insert is what replaces the word being
// typed.
type SuggestItem struct {
	Label string // "Alex Rivera", "smile"
	Sub   string // "@alex:example.org", ":smile:"
	Emoji string // set for an emoji: shown in place of an avatar
	// Image, set for a custom emoji, gives its picture (nil while it
	// loads): shown in place of an avatar.
	Image  func() ui.ImageSource
	Avatar AvatarSpec
	Insert string
	// ID is the app's own: who a mention is of.
	ID string
}

// suggestPopup shows the suggestions in st above the field it is built in,
// the chosen one highlighted, and returns the index pressed (-1: none).
// It takes no room: it floats over what is above the composer.
func suggestPopup(c *ui.Context, st *ComposerState) (pick int) {
	pick = -1
	if len(st.Suggest) == 0 {
		return
	}
	p := Of(c)
	if st.sel >= len(st.Suggest) {
		st.sel = len(st.Suggest) - 1
	}
	ui.Column(c.Key("suggest")).Role(ui.RoleListBox).Label("Suggestions").
		Attach(ui.AnchorTopLeft, ui.AnchorBottomLeft).Top(-S1).MinWidth(260).MaxWidth(420).
		Padding(S1).Gap(1).Radius(RadiusMd).Background(p.Raised).Border(1, p.Border).
		Shadow(0, 4, 12, -4, p.Shadow).Children(func() {
		for i, it := range st.Suggest {
			// Options, not buttons: they are not focusable, so pressing one
			// never takes the keyboard focus off the field being typed in.
			b := ui.Row(c.Key("sg:" + it.Insert)).Height(rowHeightFor(c, 34)).PaddingX(S2).Gap(S2).Radius(RadiusSm).
				AlignItems(ui.Center).Role(ui.RoleOption).Label(it.Label).Checked(i == st.sel).Cursor(ui.CursorPointer)
			switch {
			case i == st.sel:
				b.Background(p.AccentSubtle)
			case b.Hovered():
				b.Background(p.Hover)
			}
			b.Children(func() {
				if it.Emoji != "" {
					ui.Text(c, it.Emoji).FontSize(20).Width(24).TextAlign(ui.Center)
				} else if it.Image != nil {
					if src := it.Image(); src != nil {
						ui.Image(c, src).Size(24, 24).Fit(ui.Contain)
					} else {
						ui.Box(c).Size(24, 24).Radius(RadiusSm).Background(p.BgInset)
					}
				} else {
					av := it.Avatar
					av.Size, av.Presence, av.Trust = 24, PresenceNone, TrustUnknown
					Avatar(c, av)
				}
				ui.Text(c, it.Label).FontSize(SizeBody).FontWeight(WeightMedium).SingleLine().Shrink(1).MinWidth(0)
				if it.Sub != "" {
					ui.Text(c, it.Sub).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine().Shrink(0)
				}
			})
			if b.Clicked() {
				pick = i
			}
		}
	})
	return pick
}

// rowHeightFor is a row's height: as given on desktops, at least 44 DIP on
// touch screens.
func rowHeightFor(c *ui.Context, h float32) float32 {
	if Touch(c) {
		return max(h, 44)
	}
	return h
}

// suggestKey handles a key in the field while suggestions show: Up and Down
// move the highlight, Tab or Enter takes the highlighted one, Escape puts
// them away until the text changes (Enter then sends, as always). It reports
// whether the key was used.
func suggestKey(st *ComposerState, k ui.TextKey) bool {
	if len(st.Suggest) == 0 || k.Mods != 0 {
		return false
	}
	switch k.Key {
	case ui.KeyDown:
		st.sel = (st.sel + 1) % len(st.Suggest)
		return true
	case ui.KeyUp:
		st.sel = (st.sel + len(st.Suggest) - 1) % len(st.Suggest)
		return true
	case ui.KeyTab, ui.KeyEnter:
		if st.OnPick != nil && st.sel < len(st.Suggest) {
			st.OnPick(st.Suggest[st.sel])
			st.Suggest, st.sel = nil, 0
		} else {
			st.wantPick = true
		}
		return true
	case ui.KeyEscape:
		st.suggestOff = st.Text
		return true
	}
	return false
}

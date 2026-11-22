package kit

import "github.com/egoist/mygo/ui"

// field styles the box around a text input: inset fill, a hairline that
// turns to the accent while the input has the focus.
func field(c *ui.Context, box ui.Element) {
	p := Of(c)
	border := p.Border
	if box.FocusWithin() {
		border = p.Accent
	} else if box.Hovered() {
		border = p.BorderStrong
	}
	box.Height(controlHeight(c)).PaddingX(S3).Gap(S2).Radius(RadiusSm).AlignItems(ui.Center).
		Background(p.BgInset).Border(1, box.AnimateColor("border", border, Color))
}

// TextField is a labeled single-line text input. The returned element is
// the input, for Submitted, Focus and AutoFocus.
func TextField(c *ui.Context, value *string, label, placeholder string) ui.Element {
	p := Of(c)
	var in ui.Element
	ui.Column(c).Gap(S15).Children(func() {
		if label != "" {
			ui.Text(c, label).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted)
		}
		box := ui.Row(c)
		box.Children(func() {
			in = ui.TextInputBase(c, value).Grow(1).Placeholder(placeholder).
				PlaceholderColor(p.TextMuted).Label(label).FontSize(fieldSize(c))
		})
		field(c, box)
	})
	return in
}

// PasswordField is TextField with the typing hidden (a password or
// passphrase).
func PasswordField(c *ui.Context, value *string, label, placeholder string) ui.Element {
	p := Of(c)
	var in ui.Element
	ui.Column(c).Gap(S15).Children(func() {
		if label != "" {
			ui.Text(c, label).FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted)
		}
		box := ui.Row(c)
		box.Children(func() {
			in = ui.TextInputBase(c, value).Grow(1).Password().Placeholder(placeholder).
				PlaceholderColor(p.TextMuted).Label(label).FontSize(fieldSize(c))
		})
		field(c, box)
	})
	return in
}

func fieldSize(c *ui.Context) float32 {
	// 16 on phones stops iOS-style zoom expectations and reads better.
	if Touch(c) {
		return SizeBodyTouch
	}
	return SizeBody
}

// SearchField is a search input with an icon, a clear button while it has
// text, and an optional shortcut hint (as "⌘K") while it is empty. It
// returns the input.
func SearchField(c *ui.Context, query *string, placeholder, hint string) ui.Element {
	p := Of(c)
	var in ui.Element
	box := ui.Row(c)
	box.Children(func() {
		ui.Icon(c, IconSearch).Size(16, 16).TextColor(p.TextMuted)
		in = ui.TextInputBase(c, query).Grow(1).Placeholder(placeholder).
			PlaceholderColor(p.TextMuted).Label(placeholder).FontSize(fieldSize(c))
		in.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEscape && *query != "" {
				k.SetText("")
				return true
			}
			return false
		})
		if *query != "" {
			x := ui.ButtonBase(c).Size(20, 20).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).Label("Clear search")
			if x.Hovered() {
				x.Background(p.Hover)
			}
			x.Children(func() { ui.Icon(c, IconX).Size(14, 14).TextColor(p.TextMuted) })
			if x.Clicked() {
				*query = ""
			}
		} else if hint != "" && !Touch(c) {
			Kbd(c, hint)
		}
	})
	field(c, box)
	return in
}

// SearchButton looks like a search field but is a button: the room list's
// "Search" that opens the quick switcher, showing its shortcut.
func SearchButton(c *ui.Context, label, hint string) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c)
	bg := p.BgInset
	if b.Pressed() {
		bg = p.Pressed.Over(p.BgInset)
	} else if b.Hovered() {
		bg = p.Hover.Over(p.BgInset)
	}
	b.Height(controlHeight(c)).PaddingX(S3).Gap(S2).Radius(RadiusSm).AlignItems(ui.Center).
		Background(b.AnimateColor("bg", bg, Color)).Border(1, p.Border).Label(label)
	b.Children(func() {
		ui.Icon(c, IconSearch).Size(16, 16).TextColor(p.TextMuted)
		ui.Text(c, label).FontSize(fieldSize(c)).TextColor(p.TextMuted).Grow(1).SingleLine()
		if hint != "" && !Touch(c) {
			Kbd(c, hint)
		}
	})
	return b
}

// Kbd shows a keyboard shortcut, as "⌘K".
func Kbd(c *ui.Context, keys string) ui.Element {
	p := Of(c)
	return pillText(c, keys, 20, SizeCaption, WeightMedium, p.TextMuted, p.Bg).PaddingX(5).Radius(5).Border(1, p.Border)
}

// CmdF is the label of find-in-chat's shortcut on this platform.
func CmdF() string {
	if ui.Cmd == ui.Super {
		return "⌘F"
	}
	return "Ctrl F"
}

// CmdK is the label of the quick switcher's shortcut on this platform.
func CmdK() string {
	if ui.Cmd == ui.Super {
		return "⌘K"
	}
	return "Ctrl K"
}

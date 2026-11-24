package kit

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// Variant is the look of a button.
type Variant int

const (
	// Primary is the one main action of a view, in the accent.
	Primary Variant = iota
	// Secondary is an outlined neutral button.
	Secondary
	// Ghost has no fill until hovered: toolbars, headers.
	Ghost
	// Danger is a destructive action.
	Danger
)

// Touch reports whether the primary pointer is a finger, for larger
// targets.
func Touch(c *ui.Context) bool {
	// mygo reports the pointer only on Linux, Windows and the web: a phone's
	// own system never says "coarse", so its OS says it for it.
	if isPhoneOS {
		return true
	}
	return c.Preferences().Pointer == ui.PointerCoarse
}

// isPhoneOS is true where the one pointer is a finger.
var isPhoneOS = runtime.GOOS == "ios" || runtime.GOOS == "android"

// SetPhoneOS makes the kit behave as on a phone's system (finger-sized
// controls, swiping messages, no hover) where the OS is a computer's: for
// tests and the gallery's phone pages. It returns what to give it back to.
func SetPhoneOS(on bool) (was bool) {
	was, isPhoneOS = isPhoneOS, on
	return was
}

// PhoneOS reports whether the app runs on a phone's system (iOS, Android),
// for what only a computer has: folders of files the app can read, say.
func PhoneOS() bool { return isPhoneOS }

// Compact reports whether the window is a phone's width.
func Compact(c *ui.Context) bool {
	w, _ := c.Size()
	return w < CompactWidth
}

func controlHeight(c *ui.Context) float32 {
	if Touch(c) {
		return TouchTarget
	}
	return 32
}

func buttonColors(p *Palette, v Variant, hovered, pressed bool) (bg, fg, border ui.Color) {
	switch v {
	case Primary:
		bg, fg = p.Accent, p.OnAccent
		if pressed {
			bg = p.AccentPressed
		} else if hovered {
			bg = p.AccentHover
		}
	case Danger:
		bg, fg = p.Danger, ui.Hex("#FFFFFF")
		if p.Dark {
			fg = ui.Hex("#1A0E0E")
		}
		if pressed {
			bg = p.Danger.Mix(p.Text, 0.15)
		} else if hovered {
			bg = p.Danger.Mix(p.Text, 0.08)
		}
	case Secondary:
		bg, fg, border = p.Raised, p.Text, p.BorderStrong
		if pressed {
			bg = p.Pressed.Over(p.Raised)
		} else if hovered {
			bg = p.Hover.Over(p.Raised)
		}
	default: // Ghost
		bg, fg = ui.Transparent, p.Text
		if pressed {
			bg = p.Pressed
		} else if hovered {
			bg = p.Hover
		}
	}
	return
}

// Button is a labeled button, with an optional leading icon.
func Button(c *ui.Context, v Variant, label string, icon *ui.SVG) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c)
	bg, fg, border := buttonColors(p, v, b.Hovered(), b.Pressed())
	b.Height(controlHeight(c)).PaddingX(S3 + S05).Gap(S15).Radius(RadiusSm).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
		Background(b.AnimateColor("bg", bg, Color)).Label(label)
	if border.A > 0 {
		b.Border(1, border)
	}
	Pressable(b)
	b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(16, 16).TextColor(fg)
		}
		ui.Text(c, label).FontSize(SizeBody).FontWeight(WeightMedium).TextColor(fg).SingleLine()
	})
	return b
}

// IconButton is a square button showing an icon alone; label names it for
// screen readers and its tooltip.
func IconButton(c *ui.Context, v Variant, icon *ui.SVG, label string) ui.Element {
	return iconButton(c, v, icon, label, true)
}

// iconButton is IconButton, with or without its own tooltip (above).
func iconButton(c *ui.Context, v Variant, icon *ui.SVG, label string, tip bool) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c)
	bg, fg, border := buttonColors(p, v, b.Hovered(), b.Pressed())
	if v == Ghost {
		fg = p.TextMuted
		if b.Hovered() {
			fg = p.Text
		}
	}
	s := controlHeight(c)
	b.Size(s, s).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
		Background(b.AnimateColor("bg", bg, Color)).Label(label)
	if tip && !Touch(c) {
		b.Tooltip(label)
	}
	if border.A > 0 {
		b.Border(1, border)
	}
	Pressable(b)
	b.Children(func() {
		ui.Icon(c, icon).Size(18, 18).TextColor(b.AnimateColor("fg", fg, Color))
	})
	return b
}

// RoundButton is a circular icon button, as the controls of a call. on is
// false for a control turned off (camera off), which shows it inset.
func RoundButton(c *ui.Context, icon *ui.SVG, label string, on bool, danger bool) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c)
	bg, fg := p.Raised, p.Text
	if !on {
		bg, fg = p.Text, p.Bg
	}
	if danger {
		bg, fg = p.Danger, ui.Hex("#FFFFFF")
	}
	if b.Pressed() {
		bg = bg.Mix(p.Text, 0.12)
	} else if b.Hovered() {
		bg = bg.Mix(p.Text, 0.06)
	}
	b.Size(48, 48).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
		Background(b.AnimateColor("bg", bg, Color)).Label(label).Checked(on)
	if !danger && on {
		b.Border(1, p.Border)
	}
	if !Touch(c) {
		b.Tooltip(label)
	}
	Pressable(b)
	b.Children(func() {
		ui.Icon(c, icon).Size(20, 20).TextColor(fg)
	})
	return b
}

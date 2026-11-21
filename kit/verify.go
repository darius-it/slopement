package kit

import "github.com/egoist/mygo/ui"

// SASEmoji is one of the seven emoji of a SAS verification, with its
// English name from the Matrix spec's table.
type SASEmoji struct {
	Emoji, Name string
}

// VerifyResult is the user's answer to an emoji comparison.
type VerifyResult int

const (
	VerifyPending VerifyResult = iota
	VerifyMatch
	VerifyMismatch
)

// SASGrid shows the emoji to compare: seven cards, four then three,
// wrapping on narrow widths, each emoji with its name under it.
func SASGrid(c *ui.Context, emoji []SASEmoji) {
	p := Of(c)
	ui.Row(c).Wrap().Gap(S2).Justify(ui.Center).Children(func() {
		for i, e := range emoji {
			ui.Column(c.Key(i)).Width(72).Padding(S3, S1).Gap(S1).Radius(RadiusMd).Background(p.BgInset).
				AlignItems(ui.Center).Label(e.Name).Children(func() {
				ui.Text(c, e.Emoji).FontSize(32).LineHeight(1.1)
				ui.Text(c, e.Name).FontSize(SizeCaption).FontWeight(WeightMedium).TextColor(p.TextMuted).
					SingleLine().TextAlign(ui.Center)
			})
		}
	})
}

// VerifyEmoji is the whole comparison step: a title, what to do, the
// grid, and two equal answers, "They don't match" first so the safe
// choice isn't the one under the thumb by habit.
func VerifyEmoji(c *ui.Context, other string, emoji []SASEmoji) (res VerifyResult) {
	p := Of(c)
	ui.Column(c).Gap(S4).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Gap(S1).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconShield).Size(28, 28).TextColor(p.Accent)
			ui.Text(c, "Compare emoji").FontSize(SizeTitle).FontWeight(WeightSemibold).TextAlign(ui.Center)
			ui.Text(c, "Confirm the emoji below appear on "+other+", in the same order.").
				FontSize(SizeBody).TextColor(p.TextMuted).TextAlign(ui.Center).MaxWidth(360)
		})
		SASGrid(c, emoji)
		ui.Row(c).Gap(S2).Children(func() {
			if Button(c, Secondary, "They don't match", nil).Grow(1).Clicked() {
				res = VerifyMismatch
			}
			if Button(c, Primary, "They match", IconCheck).Grow(1).Clicked() {
				res = VerifyMatch
			}
		})
	})
	return res
}

// VerifyChoice is how to verify this session after login.
type VerifyChoice int

const (
	VerifyNone VerifyChoice = iota
	VerifyWithDevice
	VerifyWithKey
	VerifyReset
)

// VerifyStart asks how to verify this session: with another device (the
// emoji), or with the Security Key or Phrase; resetting is a quiet link.
func VerifyStart(c *ui.Context) (ch VerifyChoice) {
	p := Of(c)
	ui.Column(c).Gap(S4).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Gap(S1).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconShield).Size(28, 28).TextColor(p.Accent)
			ui.Text(c, "Verify this device").FontSize(SizeTitle).FontWeight(WeightSemibold)
			ui.Text(c, "Verify it to read your encrypted messages and to show others it's really you.").
				FontSize(SizeBody).TextColor(p.TextMuted).TextAlign(ui.Center).MaxWidth(340)
		})
		if choiceRow(c, IconMessage, "Use another device", "Compare emoji with a device you're signed in on").Clicked() {
			ch = VerifyWithDevice
		}
		if choiceRow(c, IconKey, "Use your Security Key", "Or your Security Phrase").Clicked() {
			ch = VerifyWithKey
		}
		l := ui.ButtonBase(c).AlignSelf(ui.Center).Padding(S1, S2).Radius(RadiusSm).Label("Can't verify? Reset")
		fg := p.TextMuted
		if l.Hovered() {
			fg = p.Text
		}
		l.Children(func() { ui.Text(c, "Can't verify?").FontSize(SizeSmall).TextColor(fg).Underline() })
		if l.Clicked() {
			ch = VerifyReset
		}
	})
	return ch
}

// choiceRow is a large option row with an icon, a title and a subtitle.
func choiceRow(c *ui.Context, icon *ui.SVG, title, sub string) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(title))
	bg, bd := p.Raised, p.Border
	if b.Pressed() {
		bg = p.Pressed.Over(p.Raised)
	} else if b.Hovered() {
		bg, bd = p.Hover.Over(p.Raised), p.BorderStrong
	}
	b.Padding(S3).Gap(S3).Radius(RadiusMd).AlignItems(ui.Center).
		Background(b.AnimateColor("bg", bg, Color)).Border(1, b.AnimateColor("bd", bd, Color)).Label(title)
	b.Children(func() {
		ui.Box(c).Size(36, 36).Radius(RadiusSm+2).Background(p.AccentSubtle).AlignItems(ui.Center).Justify(ui.Center).
			Children(func() { ui.Icon(c, icon).Size(18, 18).TextColor(p.AccentStrong) })
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(SizeBody).FontWeight(WeightMedium)
			ui.Text(c, sub).FontSize(SizeSmall).TextColor(p.TextMuted)
		})
		ui.Icon(c, IconChevronR).Size(16, 16).TextColor(p.TextFaint)
	})
	return b
}

// ChoiceRow is exported for other screens (login: server choice, etc.).
func ChoiceRow(c *ui.Context, icon *ui.SVG, title, sub string) ui.Element {
	return choiceRow(c, icon, title, sub)
}

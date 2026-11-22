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

const (
	sasCell     = 72
	sasRowWidth = 7*sasCell + 6*S2 // the seven emoji in one row
	// VerifyStartWidth is VerifyStart's width.
	VerifyStartWidth = 400
)

// SASGrid shows the emoji to compare: seven cards in a row, wrapping on
// narrow widths (phones), each emoji with its name under it.
func SASGrid(c *ui.Context, emoji []SASEmoji) {
	p := Of(c)
	ui.Row(c).Wrap().Gap(S2).Justify(ui.Center).Children(func() {
		for i, e := range emoji {
			ui.Column(c.Key(i)).Width(sasCell).Padding(S3, S1).Gap(S1).Radius(RadiusMd).Background(p.BgInset).
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
	// As wide as the seven emoji on one row, so they never wrap 5 + 2:
	// the order is what the user compares.
	ui.Column(c).Width(sasRowWidth).MaxWidthPercent(100).Gap(S4).AlignItems(ui.Stretch).Children(func() {
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
	ui.Column(c).Width(VerifyStartWidth).MaxWidthPercent(100).Gap(S4).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Gap(S1).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconShield).Size(28, 28).TextColor(p.Accent)
			ui.Text(c, "Verify this device").FontSize(SizeTitle).FontWeight(WeightSemibold)
			ui.Text(c, "Verify it to read your encrypted messages and to show others it's really you.").
				FontSize(SizeBody).TextColor(p.TextMuted).TextAlign(ui.Center).MaxWidth(340)
		})
		// The two ways are one group, close together; the way out sits well
		// apart below them.
		ui.Column(c).Gap(S1).Children(func() {
			if choiceRow(c, IconMessage, "Use another device", "Compare emoji with a device you're signed in on").Clicked() {
				ch = VerifyWithDevice
			}
			if choiceRow(c, IconKey, "Use your Security Key", "Or your Security Phrase").Clicked() {
				ch = VerifyWithKey
			}
		})
		l := ui.ButtonBase(c).AlignSelf(ui.Center).Padding(S1, S2).Radius(RadiusSm).Label("Can't verify? Reset").Margin(S2, 0, 0, 0)
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

// VerifyAsk is an incoming request: who asks, and the two answers —
// decline first, so the safe choice isn't the one under the thumb by
// habit.
func VerifyAsk(c *ui.Context, from string) (accept, decline bool) {
	p := Of(c)
	ui.Column(c).Width(VerifyStartWidth).MaxWidthPercent(100).Gap(S4).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Gap(S1).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconShield).Size(28, 28).TextColor(p.Accent)
			ui.Text(c, "Verify this device").FontSize(SizeTitle).FontWeight(WeightSemibold)
			ui.Text(c, from+" asks to verify this device. Only agree if you started it there yourself.").
				FontSize(SizeBody).TextColor(p.TextMuted).TextAlign(ui.Center).MaxWidth(340)
		})
		ui.Row(c).Gap(S2).Children(func() {
			if Button(c, Secondary, "Decline", nil).Grow(1).Clicked() {
				decline = true
			}
			if Button(c, Primary, "Accept", IconCheck).Grow(1).Clicked() {
				accept = true
			}
		})
	})
	return accept, decline
}

// VerifyWait: we started it; the other side has to accept. cancel backs
// out.
func VerifyWait(c *ui.Context) (cancel bool) {
	p := Of(c)
	ui.Column(c).Width(VerifyStartWidth).MaxWidthPercent(100).Gap(S4).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, IconShield).Size(28, 28).TextColor(p.Accent)
		ui.Text(c, "Waiting for your other device").FontSize(SizeTitle).FontWeight(WeightSemibold).TextAlign(ui.Center)
		ui.Text(c, "Accept the request there — it may need a tap — and the emoji appear here.").
			FontSize(SizeBody).TextColor(p.TextMuted).TextAlign(ui.Center).MaxWidth(340)
		if Button(c, Secondary, "Cancel", nil).Clicked() {
			cancel = true
		}
	})
	return cancel
}

// VerifyKey asks for the Security Key; submitted reports Restore pressed
// (or Enter). busy swaps the button for a working line.
func VerifyKey(c *ui.Context, key *string, busy bool) (submitted bool) {
	p := Of(c)
	ui.Column(c).Width(VerifyStartWidth).MaxWidthPercent(100).Gap(S4).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Gap(S1).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconKey).Size(28, 28).TextColor(p.Accent)
			ui.Text(c, "Use your Security Key").FontSize(SizeTitle).FontWeight(WeightSemibold)
			ui.Text(c, "In Element: Settings → Security & Privacy → Security Key. Your Security Phrase works too.").
				FontSize(SizeBody).TextColor(p.TextMuted).TextAlign(ui.Center).MaxWidth(340)
		})
		var in ui.Element
		ui.Column(c).Gap(S15).Children(func() {
			ui.Text(c, "Security Key").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted)
			box := ui.Row(c)
			box.Children(func() {
				in = ui.TextInputBase(c, key).Grow(1).Password().
					Placeholder("EsTc WLwb Z5P1…").PlaceholderColor(p.TextMuted).
					Label("Security Key").FontSize(fieldSize(c)).AutoFocus()
			})
			field(c, box)
		})
		if in.Submitted() {
			submitted = true
		}
		if busy {
			ui.Text(c, "Reading the key…").FontSize(SizeSmall).TextColor(p.TextMuted)
		} else if Button(c, Primary, "Restore", IconKey).Clicked() {
			submitted = true
		}
	})
	return submitted
}

// choiceRow is a large option row with an icon, a title and a subtitle.
// It sits inside a group card or a sheet, so it is flat — no background
// or border of its own (no cards in cards); hover and press tint over
// what is behind it.
func choiceRow(c *ui.Context, icon *ui.SVG, title, sub string) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(title))
	bg := ui.Transparent
	switch {
	case b.Pressed():
		bg = p.Pressed
	case b.Hovered():
		bg = p.Hover
	}
	b.Padding(S3).Gap(S3).Radius(RadiusMd - 2).AlignItems(ui.Center).
		Background(b.AnimateColor("bg", bg, Color)).Label(title)
	b.Children(func() {
		ui.Box(c).Size(36, 36).Radius(RadiusSm + 2).Background(p.AccentSubtle).AlignItems(ui.Center).Justify(ui.Center).
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

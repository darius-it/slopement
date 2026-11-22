package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Status is a user's custom status: an emoji and a short text, until a
// time or for good. It shows under their name in their DM's header and the
// member list, beside it in room rows, and in their profile card.
type Status struct {
	Emoji string
	Text  string
	Until time.Time // zero: until cleared
}

// Set reports whether there is a status to show at now (not expired).
func (s Status) Set(now time.Time) bool {
	return (s.Emoji != "" || s.Text != "") && (s.Until.IsZero() || now.Before(s.Until))
}

// StatusMaxLen is the longest status text.
const StatusMaxLen = 80

// ClearAfter are when a status can clear itself (Slack's and Discord's).
var ClearAfter = []string{"30 minutes", "1 hour", "4 hours", "Today", "This week", "Don't clear"}

// ClearTime returns when a status set at now clears, for ClearAfter[i];
// zero for never.
func ClearTime(i int, now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch i {
	case 0:
		return now.Add(30 * time.Minute)
	case 1:
		return now.Add(time.Hour)
	case 2:
		return now.Add(4 * time.Hour)
	case 3:
		return day.AddDate(0, 0, 1)
	case 4:
		// The end of the week: the coming Monday's start.
		days := (8 - int(now.Weekday())) % 7
		if days == 0 {
			days = 7
		}
		return day.AddDate(0, 0, days)
	}
	return time.Time{}
}

// StatusDraft is the status being edited.
type StatusDraft struct {
	SheetState
	Emoji string
	Text  string
	Clear int
	emoji EmojiPickerState
}

// Edit loads s into the draft and opens it.
func (d *StatusDraft) Edit(s Status) {
	d.Emoji, d.Text, d.Clear, d.Open = s.Emoji, s.Text, 3, true
	if s.Emoji == "" {
		d.Emoji = "💬"
	}
}

// Status returns the draft as a status set at now.
func (d *StatusDraft) Status(now time.Time) Status {
	return Status{Emoji: d.Emoji, Text: d.Text, Until: ClearTime(d.Clear, now)}
}

// StatusAction is what the editor asked for.
type StatusAction struct {
	Save, Clear bool
}

// StatusEditor sets the user's status: a preview of how others see it,
// the emoji and text, and when it clears (Discord's on Mobbin). No
// suggested statuses: it's the user's own words. A dialog on desktops, a
// sheet on phones.
func StatusEditor(c *ui.Context, d *StatusDraft, me AvatarSpec, name string, now time.Time) (a StatusAction) {
	p := Of(c)
	form := func() {
		// What others will see: always one line under the name, the
		// status as typed or a faint placeholder in its place, so the
		// dialog never changes height as the user types.
		ui.Row(c).Gap(S3).Padding(S3).Radius(RadiusMd).Background(p.BgInset).AlignItems(ui.Center).Children(func() {
			av := me
			av.Size, av.Name = 40, name
			Avatar(c, av)
			ui.Column(c).MinWidth(0).Grow(1).Gap(1).Children(func() {
				ui.Text(c, name).FontSize(SizeBody).FontWeight(WeightSemibold)
				ui.Row(c).Height(18).AlignItems(ui.Center).MinWidth(0).Children(func() {
					if hasText(d.Text) {
						StatusLine(c, d.Status(now), now, true)
					} else {
						ui.RichText(c, ui.Span{Text: d.Emoji + " "}, ui.Span{Text: "What's happening?", Color: p.TextFaint}).
							FontSize(SizeSmall).SingleLine()
					}
				})
			})
		})
		// Emoji and text: the emoji opens the whole emoji picker.
		box := ui.Row(c)
		box.Children(func() {
			e := ui.ButtonBase(c.Key("emoji")).Size(28, 28).Radius(RadiusSm).AlignItems(ui.Center).Justify(ui.Center).
				Label("Status emoji").Tooltip("Change emoji")
			if e.Hovered() || d.emoji.Open {
				e.Background(p.Hover)
			}
			e.Children(func() { ui.Text(c, d.Emoji).FontSize(18) })
			if e.Clicked() {
				d.emoji.Open = !d.emoji.Open
			}
			if em := EmojiPopover(c, e, &d.emoji, nil); em != "" {
				d.Emoji = em
			}
			in := ui.TextInputBase(c.Key("text"), &d.Text).Grow(1).MaxLength(StatusMaxLen).
				Placeholder("What's happening?").PlaceholderColor(p.TextMuted).FontSize(fieldSize(c)).Label("Status").AutoFocus()
			if in.Submitted() && hasText(d.Text) {
				a.Save = true
			}
			if d.Text != "" {
				x := ui.ButtonBase(c.Key("x")).Size(20, 20).Radius(RadiusFull).AlignItems(ui.Center).Justify(ui.Center).Label("Clear text")
				if x.Hovered() {
					x.Background(p.Hover)
				}
				x.Children(func() { ui.Icon(c, IconX).Size(14, 14).TextColor(p.TextMuted) })
				if x.Clicked() {
					d.Text = ""
				}
			}
		})
		field(c, box)
		// When it clears.
		ui.Column(c).Gap(S15).Children(func() {
			ui.Text(c, "Clear after").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.TextMuted)
			ui.Row(c).Gap(S1 + 2).Wrap().Children(func() {
				for i, l := range ClearAfter {
					if Chip(c.Key("clr"+l), l, d.Clear == i).Clicked() {
						d.Clear = i
					}
				}
			})
		})
		ui.Row(c).Gap(S2).Children(func() {
			if Button(c, Ghost, "Clear status", nil).Clicked() {
				a.Clear = true
			}
			ui.Spacer(c)
			if Button(c, Primary, "Save", nil).Disabled(!hasText(d.Text)).Clicked() {
				a.Save = true
			}
		})
	}
	if Compact(c) {
		Sheet(c, &d.SheetState, "Set a status", form)
	} else {
		Dialog(c, &d.Open, "Set a status", 420, form)
	}
	if a.Save || a.Clear {
		d.Open = false
	}
	return a
}

// StatusLine is a status in one quiet line: its emoji and text, with when
// it clears when full is set ("until 15:00"). Set full only for the
// user's own status: when someone's status clears is theirs to know.
func StatusLine(c *ui.Context, s Status, now time.Time, full bool) ui.Element {
	p := Of(c)
	if !s.Set(now) {
		return ui.Element{}
	}
	text := s.Text
	if full && !s.Until.IsZero() {
		text += " · until " + untilText(s.Until, now)
	}
	// No emoji, no leading gap: the words start where the line does.
	if s.Emoji == "" {
		return ui.RichText(c, ui.Span{Text: text, Color: p.TextMuted}).FontSize(SizeSmall).SingleLine()
	}
	return ui.RichText(c,
		ui.Span{Text: s.Emoji + " "},
		ui.Span{Text: text, Color: p.TextMuted},
	).FontSize(SizeSmall).SingleLine()
}

func untilText(t, now time.Time) string {
	// A status that clears at midnight ("Today", "This week") names the
	// day it ends on, without a time.
	if t.Hour() == 0 && t.Minute() == 0 {
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		switch {
		case t.Equal(tomorrow):
			return "tomorrow"
		case t.Sub(now) < 7*24*time.Hour:
			return t.Format("Monday")
		}
		return t.Format("2 Jan")
	}
	if t.YearDay() == now.YearDay() && t.Year() == now.Year() {
		return t.Format("15:04")
	}
	if t.Sub(now) < 6*24*time.Hour {
		return t.Format("Mon 15:04")
	}
	return t.Format("2 Jan")
}

// ProfileSpec is a user's profile card.
type ProfileSpec struct {
	Name, MXID string
	Avatar     AvatarSpec
	Status     Status
	Trust      Trust
	Pronouns   string
	Self       bool // the user's own card: edit the status, no Message or Call
	// Recolor shows the row to choose the name's color; ColorIndex is the
	// one chosen now (-1 automatic).
	Recolor    bool
	ColorIndex int
	// Ignorable shows Ignore (or Unignore, when Ignored) under the buttons.
	Ignorable, Ignored bool
}

// ProfileAction is what the card asked for.
type ProfileAction struct {
	Message, Call, Verify, EditStatus bool
	// Ignore and Unignore are explicit, not a toggle: a card may be built
	// twice in a frame, and an action must do the same the second time.
	Ignore, Unignore bool
	// SetColor: the name's color was chosen (Color: an index of the
	// palette, -1 for automatic).
	SetColor bool
	Color    int
}

// ProfileCard is the card a user's avatar or name opens: their picture,
// name, ID, status, trust, and quick actions. A popover on desktops.
func ProfileCard(c *ui.Context, s ProfileSpec, colors *UserColors, now time.Time) (a ProfileAction) {
	p := Of(c)
	ui.Column(c.Key("profile:" + s.MXID)).Width(300).Gap(S3).Children(func() {
		ui.Row(c).Gap(S3).AlignItems(ui.Center).Children(func() {
			av := s.Avatar
			av.Size, av.Name, av.ID, av.Colors, av.Trust = 56, s.Name, s.MXID, colors, s.Trust
			Avatar(c, av)
			ui.Column(c).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, s.Name).FontSize(SizeTitle - 1).FontWeight(WeightSemibold).TextColor(colors.Color(p, s.MXID)).SingleLine()
				sub := s.MXID
				if s.Pronouns != "" {
					sub += " · " + s.Pronouns
				}
				ui.Text(c, sub).FontSize(SizeSmall).TextColor(p.TextMuted).SingleLine().Selectable()
			})
		})
		if s.Status.Set(now) {
			// The status in a soft bubble, as Discord's profile shows it.
			ui.Row(c).Padding(S2, S3).Radius(RadiusMd).Background(p.BgInset).Children(func() {
				StatusLine(c, s.Status, now, false).Grow(1)
			})
		}
		if s.Recolor {
			ui.Row(c.Key("recolor")).Gap(S2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Name colour").FontSize(SizeSmall).TextColor(p.TextMuted).Grow(1).SingleLine()
				if i, ok := ColorDots(c, s.ColorIndex, s.MXID); ok {
					a.SetColor, a.Color = true, i
				}
			})
		}
		if s.Trust == TrustWarning {
			ui.Row(c).Gap(S2).Padding(S2, S3).Radius(RadiusMd).Background(p.DangerSubtl).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, IconShieldX).Size(16, 16).TextColor(p.Danger)
				ui.Text(c, "Not verified").FontSize(SizeSmall).FontWeight(WeightMedium).TextColor(p.Danger).Grow(1)
				a.Verify = Button(c, Secondary, "Verify", nil).Height(26).Clicked()
			})
		}
		ui.Row(c).Gap(S2).Children(func() {
			if s.Self {
				a.EditStatus = Button(c, Primary, "Edit status", IconSmile).Grow(1).Clicked()
				return
			}
			a.Message = Button(c, Primary, "Message", IconMessage).Grow(1).Clicked()
			a.Call = IconButton(c, Secondary, IconPhone, "Call").Clicked()
		})
		if s.Ignorable && !s.Self {
			label := "Ignore"
			if s.Ignored {
				label = "Unignore"
			}
			ui.Row(c).Justify(ui.Center).Children(func() {
				if Button(c, Ghost, label, nil).Height(26).Clicked() {
					if s.Ignored {
						a.Unignore = true
					} else {
						a.Ignore = true
					}
				}
			})
		}
	})
	return a
}

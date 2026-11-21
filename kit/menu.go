package kit

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// MenuStyle is the kit's look for menus MyGo draws in the window: the
// raised surface, a hairline, the one soft shadow, rows on the 4 grid.
func MenuStyle(c *ui.Context) ui.PopoverMenuOptions {
	p := Of(c)
	return ui.PopoverMenuOptions{
		Panel: func(panel ui.Element) {
			panel.Padding(S1).Radius(RadiusMd).Background(p.Raised).Border(1, p.Border).
				Shadow(0, 8, 24, -4, p.Shadow).Shadow(0, 1, 2, 0, p.Shadow).MinWidth(200).
				Transition(PopIn())
		},
		Item: func(item ui.Element) {
			item.Radius(RadiusSm)
		},
	}
}

// MuteFor is a mute duration; 0 is until turned back on.
type MuteFor time.Duration

// MuteChoices are the durations offered to mute a room.
var MuteChoices = []struct {
	Label string
	For   MuteFor
}{
	{"For 15 minutes", MuteFor(15 * time.Minute)},
	{"For 2 hours", MuteFor(2 * time.Hour)},
	{"For 1 day", MuteFor(24 * time.Hour)},
	{"For 1 week", MuteFor(7 * 24 * time.Hour)},
	{"Until I turn it back on", 0},
}

// MutedUntilText describes a mute ending at until (zero: forever) relative
// to now, as "Muted until 14:30", "Muted until Tue 09:00", "Muted".
func MutedUntilText(until, now time.Time) string {
	if until.IsZero() {
		return "Muted"
	}
	if until.YearDay() == now.YearDay() && until.Year() == now.Year() {
		return "Muted until " + until.Format("15:04")
	}
	if until.Sub(now) < 6*24*time.Hour {
		return "Muted until " + until.Format("Mon 15:04")
	}
	return "Muted until " + until.Format("2 Jan")
}

// RoomMenuState is what the room menu shows checked.
type RoomMenuState struct {
	Muted      bool
	MutedUntil time.Time
	Favourite  bool
	LowPrio    bool
	Unread     bool
}

// RoomAction is what the room menu chose.
type RoomAction struct {
	Mute       *MuteFor // set to mute, for its duration
	Unmute     bool
	ToggleFav  bool
	ToggleLow  bool
	MarkRead   bool
	MarkUnread bool
	CopyLink   bool
	Settings   bool
	Leave      bool
}

// RoomMenu fills m with a room's actions and reports the one chosen. Use it
// as a row's PopoverMenu, its ⋯ button's and the room header's.
func RoomMenu(m *ui.Menu, s RoomMenuState, now time.Time) (a RoomAction) {
	if s.Unread {
		if m.Item("Mark as read").Chosen() {
			a.MarkRead = true
		}
	} else if m.Item("Mark as unread").Chosen() {
		a.MarkUnread = true
	}
	fav := "Add to favourites"
	if s.Favourite {
		fav = "Remove from favourites"
	}
	if m.Item(fav).Checked(s.Favourite).Chosen() {
		a.ToggleFav = true
	}
	if s.Muted {
		if m.Item("Unmute").Subtitle(MutedUntilText(s.MutedUntil, now)).Chosen() {
			a.Unmute = true
		}
	} else {
		m.Submenu("Mute", func(m *ui.Menu) {
			for _, ch := range MuteChoices {
				it := m.Item(ch.Label)
				if ch.For != 0 {
					it.Trailing(now.Add(time.Duration(ch.For)).Format(untilLayout(time.Duration(ch.For))))
				}
				if it.Chosen() {
					d := ch.For
					a.Mute = &d
				}
			}
		})
	}
	if m.Item("Low priority").Checked(s.LowPrio).Chosen() {
		a.ToggleLow = true
	}
	m.Separator()
	if m.Item("Copy link").Chosen() {
		a.CopyLink = true
	}
	if m.Item("Settings").Chosen() {
		a.Settings = true
	}
	m.Separator()
	if m.Item("Leave").Danger().Chosen() {
		a.Leave = true
	}
	return a
}

func untilLayout(d time.Duration) string {
	if d < 24*time.Hour {
		return "15:04"
	}
	if d < 7*24*time.Hour {
		return "Mon 15:04"
	}
	return "2 Jan"
}

// MuteList is the mute choices as a list, for a phone's sheet: each row
// says until when it mutes. It reports the choice, if any.
func MuteList(c *ui.Context, now time.Time) (chosen *MuteFor) {
	p := Of(c)
	ui.Column(c).Radius(RadiusMd).Background(p.Raised).Border(1, p.Border).Clip().Children(func() {
		for i, ch := range MuteChoices {
			if i > 0 {
				ui.Box(c).Height(1).Background(p.Border).MarginStart(S4)
			}
			b := ui.ButtonBase(c.Key(ch.Label))
			bg := ui.Transparent
			if b.Pressed() {
				bg = p.Pressed
			} else if b.Hovered() {
				bg = p.Hover
			}
			b.MinHeight(TouchTarget + 4).PaddingX(S4).Gap(S3).AlignItems(ui.Center).
				Background(b.AnimateColor("bg", bg, Color)).Label(ch.Label)
			b.Children(func() {
				ui.Text(c, ch.Label).FontSize(SizeLabel).TextColor(p.Text).Grow(1)
				if ch.For != 0 {
					ui.Text(c, fmt.Sprint("until ", now.Add(time.Duration(ch.For)).Format(untilLayout(time.Duration(ch.For))))).
						FontSize(SizeSmall).FontFeatures("tnum").TextColor(p.TextMuted)
				}
			})
			if b.Clicked() {
				d := ch.For
				chosen = &d
			}
		}
	})
	return chosen
}

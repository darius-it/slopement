package kit

import (
	"github.com/egoist/mygo/ui"
)

// RoomRowData is what a room list row shows.
type RoomRowData struct {
	ID      string
	Name    string
	Avatar  AvatarSpec
	Preview string // last message, "Alice: hi"
	Time    string // "14:02", "Yesterday"
	Unread  int
	Mention int // highlights
	Muted   bool
	Fav     bool
	// Typing replaces the preview while someone types.
	Typing string
	// Call is set while a call is going on in the room.
	Call bool
	// Encrypted shows a small lock beside the name.
	Encrypted bool
}

// RowState is how the pointer and selection touch a row, from the row
// element or, inside a ui.List, from ui.ListRow.
type RowState struct {
	Hovered, Pressed, Selected, Focused bool
}

// rowBg is the background of a list row in state s.
func rowBg(p *Palette, s RowState) ui.Color {
	switch {
	case s.Selected:
		return p.AccentSubtle
	case s.Pressed:
		return p.Pressed
	case s.Hovered:
		return p.Hover
	}
	return ui.Transparent
}

// RoomRowContent builds the inside of a room row; RoomRow wraps it in a
// button, a ui.List wraps it in its rows.
func RoomRowContent(c *ui.Context, r RoomRowData, s RowState) {
	p := Of(c)
	unread := r.Unread > 0 || r.Mention > 0
	name, meta := p.Text, p.TextMuted
	weight := WeightMedium
	if unread && !r.Muted {
		weight = WeightSemibold
	}
	if r.Muted && !s.Selected {
		name = p.TextMuted
	}
	a := r.Avatar
	if a.Size == 0 {
		a.Size = 36
	}
	if a.Name == "" {
		a.Name, a.ID = r.Name, r.ID
	}
	Avatar(c, a)
	ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
		ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
			ui.Text(c, r.Name).FontSize(SizeBody).FontWeight(weight).TextColor(name).SingleLine().Shrink(1).MinWidth(0)
			if r.Encrypted {
				ui.Icon(c, IconLock).Size(12, 12).TextColor(p.TextFaint)
			}
			ui.Spacer(c)
			if r.Call {
				ui.Icon(c, IconPhone).Size(13, 13).TextColor(p.Success)
			}
			ui.Text(c, r.Time).FontSize(SizeCaption).FontFeatures("tnum").TextColor(meta).SingleLine().Shrink(0)
		})
		ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
			preview, pc := r.Preview, p.TextMuted
			if r.Typing != "" {
				preview, pc = r.Typing, p.AccentStrong
			}
			ui.Text(c, preview).FontSize(SizeSmall).TextColor(pc).SingleLine().Grow(1).MinWidth(0)
			if r.Muted {
				ui.Icon(c, IconBellOff).Size(13, 13).TextColor(p.TextFaint)
			}
			switch {
			case r.Mention > 0:
				Count(c, r.Mention, true)
			case r.Unread > 0 && r.Muted:
				Dot(c, false)
			case r.Unread > 0:
				Count(c, r.Unread, false)
			}
		})
	})
}

// RoomRow is a room of the room list: a button showing RoomRowContent,
// selected while its room is open. menu, if set, is the room's actions:
// it opens on a right click (as a popover drawn by the kit) and from a ⋯
// button that shows on hover.
func RoomRow(c *ui.Context, r RoomRowData, selected bool, menu func(m *ui.Menu)) ui.Element {
	p := Of(c)
	b := ui.ButtonBase(c.Key(r.ID))
	s := RowState{Hovered: b.Hovered(), Pressed: b.Pressed(), Selected: selected}
	h := float32(RowHeight)
	if Touch(c) {
		h = 64
	}
	b.Height(h).PaddingX(S2).Gap(S3).Radius(RadiusMd-2).AlignItems(ui.Center).
		Background(b.AnimateColor("bg", rowBg(p, s), Color)).Selected(selected).Label(r.Name).
		FocusRingOffset(-2)
	if menu != nil {
		b.PopoverMenu(menu, MenuStyle(c))
	}
	b.Children(func() { RoomRowContent(c, r, s) })
	return b
}

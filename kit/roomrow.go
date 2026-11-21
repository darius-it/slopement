package kit

import (
	"time"

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
	// Encrypted is set for encrypted rooms, the norm: nothing is shown for
	// it, only the header of an unencrypted room says so.
	Encrypted bool
	// Status is a DM partner's custom status: its emoji shows beside the
	// name, its text on hover.
	Status Status
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
	now := time.Now()
	status := r.Status.Set(now)
	// The avatar, with the person's status as a small thought bubble on
	// its corner, trailed by two dots down to it.
	ui.Box(c).Shrink(0).Children(func() {
		Avatar(c, a)
		if status && r.Status.Emoji != "" {
			miniThought(c, r.Status)
		}
	})
	ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
		ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
			ui.Text(c, r.Name).FontSize(SizeBody).FontWeight(weight).TextColor(name).SingleLine().Shrink(1).MinWidth(0)
			ui.Spacer(c)
			if r.Call {
				ui.Icon(c, IconPhone).Size(13, 13).TextColor(p.Success)
			}
			ui.Text(c, r.Time).FontSize(SizeCaption).FontFeatures("tnum").TextColor(meta).SingleLine().Shrink(0)
		})
		ui.Row(c).Gap(S15).AlignItems(ui.Center).Children(func() {
			preview, pc := r.Preview, p.TextMuted
			switch {
			case r.Typing != "":
				preview, pc = r.Typing, p.AccentStrong
			}
			ui.Text(c.Key("preview"), preview).FontSize(SizeSmall).TextColor(pc).SingleLine().Grow(1).MinWidth(0)
			// One mark at most: a mention beats a mute beats a count.
			switch {
			case r.Mention > 0:
				Count(c, r.Mention, true)
			case r.Muted:
				ui.Icon(c, IconBellOff).Size(13, 13).TextColor(p.TextFaint).Label("Muted")
			case r.Unread > 0:
				Count(c, r.Unread, false)
			}
		})
	})
}

// miniThought is a status on an avatar's corner: a little cloud with the
// emoji, two dots trailing down to the avatar, as a comic's thought.
// Hovering the cloud itself opens it, over the row, to the whole status
// (never when it clears: that's the owner's business).
func miniThought(c *ui.Context, s Status) {
	p := Of(c)
	// Anchored by its left edge, so it opens to the right, over the row.
	// The box is wide enough for the open cloud (an attached box is capped
	// at its parent's width otherwise); the pointer passes its empty part.
	ui.Box(c.Key("thought")).Attach(ui.AnchorTopRight, ui.AnchorTopLeft).Left(-8).Top(-13).ZIndex(5).
		Width(230).AlignItems(ui.Start).PassThrough().Children(func() {
		// Only the cloud's width moves: the emoji sits in a fixed slot at
		// its left, and the text is always laid out beside it, clipped by
		// the cloud and faded, so nothing inside shifts. Ease-out, no
		// overshoot; closing is quicker than opening.
		const closed = 20
		cloud := ui.Row(c.Key("cloud")).Height(closed).Shrink(0).Radius(RadiusFull).Clip().
			AlignItems(ui.Center).Justify(ui.Start).
			Background(p.Raised).Border(1, p.Border).Shadow(0, 2, 6, -2, p.Shadow).
			Label(s.Emoji + " " + s.Text)
		var txt ui.Element
		cloud.Children(func() {
			ui.Box(c.Key("slot")).Size(closed-2, closed-2).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, s.Emoji).FontSize(11)
			})
			if s.Text != "" {
				txt = ui.Text(c.Key("txt"), s.Text).FontSize(SizeCaption).TextColor(p.Text).SingleLine().
					Shrink(0).MaxWidth(190).Margin(0, S2, 0, 2).
					AccessHidden(true) // the cloud's label reads it already
			}
		})
		// The open width is the text's, measured last frame (it's always
		// there); until it's measured the cloud stays closed.
		hasText := s.Text != ""
		full := float32(closed)
		if hasText {
			if w := txt.Bounds().W; w > 0 {
				full = closed + 2 + w + S2
			}
		}
		open := cloud.Hovered() && full > closed
		target, fade, anim := float32(closed), float32(0), ui.Animation{Duration: DurFast - 30*time.Millisecond, Ease: EaseOut}
		if open {
			target, fade, anim = full, 1, ui.Animation{Duration: DurBase, Ease: EaseOut}
		}
		cloud.Width(cloud.AnimateTo("w", target, anim))
		if hasText {
			txt.Opacity(txt.AnimateTo("o", fade, anim))
		}
		for i, d := range []struct{ x, y, s float32 }{{-5, 14, 4}, {-9, 18, 3}} {
			ui.Box(c.Key(i)).Size(d.s, d.s).Radius(RadiusFull).Background(p.Raised).Border(1, p.Border).
				Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Left(d.x + 6).Top(d.y)
		}
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
	b.Height(h).PaddingX(S2).Gap(S3).Radius(RadiusMd - 2).AlignItems(ui.Center).Justify(ui.Start).
		Background(b.AnimateColor("bg", rowBg(p, s), Color)).AccessSelected(selected).Label(r.Name).
		FocusRingOffset(-2)
	if menu != nil {
		ContextMenu(c, b, menu)
	}
	b.Children(func() { RoomRowContent(c, r, s) })
	return b
}

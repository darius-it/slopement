package kit

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// Presence of a user.
type Presence int

const (
	PresenceNone Presence = iota
	PresenceOnline
	PresenceAway
)

// Trust is the verification state shown on an avatar.
type Trust int

const (
	TrustUnknown Trust = iota // nothing shown
	TrustVerified
	TrustWarning // unverified device / identity changed
)

// AvatarSpec describes an avatar to draw.
type AvatarSpec struct {
	// Name is shown as initials without an image, and names the avatar.
	Name string
	// ID (an MXID or room ID) picks the fallback color.
	ID string
	// Image is the decoded thumbnail, nil while loading or without one.
	Image *ui.Bitmap
	// Size in DIPs: 24, 32, 36, 40, 64, 96.
	Size float32
	// Square rounds it as a space or room icon instead of a circle.
	Square   bool
	Presence Presence
	Trust    Trust
	// Colors resolves user color overrides; nil uses the hash.
	Colors *UserColors
}

// Initials returns up to two initials of name, skipping a leading @, # or !.
func Initials(name string) string {
	name = strings.TrimLeft(name, "@#!+")
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || r == ':' || r == '-' || r == '_' || r == '.'
	})
	var out []rune
	for _, f := range fields {
		r, _ := utf8.DecodeRuneInString(f)
		if r == utf8.RuneError || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			continue // emoji and symbols make poor initials
		}
		out = append(out, unicode.ToUpper(r))
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

func avatarRadius(a AvatarSpec) float32 {
	if a.Square {
		return a.Size * 0.28
	}
	return a.Size / 2
}

// Avatar draws a picture of a user, room or space: its image cropped, or
// its initials on its color, with presence and trust marks.
func Avatar(c *ui.Context, a AvatarSpec) ui.Element {
	p := Of(c)
	if a.Size == 0 {
		a.Size = 36
	}
	col := a.Colors.Color(p, a.ID)
	r := avatarRadius(a)
	box := ui.Box(c).Size(a.Size, a.Size).Shrink(0).Label(a.Name)
	box.Children(func() {
		if a.Image != nil {
			ui.Image(c, a.Image).Size(a.Size, a.Size).Fit(ui.Cover).Radius(r)
		} else {
			ui.Box(c).Size(a.Size, a.Size).Radius(r).Background(col.Alpha(0.16).Over(p.Bg)).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, Initials(a.Name)).FontSize(a.Size * 0.4).FontWeight(WeightSemibold).
					TextColor(col).SingleLine()
			})
		}
		mark := a.Size * 0.3
		if mark < 10 {
			mark = 10
		}
		if a.Presence != PresenceNone {
			dot := p.Success
			if a.Presence == PresenceAway {
				dot = p.Warning
			}
			ui.Box(c).Size(mark, mark).Radius(RadiusFull).Background(dot).Border(2, p.Bg).
				Attach(ui.AnchorBottomRight, ui.AnchorCenter).Top(-mark * 0.35).Left(-mark * 0.35)
		}
		if a.Trust != TrustUnknown {
			icon, color := IconShield, p.Success
			if a.Trust == TrustWarning {
				icon, color = IconShieldX, p.Danger
			}
			s := mark + 4
			ui.Box(c).Size(s, s).Radius(RadiusFull).Background(p.Bg).AlignItems(ui.Center).Justify(ui.Center).
				Attach(ui.AnchorBottomRight, ui.AnchorCenter).Top(-s * 0.3).Left(-s * 0.3).Children(func() {
				ui.Icon(c, icon).Size(mark, mark).TextColor(color)
			})
		}
	})
	return box
}

// AvatarEditor is an avatar the user can change: on hover (or always on
// touch) a camera badge shows; Clicked reports a press to open the picker.
// It shows no presence dot or trust shield: its corner badge is the
// pencil, and a dot there would draw under it.
func AvatarEditor(c *ui.Context, a AvatarSpec) ui.Element {
	p := Of(c)
	a.Presence, a.Trust = PresenceNone, TrustUnknown
	b := ui.ButtonBase(c).Radius(avatarRadius(a)).Label("Change picture")
	b.Children(func() {
		Avatar(c, a)
		show := Touch(c) || b.Hovered() || b.FocusVisible()
		o := float32(0)
		if show {
			o = 1
		}
		o = b.AnimateWith("o", o, DurFast, EaseOut)
		ui.Box(c).Size(a.Size, a.Size).Radius(avatarRadius(a)).Background(ui.RGBA(0, 0, 0, 0.35)).
			Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Opacity(o * boolf(!Touch(c))).PassThrough().
			AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, IconCamera).Size(a.Size*0.3, a.Size*0.3).TextColor(ui.Hex("#FFFFFF"))
		})
		s := a.Size * 0.32
		ui.Box(c).Size(s, s).Radius(RadiusFull).Background(p.Raised).Border(1, p.Border).
			Attach(ui.AnchorBottomRight, ui.AnchorBottomRight).AlignItems(ui.Center).Justify(ui.Center).
			Shadow(0, 1, 3, 0, p.Shadow).PassThrough().Children(func() {
			ui.Icon(c, IconPencil).Size(s*0.5, s*0.5).TextColor(p.Text)
		})
	})
	Pressable(b)
	return b
}

func boolf(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// Package kit is slopement's design kit: the tokens (color, type, space,
// radius, motion) and the components every screen is built from. Screens
// never use raw colors or sizes; they ask the kit.
package kit

import (
	"github.com/egoist/mygo/ui"
)

// Palette is every color of one appearance. The ui.Theme the widgets read
// is derived from it (see Theme), and kit components read the rest here.
type Palette struct {
	Dark bool

	// Surfaces, from the window outwards.
	Bg       ui.Color // the timeline / main content
	BgSubtle ui.Color // rail, room list, sidebars
	BgInset  ui.Color // composer, search field, code
	Raised   ui.Color // popovers, menus, sheets, dialogs
	Hover    ui.Color // row and ghost-button hover (translucent)
	Pressed  ui.Color // row and ghost-button press (translucent)

	Border       ui.Color // hairlines
	BorderStrong ui.Color // inputs, focused edges

	Text      ui.Color
	TextMuted ui.Color // previews, timestamps, secondary labels
	TextFaint ui.Color // placeholders, disabled

	// Accent: light purple. Fill is for primary buttons and the selected
	// indicator; OnAccent is text on it; Subtle is a tinted background for
	// the selected room and mention highlights; Strong is accent-colored
	// text on Bg.
	Accent        ui.Color
	AccentHover   ui.Color
	AccentPressed ui.Color
	OnAccent      ui.Color
	AccentSubtle  ui.Color
	AccentStrong  ui.Color

	Danger      ui.Color
	DangerSubtl ui.Color
	Warning     ui.Color
	Success     ui.Color

	Scrim  ui.Color // behind dialogs and sheets
	Shadow ui.Color // the one soft shadow of floating layers

	// Users are the username colors, lightness-balanced for this Bg.
	Users [8]ui.Color

	// The phone chat (bubbles): a soft lavender wallpaper, incoming
	// bubbles on the raised surface, outgoing ones in a light purple with
	// dark text, as KakaoTalk's and LINE's, in the accent's hue.
	Wallpaper   ui.Color
	BubbleIn    ui.Color
	BubbleOut   ui.Color
	OnBubbleOut ui.Color
	// LinkOnBubble is a link's color in an outgoing bubble: links are
	// colored, not underlined, so it must stand apart from OnBubbleOut.
	LinkOnBubble ui.Color
}

// userHues are the hues of the username palette, in degrees, spread so
// neighbours are told apart and none is the accent's own.
var userHues = [8]float32{25, 60, 100, 150, 195, 235, 330, 355}

func users(l, c float32) (p [8]ui.Color) {
	for i, h := range userHues {
		p[i] = ui.Oklch(l, c, h).SRGB()
	}
	return p
}

// accentHue is the hue of the accent, a lavender purple.
const accentHue = 296

// Light is the light appearance.
var Light = Palette{
	Dark:     false,
	Bg:       ui.Hex("#FCFCFD"),
	BgSubtle: ui.Hex("#F5F5F8"),
	BgInset:  ui.Hex("#F1F0F5"),
	Raised:   ui.Hex("#FFFFFF"),
	Hover:    ui.RGBA(24, 20, 48, 0.045),
	Pressed:  ui.RGBA(24, 20, 48, 0.08),

	Border:       ui.Hex("#E8E7EE"),
	BorderStrong: ui.Hex("#D3D1DC"),

	Text:      ui.Hex("#18171D"),
	TextMuted: ui.Hex("#686676"),
	TextFaint: ui.Hex("#8E8C9B"),

	Accent:        ui.Oklch(0.575, 0.165, accentHue).SRGB(),
	AccentHover:   ui.Oklch(0.535, 0.165, accentHue).SRGB(),
	AccentPressed: ui.Oklch(0.48, 0.16, accentHue).SRGB(),
	OnAccent:      ui.Hex("#FFFFFF"),
	AccentSubtle:  ui.Oklch(0.955, 0.025, accentHue).SRGB(),
	AccentStrong:  ui.Oklch(0.50, 0.17, accentHue).SRGB(),

	Danger:      ui.Oklch(0.55, 0.19, 25).SRGB(),
	DangerSubtl: ui.Oklch(0.96, 0.02, 25).SRGB(),
	Warning:     ui.Oklch(0.62, 0.13, 70).SRGB(),
	Success:     ui.Oklch(0.55, 0.12, 155).SRGB(),

	Scrim:  ui.RGBA(16, 14, 24, 0.28),
	Shadow: ui.RGBA(16, 14, 32, 0.10),

	Users: users(0.52, 0.13),

	Wallpaper:   ui.Oklch(0.955, 0.018, accentHue).SRGB(),
	BubbleIn:    ui.Hex("#FFFFFF"),
	BubbleOut:   ui.Oklch(0.86, 0.075, accentHue).SRGB(),
	OnBubbleOut: ui.Oklch(0.24, 0.05, accentHue).SRGB(),

	LinkOnBubble: ui.Oklch(0.42, 0.19, accentHue).SRGB(),
}

// DarkPalette is the dark appearance.
var DarkPalette = Palette{
	Dark:     true,
	Bg:       ui.Hex("#121116"),
	BgSubtle: ui.Hex("#18171D"),
	BgInset:  ui.Hex("#1E1D24"),
	Raised:   ui.Hex("#1F1E25"),
	Hover:    ui.RGBA(236, 232, 255, 0.05),
	Pressed:  ui.RGBA(236, 232, 255, 0.09),

	Border:       ui.Hex("#2A2931"),
	BorderStrong: ui.Hex("#3A3843"),

	Text:      ui.Hex("#ECEBF1"),
	TextMuted: ui.Hex("#9D9BA9"),
	TextFaint: ui.Hex("#6B6977"),

	Accent:        ui.Oklch(0.78, 0.115, accentHue).SRGB(),
	AccentHover:   ui.Oklch(0.82, 0.10, accentHue).SRGB(),
	AccentPressed: ui.Oklch(0.74, 0.115, accentHue).SRGB(),
	OnAccent:      ui.Hex("#17141F"),
	AccentSubtle:  ui.Oklch(0.27, 0.045, accentHue).SRGB(),
	AccentStrong:  ui.Oklch(0.80, 0.11, accentHue).SRGB(),

	Danger:      ui.Oklch(0.70, 0.15, 25).SRGB(),
	DangerSubtl: ui.Oklch(0.27, 0.05, 25).SRGB(),
	Warning:     ui.Oklch(0.78, 0.12, 75).SRGB(),
	Success:     ui.Oklch(0.74, 0.12, 155).SRGB(),

	Scrim:  ui.RGBA(0, 0, 0, 0.5),
	Shadow: ui.RGBA(0, 0, 0, 0.4),

	Users: users(0.78, 0.11),

	Wallpaper:   ui.Oklch(0.16, 0.014, accentHue).SRGB(),
	BubbleIn:    ui.Oklch(0.245, 0.012, accentHue).SRGB(),
	BubbleOut:   ui.Oklch(0.40, 0.10, accentHue).SRGB(),
	OnBubbleOut: ui.Oklch(0.97, 0.015, accentHue).SRGB(),

	LinkOnBubble: ui.Oklch(0.86, 0.1, accentHue).SRGB(),
}

// Of returns the palette of the theme the context builds with.
func Of(c *ui.Context) *Palette {
	if c.Theme().Dark {
		return &DarkPalette
	}
	return &Light
}

// Space is the 4 DIP spacing scale.
const (
	S1  = 4
	S2  = 8
	S3  = 12
	S4  = 16
	S5  = 20
	S6  = 24
	S8  = 32
	S05 = 2
	S15 = 6
)

// Radii.
const (
	RadiusSm   = 6  // controls
	RadiusMd   = 10 // cards, popovers, menus
	RadiusLg   = 14 // sheets, dialogs
	RadiusFull = 999
)

// Type scale (DIPs). Body is the desktop body; BodyTouch the phone one.
const (
	SizeCaption   = 11
	SizeSmall     = 13
	SizeBody      = 14
	SizeBodyTouch = 16
	SizeLabel     = 15
	SizeTitle     = 18
	SizeHeading   = 22
	SizeDisplay   = 28
)

// Weights: three only.
const (
	WeightRegular  = 400
	WeightMedium   = 500
	WeightSemibold = 600
)

// Layout sizes.
const (
	RailWidth     = 64
	RoomListWidth = 280
	RowHeight     = 52
	RowCompact    = 36
	TouchTarget   = 44
	// CompactWidth is the width under which the phone layout is used.
	CompactWidth = 720
)

// Font is the family list of the kit: the system's UI font, which is the
// cheapest (no embedded font to load) and the most native on each platform.
const Font = ""

var themes [2]*ui.Theme

func init() {
	themes[0] = build(&Light, ui.LightTheme())
	themes[1] = build(&DarkPalette, ui.DarkTheme())
}

func build(p *Palette, base *ui.Theme) *ui.Theme {
	t := *base
	t.Dark = p.Dark
	t.Background = p.Bg
	t.Surface = p.BgInset
	t.SurfaceHover = p.Hover.Over(p.BgInset)
	t.SurfacePressed = p.Pressed.Over(p.BgInset)
	t.Border = p.Border
	t.Text = p.Text
	t.TextMuted = p.TextMuted
	t.Accent = p.Accent
	t.AccentHover = p.AccentHover
	t.AccentPressed = p.AccentPressed
	t.AccentText = p.OnAccent
	t.InactiveAccent = p.Accent
	t.Danger = p.Danger
	t.Warning = p.Warning
	t.Success = p.Success
	t.Selection = p.Accent.Alpha(0.28)
	t.Focus = p.Accent
	t.FocusRingWidth = 2
	t.FocusRingOffset = 1
	if p.Dark {
		t.Inverse, t.InverseText = ui.Hex("#ECEBF1"), ui.Hex("#17161C")
	} else {
		t.Inverse, t.InverseText = ui.Hex("#232129"), ui.Hex("#F5F5F8")
	}
	t.Scrollbar = p.TextFaint.Alpha(0.5)
	t.Radius = RadiusSm
	t.Spacing = 4
	t.FontSize = SizeBody
	t.Font = Font
	return &t
}

// Theme returns the kit's theme for the appearance.
func Theme(dark bool) *ui.Theme {
	if dark {
		return themes[1]
	}
	return themes[0]
}

// Apply themes the window with the kit, following the system's
// appearance: call it first in the root view. Every popup menu MyGo builds
// (text inputs' editing menus, tables', ContextMenu's) is drawn in the
// window in the kit's look, not the system's.
func Apply(c *ui.Context) {
	c.SetTheme(Theme(c.Theme().Dark))
	c.SetMenuPresentation(ui.PopoverMenus)
}

type backdropKey struct {
	base *ui.Theme
	col  ui.Color
}

var backdrops = map[backdropKey]*ui.Theme{}

// Backdrop colors what shows behind the window's content: on a phone, the
// strips the status bar and the home indicator cover, which the root's
// content stays clear of. Make it the color of the screen's own edge (a
// room's wallpaper), or the strips read as a different band. Call it after
// Apply.
func Backdrop(c *ui.Context, col ui.Color) {
	base := c.Theme()
	if base.Background == col {
		return
	}
	k := backdropKey{base, col}
	t, ok := backdrops[k]
	if !ok {
		cp := *base
		cp.Background = col
		t = &cp
		backdrops[k] = t
	}
	c.SetTheme(t)
}

// ApplyTheme is Apply with a theme chosen by the app (a forced light or
// dark).
func ApplyTheme(c *ui.Context, dark bool) {
	c.SetTheme(Theme(dark))
	c.SetMenuPresentation(ui.PopoverMenus)
}

package kit

import "github.com/egoist/mygo/ui"

// AccentPreset is a choice of the accent's tint. The kit has one accent used
// sparingly; the tint is its hue, the lightness and chroma staying the
// palette's, with a small lightness correction where a hue needs one to keep
// its text readable on it (the contrast test checks every preset).
type AccentPreset struct {
	Name string
	Hue  float32
	// Light and Dark shift the fill's lightness in each appearance.
	Light, Dark float32
}

// AccentPresets are the tints to choose from; the first is the default.
var AccentPresets = []AccentPreset{
	{Name: "Lavender", Hue: 296},
	{Name: "Blue", Hue: 255, Light: -0.02},
	{Name: "Teal", Hue: 200, Light: -0.04},
	{Name: "Green", Hue: 150, Light: -0.08},
	{Name: "Rose", Hue: 355},
	{Name: "Amber", Hue: 60, Light: -0.12},
}

var accentNow = 0

// Accent is the preset in use.
func Accent() int { return accentNow }

// SetAccent changes the tint everywhere: the accent colors of both palettes
// and the themes built from them. A no-op when it is the one in use. Call it
// on the UI thread before the frame's views are built.
func SetAccent(i int) {
	if i < 0 || i >= len(AccentPresets) {
		i = 0
	}
	if i == accentNow {
		return
	}
	accentNow = i
	pr := AccentPresets[i]
	h := pr.Hue

	l := &Light
	l.Accent = ui.Oklch(0.575+pr.Light, 0.165, h).SRGB()
	l.AccentHover = ui.Oklch(0.535+pr.Light, 0.165, h).SRGB()
	l.AccentPressed = ui.Oklch(0.48+pr.Light, 0.16, h).SRGB()
	l.AccentSubtle = ui.Oklch(0.955, 0.025, h).SRGB()
	l.AccentStrong = ui.Oklch(0.50+pr.Light, 0.17, h).SRGB()
	l.Wallpaper = ui.Oklch(0.955, 0.018, h).SRGB()
	l.BubbleOut = ui.Oklch(0.86, 0.075, h).SRGB()
	l.OnBubbleOut = ui.Oklch(0.24, 0.05, h).SRGB()
	l.LinkOnBubble = ui.Oklch(0.42+pr.Light, 0.19, h).SRGB()

	d := &DarkPalette
	d.Accent = ui.Oklch(0.78+pr.Dark, 0.115, h).SRGB()
	d.AccentHover = ui.Oklch(0.82+pr.Dark, 0.10, h).SRGB()
	d.AccentPressed = ui.Oklch(0.74+pr.Dark, 0.115, h).SRGB()
	d.AccentSubtle = ui.Oklch(0.27, 0.045, h).SRGB()
	d.AccentStrong = ui.Oklch(0.80+pr.Dark, 0.11, h).SRGB()
	d.Wallpaper = ui.Oklch(0.16, 0.014, h).SRGB()
	d.BubbleIn = ui.Oklch(0.245, 0.012, h).SRGB()
	d.BubbleOut = ui.Oklch(0.40, 0.10, h).SRGB()
	d.OnBubbleOut = ui.Oklch(0.97, 0.015, h).SRGB()
	d.LinkOnBubble = ui.Oklch(0.86, 0.1, h).SRGB()

	themes[0] = build(&Light, ui.LightTheme())
	themes[1] = build(&DarkPalette, ui.DarkTheme())
}

// Dense makes the timeline closer: less room between messages and between
// their groups. Set from the app's setting each frame.
var Dense bool

// groupGap is the room above the first message of a group, and rowPad the
// room above and below each message, in the density in use.
func groupGap() float32 {
	if Dense {
		return S15
	}
	return MessageGroupGap
}

func rowPad() float32 {
	if Dense {
		return 1
	}
	return S05
}

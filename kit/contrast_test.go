package kit

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func lin(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func luminance(c ui.Color) float64 {
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast returns the WCAG contrast ratio of a over b.
func contrast(a, b ui.Color) float64 {
	if a.A < 255 {
		a = a.Over(b)
	}
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestContrast(t *testing.T) {
	for _, p := range []*Palette{&Light, &DarkPalette} {
		name := "light"
		if p.Dark {
			name = "dark"
		}
		check := func(what string, fg, bg ui.Color, min float64) {
			t.Helper()
			if r := contrast(fg, bg); r < min {
				t.Errorf("%s: %s = %.2f, want >= %.1f (fg %v bg %v)", name, what, r, min, fg, bg)
			}
		}
		for _, bg := range []struct {
			n string
			c ui.Color
		}{{"Bg", p.Bg}, {"BgSubtle", p.BgSubtle}, {"Raised", p.Raised}} {
			check("Text on "+bg.n, p.Text, bg.c, 7)
			check("TextMuted on "+bg.n, p.TextMuted, bg.c, 4.5)
			check("AccentStrong on "+bg.n, p.AccentStrong, bg.c, 4.5)
			check("Accent (UI) on "+bg.n, p.Accent, bg.c, 3)
			for i, u := range p.Users {
				check("User color "+string(rune('0'+i))+" on "+bg.n, u, bg.c, 4.5)
			}
		}
		check("OnAccent on Accent", p.OnAccent, p.Accent, 4.5)
		check("OnAccent on AccentHover", p.OnAccent, p.AccentHover, 4.5)
		check("AccentStrong on AccentSubtle", p.AccentStrong, p.AccentSubtle, 4.5)
		check("Text on AccentSubtle", p.Text, p.AccentSubtle, 7)
		check("Danger on Bg", p.Danger, p.Bg, 4.5)
		check("TextFaint on BgInset (decorative only)", p.TextFaint, p.BgInset, 2.5)
		check("Text on BgInset", p.Text, p.BgInset, 7)
		// The phone chat.
		check("Text on BubbleIn", p.Text, p.BubbleIn, 7)
		check("OnBubbleOut on BubbleOut", p.OnBubbleOut, p.BubbleOut, 7)
		check("LinkOnBubble on BubbleOut", p.LinkOnBubble, p.BubbleOut, 4.5)
		check("AccentStrong links on Bg", p.AccentStrong, p.Bg, 4.5)
		check("TextMuted on Wallpaper (times)", p.TextMuted, p.Wallpaper, 4.5)
		check("AccentStrong on Wallpaper (read)", p.AccentStrong, p.Wallpaper, 4.5)
		check("BubbleIn stands off Wallpaper", p.BubbleIn, p.Wallpaper, 1.05)
		for i, u := range p.Users {
			check("User color "+string(rune('0'+i))+" on BubbleIn", u, p.BubbleIn, 4.5)
		}
	}
}

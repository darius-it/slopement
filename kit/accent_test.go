package kit

import "testing"

// Every accent tint keeps what must stay readable readable, in both
// appearances: text on the accent fill, the accent as text, links on bubbles.
func TestAccentPresetsKeepTheirContrast(t *testing.T) {
	defer SetAccent(0)
	for i, pr := range AccentPresets {
		SetAccent(i)
		for _, p := range []*Palette{&Light, &DarkPalette} {
			name := pr.Name + "/light"
			if p.Dark {
				name = pr.Name + "/dark"
			}
			check := func(what string, ratio, min float64) {
				t.Helper()
				if ratio < min {
					t.Errorf("%s: %s = %.2f, want >= %.1f", name, what, ratio, min)
				}
			}
			check("OnAccent on Accent", contrast(p.OnAccent, p.Accent), 4.5)
			check("OnAccent on AccentHover", contrast(p.OnAccent, p.AccentHover), 4.5)
			check("OnAccent on AccentPressed", contrast(p.OnAccent, p.AccentPressed), 4.5)
			check("AccentStrong on AccentSubtle", contrast(p.AccentStrong, p.AccentSubtle), 4.5)
			check("Text on AccentSubtle", contrast(p.Text, p.AccentSubtle), 7)
			check("OnBubbleOut on BubbleOut", contrast(p.OnBubbleOut, p.BubbleOut), 7)
			check("LinkOnBubble on BubbleOut", contrast(p.LinkOnBubble, p.BubbleOut), 4.5)
			check("Text on BubbleIn", contrast(p.Text, p.BubbleIn), 7)
			check("AccentStrong on Bg", contrast(p.AccentStrong, p.Bg), 4.5)
			check("AccentStrong on Raised", contrast(p.AccentStrong, p.Raised), 4.5)
			check("AccentStrong on Wallpaper", contrast(p.AccentStrong, p.Wallpaper), 4.5)
			check("TextMuted on Wallpaper", contrast(p.TextMuted, p.Wallpaper), 4.5)
			check("Accent as UI on Bg", contrast(p.Accent, p.Bg), 3)
			check("BubbleIn stands off Wallpaper", contrast(p.BubbleIn, p.Wallpaper), 1.05)
		}
	}
}

// SetAccent changes the themes the views are built with, and going back to
// the first restores the original.
func TestSetAccentChangesTheTheme(t *testing.T) {
	defer SetAccent(0)
	before := Theme(false).Accent
	SetAccent(1)
	if Theme(false).Accent == before || Theme(true).Accent == DarkPalette.OnAccent {
		t.Fatal("the theme did not take the new tint")
	}
	if Theme(false).Accent != Light.Accent {
		t.Fatal("the theme and the palette disagree")
	}
	SetAccent(0)
	if Theme(false).Accent != before {
		t.Fatal("the default tint did not come back")
	}
	SetAccent(99) // out of range: the default
	if Accent() != 0 {
		t.Fatalf("a bad preset gave %d", Accent())
	}
}

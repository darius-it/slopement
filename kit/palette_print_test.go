package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func hex(c ui.Color) string {
	const d = "0123456789ABCDEF"
	return "#" + string([]byte{d[c.R>>4], d[c.R&15], d[c.G>>4], d[c.G&15], d[c.B>>4], d[c.B&15]})
}

// TestPrintPalette logs the derived hex values, for docs/design-language.md.
func TestPrintPalette(t *testing.T) {
	for _, p := range []*Palette{&Light, &DarkPalette} {
		t.Logf("dark=%v accent=%s hover=%s pressed=%s subtle=%s strong=%s danger=%s warning=%s success=%s",
			p.Dark, hex(p.Accent), hex(p.AccentHover), hex(p.AccentPressed), hex(p.AccentSubtle), hex(p.AccentStrong), hex(p.Danger), hex(p.Warning), hex(p.Success))
		s := ""
		for _, u := range p.Users {
			s += hex(u) + " "
		}
		t.Logf("users: %s", s)
	}
}

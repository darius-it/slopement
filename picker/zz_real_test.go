package picker

import (
	"os"
	"testing"
)

// DISCORD_EXPORT=<file> go test -run RealDiscordExport ./picker/ checks a real
// export: every entry should come out as a GIF.
func TestRealDiscordExport(t *testing.T) {
	path := os.Getenv("DISCORD_EXPORT")
	if path == "" {
		t.Skip("DISCORD_EXPORT not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseDiscordFavorites(data)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, it := range items {
		if it.Full.URL != "" {
			n++
		}
	}
	t.Logf("%d GIFs from the export", len(items))
	for i, it := range items {
		if i < 8 || i%100 == 0 {
			t.Logf("%q %s", it.Title, it.Full.URL)
		}
	}
	if n != len(items) {
		t.Fatalf("%d without a URL", len(items)-n)
	}
}

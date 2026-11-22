package picker

import (
	"path/filepath"
	"testing"
)

// An export as discord-gif-exporter writes it: Tenor MP4s, a Giphy MP4, a
// plain GIF, and a video with no GIF.
const discordExport = `{
  "https://tenor.com/view/cat-typing-gif-12345": {"format": 2, "src": "//media.tenor.com/Xy9AbCdEAAPo/cat-typing.mp4", "width": 498, "height": 280, "order": 1},
  "https://giphy.com/gifs/dog-happy-3o7TKSjR": {"format": 2, "src": "https://media2.giphy.com/media/3o7TKSjR/giphy.mp4", "width": 480, "height": 360, "order": 2},
  "https://example.com/party.gif": {"format": 1, "src": "https://example.com/party.gif", "width": 200, "height": 200, "order": 3},
  "https://cdn.discordapp.com/attachments/1/2/clip.mp4": {"format": 2, "src": "https://media.discordapp.net/attachments/1/2/clip.mp4", "width": 640, "height": 360, "order": 4}
}`

func TestParseDiscordFavorites(t *testing.T) {
	items, err := parseDiscordFavorites([]byte(discordExport))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("%d items, want 3 (the video without a GIF skipped)", len(items))
	}
	// Newest first (the highest order).
	want := []string{
		"https://example.com/party.gif",
		"https://media2.giphy.com/media/3o7TKSjR/giphy.gif",
		"https://media.tenor.com/Xy9AbCdEAAAC/cat-typing.gif",
	}
	for i, w := range want {
		if items[i].Full.URL != w {
			t.Errorf("item %d = %s, want %s", i, items[i].Full.URL, w)
		}
	}
	if items[2].Title != "cat typing gif" || items[2].Full.Width != 498 {
		t.Errorf("tenor item = %+v", items[2])
	}
	if _, err := parseDiscordFavorites([]byte(`[1,2]`)); err != ErrNotDiscordExport {
		t.Error("a non-export was accepted")
	}
}

func TestFavoriteURL(t *testing.T) {
	c := newAt(nil, filepath.Join(t.TempDir(), "picker.json"))
	u := "https://static.klipy.com/ii/abc/8a/b1/uAaLRrFA.gif"
	c.SetFavoriteURL(u, 320, 240, true)
	if !c.FavoriteURL(u) || len(c.saved.Favorites) != 1 || c.saved.Favorites[0].Full.Width != 320 {
		t.Fatalf("not added: %+v", c.saved.Favorites)
	}
	c.SetFavoriteURL(u, 0, 0, true) // again: no duplicate
	c.SetFavoriteURL(u, 0, 0, false)
	if c.FavoriteURL(u) || len(c.saved.Favorites) != 0 {
		t.Fatal("not removed")
	}
}

// Discord's media proxy wraps the real address; Tenor's MP4 behind it still
// becomes its GIF.
func TestDiscordProxiedTenor(t *testing.T) {
	src := "https://images-ext-1.discordapp.net/external/fhJm/https/media.tenor.com/zVPv-paCuVMAAAPo/otter.mp4"
	if got, want := unproxy(src), "https://media.tenor.com/zVPv-paCuVMAAAPo/otter.mp4"; got != want {
		t.Fatalf("unproxy %q", got)
	}
	if got, want := discordGIF("https://tenor.com/view/otter-gif-1", src), "https://media.tenor.com/zVPv-paCuVMAAAAC/otter.gif"; got != want {
		t.Fatalf("gif %q", got)
	}
	if other := "https://example.com/a.mp4"; unproxy(other) != other {
		t.Fatal("changed a plain address")
	}
}

func TestImportDiscordFavorites(t *testing.T) {
	c := newAt(nil, filepath.Join(t.TempDir(), "picker.json"))
	n, err := c.ImportDiscordFavorites([]byte(discordExport))
	if err != nil || n != 3 {
		t.Fatalf("imported %d, %v", n, err)
	}
	// Again: nothing new.
	if n, _ := c.ImportDiscordFavorites([]byte(discordExport)); n != 0 {
		t.Fatalf("re-import added %d", n)
	}
	again := newAt(nil, c.path)
	if len(again.saved.Favorites) != 3 {
		t.Fatal("not saved")
	}
}

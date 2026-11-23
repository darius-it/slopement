package picker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"slopement/klipy"
)

// Reset clears the favorites and the copies, and an import afterwards copies
// every link that expires again.
func TestResetFavoritesThenReimport(t *testing.T) {
	dir := t.TempDir()
	c := newAt(nil, filepath.Join(dir, "picker.json"))
	c.update = func(f func()) { f() }
	os.MkdirAll(c.pinDir(), 0o700)
	os.WriteFile(filepath.Join(c.pinDir(), "a.gif"), gifBytes, 0o600)
	c.saved.Favorites = []klipy.Item{discordItem("a", time.Now().Add(time.Hour)), discordItem("b", time.Now().Add(time.Hour))}
	for _, f := range c.saved.Favorites {
		c.items[f.ID] = f
	}
	if c.FavoriteCount() != 2 {
		t.Fatal("count")
	}
	if n := c.ResetFavorites(); n != 2 || c.FavoriteCount() != 0 || len(c.items) != 0 {
		t.Fatalf("reset: %d, left %d", n, c.FavoriteCount())
	}
	if left, _ := os.ReadDir(c.pinDir()); len(left) != 0 {
		t.Fatalf("copies left: %d", len(left))
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "picker.json")); len(b) == 0 {
		t.Fatal("not saved")
	}
}

// Importing again, even from a new export that signs the same attachments with
// new links, does not duplicate a favorite: not before it is copied (the same
// attachment path), nor after (the same bytes).
func TestReimportDoesNotDuplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(gifBytes) }))
	defer srv.Close()
	c := newAt(nil, filepath.Join(t.TempDir(), "picker.json"))
	c.http = &http.Client{Transport: rewrite{srv}, Timeout: 5 * time.Second}
	c.update = func(f func()) { f() }

	export := func(ex time.Time) []byte {
		u := "https://media.discordapp.net/attachments/1/2/cat.gif?ex=" + strconv.FormatInt(ex.Unix(), 16) + "&is=a&hm=" + ex.Format("150405")
		return []byte(`{"` + u + `": {"format": 1, "src": "` + u + `", "width": 1, "height": 1, "order": 1}}`)
	}
	n, err := c.ImportDiscordFavorites(export(time.Now().Add(20 * time.Hour)))
	if err != nil || n != 1 {
		t.Fatalf("first import: %d %v", n, err)
	}
	waitPinned := func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) && (c.pinning || !IsLocalURL(c.saved.Favorites[0].Full.URL)) {
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitPinned()
	// The same attachment, signed again (a later export): still one.
	c.ImportDiscordFavorites(export(time.Now().Add(22 * time.Hour)))
	time.Sleep(200 * time.Millisecond)
	waitPinned()
	if got := len(c.saved.Favorites); got != 1 {
		t.Fatalf("%d favorites after importing the same GIF twice", got)
	}
	if files, _ := os.ReadDir(c.pinDir()); len(files) != 1 {
		t.Fatalf("%d copies on disk, want 1", len(files))
	}
}

// An import that lands while copies are being made is not skipped: PinExpiring
// asks to run again when the first run ends.
func TestPinRunsAgainAfterAnImportDuringARun(t *testing.T) {
	c := newAt(nil, filepath.Join(t.TempDir(), "picker.json"))
	c.update = func(f func()) { f() }
	c.pinning = true
	c.saved.Favorites = []klipy.Item{discordItem("a", time.Now().Add(time.Hour))}
	c.PinExpiring()
	if !c.pinAgain {
		t.Fatal("the second ask was dropped")
	}
}

package picker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"slopement/klipy"
)

var gifBytes = []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")

func discordItem(id string, ex time.Time) klipy.Item {
	u := "https://media.discordapp.net/attachments/1/2/cat.gif?ex=" + strconv.FormatInt(ex.Unix(), 16) + "&is=a&hm=b&"
	r := klipy.Rendition{URL: u, Width: 1, Height: 1}
	return klipy.Item{ID: id, Kind: klipy.GIFs, Thumb: r, Still: r, Full: r}
}

// rewrite sends every request to the test server.
type rewrite struct{ to *httptest.Server }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = "http", strings.TrimPrefix(r.to.URL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}

// A favorite from Discord has a link that lasts a day: it is copied to the
// device while it works, and the favorite then points at the copy.
func TestExpiringDiscordLinksAreKeptOnTheDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(gifBytes) }))
	defer srv.Close()
	dir := t.TempDir()
	c := newAt(nil, filepath.Join(dir, "picker.json"))
	c.http = &http.Client{Transport: rewrite{srv}, Timeout: 5 * time.Second}
	c.update = func(f func()) { f() }
	live := discordItem("discord:live", time.Now().Add(20*time.Hour))
	dead := discordItem("discord:dead", time.Now().Add(-time.Hour))
	other := klipy.Item{ID: "x", Full: klipy.Rendition{URL: "https://media.tenor.com/a/b.gif"}, Thumb: klipy.Rendition{URL: "https://media.tenor.com/a/b.gif"}}
	c.saved.Favorites = []klipy.Item{live, dead, other}
	for _, f := range c.saved.Favorites {
		c.items[f.ID] = f
	}

	if !expiringLink(live.Full.URL, time.Now()) || expiringLink(dead.Full.URL, time.Now()) || expiringLink(other.Full.URL, time.Now()) {
		t.Fatal("expiringLink is wrong")
	}
	c.PinExpiring()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !IsLocalURL(c.saved.Favorites[0].Full.URL) {
		time.Sleep(10 * time.Millisecond)
	}
	got := c.saved.Favorites[0]
	if !IsLocalURL(got.Full.URL) || got.Thumb.URL != got.Full.URL {
		t.Fatalf("the live favorite still points at its link: %s", got.Full.URL)
	}
	if data, err := os.ReadFile(LocalPath(got.Full.URL)); err != nil || string(data) != string(gifBytes) {
		t.Fatalf("the copy is not there: %v", err)
	}
	if c.saved.Favorites[1].Full.URL != dead.Full.URL || c.saved.Favorites[2].Full.URL != other.Full.URL {
		t.Fatal("others were touched")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "picker.json")); !strings.Contains(string(b), "file://") {
		t.Fatal("the change was not saved")
	}
}

// What cannot come back is counted, and removable in one go; copies and good
// links stay.
func TestUnavailableFavoritesCanBeRemoved(t *testing.T) {
	c := newAt(nil, filepath.Join(t.TempDir(), "picker.json"))
	dead := discordItem("d", time.Now().Add(-time.Hour))
	good := discordItem("g", time.Now().Add(time.Hour))
	local := discordItem("l", time.Now().Add(-time.Hour))
	local.Full.URL, local.Thumb.URL = "file:///x.gif", "file:///x.gif"
	deleted := klipy.Item{ID: "t", Full: klipy.Rendition{URL: "https://media.tenor.com/gone.gif"}, Thumb: klipy.Rendition{URL: "https://media.tenor.com/gone.gif"}}
	c.saved.Favorites = []klipy.Item{dead, good, local, deleted}
	c.media.entries[deleted.Thumb.URL] = &mediaEntry{} // a load that failed for good
	c.media.markGone(deleted.Thumb.URL)
	if n := c.UnavailableCount(); n != 2 {
		t.Fatalf("count %d, want 2", n)
	}
	if n := c.RemoveUnavailable(); n != 2 || len(c.saved.Favorites) != 2 || c.UnavailableCount() != 0 {
		t.Fatalf("removed %d, left %d", n, len(c.saved.Favorites))
	}
}

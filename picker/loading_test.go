package picker

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"slopement/kit"
	"slopement/klipy"
)

// Building a page of tiles starts no download: a tile loads when it asks
// (Want), once it is near the view.
func TestTilesLoadWhenWanted(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(tinyGIF())
	}))
	defer srv.Close()
	c := newAt(nil, filepath.Join(t.TempDir(), "p.json"))
	q := &queue{}
	c.SetUpdate(q.post)
	var items []klipy.Item
	for i := 0; i < 20; i++ {
		u := srv.URL + "/" + strconv.Itoa(i) + ".gif"
		items = append(items, klipy.Item{ID: "x" + strconv.Itoa(i), Thumb: klipy.Rendition{URL: u, Width: 4, Height: 4}})
	}
	tiles := c.tiles(items)
	time.Sleep(50 * time.Millisecond)
	if hits.Load() != 0 {
		t.Fatalf("%d downloads before any tile was wanted", hits.Load())
	}
	tiles[3].Want()
	q.settle(t, func() bool { _, ok := c.media.get(items[3].Thumb.URL); return ok })
	if hits.Load() != 1 {
		t.Fatalf("%d downloads for one wanted tile", hits.Load())
	}
	c.gifs.Results = c.tiles(items)
	if c.gifs.Results[3].Player == nil || c.gifs.Results[4].Player != nil {
		t.Fatal("the loaded tile has its animation, the others don't")
	}
}

// No more than maxParallel downloads run at once.
func TestDownloadsAreLimited(t *testing.T) {
	var now, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := now.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		now.Add(-1)
		w.Write(tinyGIF())
	}))
	defer srv.Close()
	c := newAt(nil, filepath.Join(t.TempDir(), "p.json"))
	q := &queue{}
	c.SetUpdate(q.post)
	const total = 16
	for i := 0; i < total; i++ {
		c.mediaOf(srv.URL + "/" + strconv.Itoa(i) + ".gif")
	}
	q.settle(t, func() bool { return c.media.Len() == total })
	if peak.Load() > maxParallel {
		t.Fatalf("%d downloads at once, limit %d", peak.Load(), maxParallel)
	}
}

// A tile that scrolled out of the view while it waited its turn is not
// downloaded; it starts over if it comes back.
func TestDroppedTilesAreNotDownloaded(t *testing.T) {
	release := make(chan struct{})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		w.Write(tinyGIF())
	}))
	defer srv.Close()
	c := newAt(nil, filepath.Join(t.TempDir(), "p.json"))
	q := &queue{}
	c.SetUpdate(q.post)
	var items []klipy.Item
	for i := 0; i < maxParallel+6; i++ {
		items = append(items, klipy.Item{ID: "x" + strconv.Itoa(i), Thumb: klipy.Rendition{URL: srv.URL + "/" + strconv.Itoa(i) + ".gif", Width: 4, Height: 4}})
	}
	tiles := c.tiles(items)
	for _, tl := range tiles {
		tl.Want()
	}
	for hits.Load() < maxParallel { // maxParallel are downloading, the rest wait their turn
		time.Sleep(5 * time.Millisecond)
	}
	for _, tl := range tiles {
		tl.Drop() // all scrolled out of the view
	}
	close(release)
	q.settle(t, func() bool { return c.media.Len() == maxParallel })
	time.Sleep(100 * time.Millisecond)
	q.drain()
	if hits.Load() != maxParallel {
		t.Fatalf("%d downloads, want only the %d under way", hits.Load(), maxParallel)
	}
	var left klipy.Item
	for _, it := range items {
		if _, ok := c.media.get(it.Thumb.URL); !ok {
			left = it
			break
		}
	}
	c.tile(left).Want() // back in view: it loads now
	q.settle(t, func() bool { _, ok := c.media.get(left.Thumb.URL); return ok })
}

// A failed load tries again after a wait (and a tap tries at once), and a
// tile says it failed meanwhile.
func TestFailedLoadRetries(t *testing.T) {
	var up atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up.Load() {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Write(tinyGIF())
	}))
	defer srv.Close()
	c := newAt(nil, filepath.Join(t.TempDir(), "p.json"))
	q := &queue{}
	c.SetUpdate(q.post)
	clock := time.Now()
	c.media.now = func() time.Time { return clock }
	url := srv.URL + "/a.gif"
	it := klipy.Item{ID: "a", Thumb: klipy.Rendition{URL: url, Width: 4, Height: 4}}
	tile := c.tiles([]klipy.Item{it})[0]
	c.gifs.Results = c.tiles([]klipy.Item{it})
	tile.Want()
	q.settle(t, func() bool { return c.media.failed(url) })
	if !c.tiles([]klipy.Item{it})[0].Failed {
		t.Fatal("the tile does not say it failed")
	}
	// Too soon: asking again does nothing.
	if c.media.start(url) {
		t.Fatal("started again before its wait")
	}
	// Past the wait it starts by itself, and loads once the server is up.
	up.Store(true)
	clock = clock.Add(3 * time.Second)
	tile.Want()
	q.settle(t, func() bool { _, ok := c.media.get(url); return ok })
	if c.tiles([]klipy.Item{it})[0].Failed {
		t.Fatal("still failed after loading")
	}

	// Out of automatic tries, a tap retries.
	up.Store(false)
	url2 := srv.URL + "/b.gif"
	it2 := klipy.Item{ID: "b", Thumb: klipy.Rendition{URL: url2, Width: 4, Height: 4}}
	c.gifs.Results = c.tiles([]klipy.Item{it2})
	for i := 0; i <= autoRetries; i++ {
		clock = clock.Add(time.Minute)
		c.mediaOf(url2)
		q.settle(t, func() bool { return c.media.failed(url2) })
	}
	clock = clock.Add(time.Minute)
	if c.media.start(url2) {
		t.Fatal("kept retrying by itself")
	}
	up.Store(true)
	c.Retry(&c.gifs.Results[0])
	q.settle(t, func() bool { _, ok := c.media.get(url2); return ok })
}

// Sending a KLIPY item reports the share with the search that found it;
// an item that isn't KLIPY's (no slug) reports nothing.
func TestSendRegistersShare(t *testing.T) {
	got := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 256)
		n, _ := r.Body.Read(b)
		got <- r.Method + " " + r.URL.Path + " " + string(b[:n])
	}))
	defer srv.Close()
	c := newAt(klipy.New(klipy.Config{Key: "k", BaseURL: srv.URL, CustomerID: "cust"}), filepath.Join(t.TempDir(), "p.json"))
	c.items["a"] = klipy.Item{ID: "a", Slug: "cat-1", Kind: klipy.GIFs, Full: klipy.Rendition{URL: "u"}}
	c.items["b"] = klipy.Item{ID: "b", Kind: klipy.GIFs, Full: klipy.Rendition{URL: "v"}}
	st := &kit.PickerState{Query: "cat"}
	c.Handle(st, kit.PickerResult{Item: &kit.PickerTile{ID: "b"}})
	st.Query = "cat" // choosing an item closes the search
	c.Handle(st, kit.PickerResult{Item: &kit.PickerTile{ID: "a"}})
	select {
	case s := <-got:
		if want := `POST /api/v1/k/gifs/share/cat-1 {"customer_id":"cust","q":"cat"}`; s != want {
			t.Fatalf("got %q, want %q", s, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no share registered")
	}
	select {
	case s := <-got:
		t.Fatalf("a second report: %q", s)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestThumbURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://media.tenor.com/Bh4Mf9OUMh8AAAAC/bro-just-typing.gif": "https://media.tenor.com/Bh4Mf9OUMh8AAAAM/bro-just-typing.gif",
		"https://media.tenor.com/Bh4Mf9OUMh8AAAAM/x.gif":               "https://media.tenor.com/Bh4Mf9OUMh8AAAAM/x.gif",
		"https://example.com/a.gif":                                    "https://example.com/a.gif",
	} {
		if got := thumbURL(in); got != want {
			t.Errorf("thumbURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// Favorites imported before thumbnails (thumb = the full GIF) get the small
// one when loaded.
func TestOldFavoritesGetThumbnails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	full := "https://media.tenor.com/Bh4Mf9OUMh8AAAAC/x.gif"
	c := newAt(nil, path)
	c.saved.Favorites = []klipy.Item{{ID: "d", Thumb: klipy.Rendition{URL: full}, Full: klipy.Rendition{URL: full}}}
	c.save()
	again := newAt(nil, path)
	f := again.saved.Favorites[0]
	if f.Thumb.URL == full || f.Full.URL != full {
		t.Fatalf("thumb %q full %q", f.Thumb.URL, f.Full.URL)
	}
}

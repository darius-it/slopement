package picker

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/klipy"
)

// tinyGIF is a 2-frame 4×4 GIF.
func tinyGIF() []byte {
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
		img.SetColorIndex(i, i, 1)
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	gif.EncodeAll(&b, g)
	return b.Bytes()
}

// fakeKlipy serves KLIPY's shapes, its media from itself.
func fakeKlipy(t *testing.T) *httptest.Server {
	var srv *httptest.Server
	data := tinyGIF()
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".gif"):
			w.Write(data)
		case strings.HasSuffix(r.URL.Path, "/gifs/categories"):
			w.Write([]byte(`{"result":true,"data":{"categories":[{"category":"lol","query":"lol","preview_url":"` + srv.URL + `/cat.gif"}]}}`))
		case strings.Contains(r.URL.Path, "/gifs/"):
			w.Write([]byte(`{"result":true,"data":{"data":[{"id":7,"slug":"a","title":"A cat","type":"gif","file":{
				"xs":{"gif":{"url":"` + srv.URL + `/xs.gif","width":4,"height":4},"jpg":{"url":"` + srv.URL + `/xs.jpg","width":4,"height":4}},
				"md":{"gif":{"url":"` + srv.URL + `/md.gif","width":4,"height":4}}}}],"current_page":1,"has_next":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// queue runs posted work on the test's goroutine, as the UI thread.
type queue struct {
	mu  sync.Mutex
	fns []func()
}

func (q *queue) post(fn func()) { q.mu.Lock(); q.fns = append(q.fns, fn); q.mu.Unlock() }
func (q *queue) drain() int {
	q.mu.Lock()
	fns := q.fns
	q.fns = nil
	q.mu.Unlock()
	for _, f := range fns {
		f()
	}
	return len(fns)
}

// settle drains until nothing more arrives for a while.
func (q *queue) settle(t *testing.T, until func() bool) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		q.drain()
		if until() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestGIFTabFlow(t *testing.T) {
	srv := fakeKlipy(t)
	q := &queue{}
	c := newAt(klipy.New(klipy.Config{Key: "k", BaseURL: srv.URL}), filepath.Join(t.TempDir(), "picker.json"))
	c.SetUpdate(q.post)

	st := &kit.PickerState{Tab: kit.TabGIFs}
	st.Open = true
	// The front page: favorites, trending, then the categories, whose
	// previews animate once loaded.
	q.settle(t, func() bool {
		m := c.Model(st)
		return len(m.Front) == 3 && m.Front[2].Player != nil
	})
	m := c.Model(st)
	if m.Front[0].Kind != kit.TileFavorites || m.Front[1].Kind != kit.TileTrending || m.Front[2].Label != "lol" {
		t.Fatalf("front = %+v", m.Front)
	}
	// Open the category: its GIFs load, then their animations.
	st.Browse = "lol"
	q.settle(t, func() bool {
		m := c.Model(st)
		return len(m.Results) == 1 && m.Results[0].Player != nil
	})
	// Star it: it leads the favorites, which persist.
	tile := c.Model(st).Results[0]
	c.Handle(st, kit.PickerResult{ToggleFav: &tile})
	if !c.Model(st).Results[0].Favorite {
		t.Fatal("not starred")
	}
	st.Browse = "favorites"
	if m := c.Model(st); len(m.Results) != 1 || m.Results[0].ID != "7" {
		t.Fatalf("favorites = %+v", m.Results)
	}
	again := newAt(nil, c.path)
	if len(again.saved.Favorites) != 1 {
		t.Fatal("favorites did not persist")
	}
	// Choosing sends the full GIF and closes the browse.
	ch := c.Handle(st, kit.PickerResult{Item: &tile})
	if ch.Item == nil || ch.Item.Full.URL != srv.URL+"/md.gif" || st.Browse != "" {
		t.Fatalf("chosen = %+v, browse %q", ch.Item, st.Browse)
	}
}

func TestEmojiRecents(t *testing.T) {
	c := newAt(nil, filepath.Join(t.TempDir(), "picker.json"))
	st := &kit.PickerState{Tab: kit.TabEmoji}
	st.Open = true
	c.Handle(st, kit.PickerResult{Emoji: "🔥"})
	c.Handle(st, kit.PickerResult{Emoji: "💜"})
	c.Handle(st, kit.PickerResult{Emoji: "🔥"})
	if got := c.Model(st).RecentEmoji; len(got) != 2 || got[0] != "🔥" || got[1] != "💜" {
		t.Fatalf("recent = %v", got)
	}
}

func TestNoKeyNotice(t *testing.T) {
	t.Setenv("KLIPY_API_KEY", "")
	c := New(klipy.New(klipy.Config{}))
	st := &kit.PickerState{Tab: kit.TabStickers}
	st.Open = true
	if m := c.Model(st); !strings.Contains(m.Notice, "Settings → GIFs") {
		t.Fatalf("notice = %q", m.Notice)
	}
}

func TestMediaCacheBound(t *testing.T) {
	m := newMediaCache(2)
	for _, u := range []string{"a", "b", "c"} {
		d, err := ui.NewGIFDecoder(tinyGIF())
		if err != nil {
			t.Fatal(err)
		}
		p, _ := ui.NewVideoPlayer(d)
		m.start(u)
		m.put(u, p)
	}
	if m.Len() != 2 {
		t.Fatalf("len = %d", m.Len())
	}
	if _, ok := m.get("a"); ok {
		t.Fatal("oldest kept")
	}
}

// A body over the limit must fail, never arrive cut short: a truncated GIF
// cannot be decoded. Real full-size GIFs are several MB, so the limit has
// room for them.
func TestDownloadLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/small":
			w.Write(tinyGIF())
		case "/declared":
			w.Header().Set("Content-Length", strconv.Itoa(maxDownload+1))
		case "/streamed":
			w.(http.Flusher).Flush() // no Content-Length: read until the limit
			chunk := make([]byte, 1<<20)
			for written := 0; written <= maxDownload; written += len(chunk) {
				w.Write(chunk)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Controller{http: srv.Client()}
	if b, err := c.download(srv.URL + "/small"); err != nil || len(b) == 0 {
		t.Fatalf("small: %v", err)
	}
	if _, err := c.download(srv.URL + "/declared"); err == nil {
		t.Fatal("accepted a declared over-limit body")
	}
	if _, err := c.download(srv.URL + "/streamed"); err == nil {
		t.Fatal("truncated an over-limit body instead of failing")
	}
}

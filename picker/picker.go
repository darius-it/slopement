// Package picker drives the composer's picker (kit.Picker): it fetches
// GIFs, stickers and categories from KLIPY, searches as the user types,
// keeps favorites and recents on disk, and loads the tiles' small
// animations off the UI thread, keeping a bounded number of them.
//
// Every model change happens on the UI thread, through the update
// function the app gives it (mygo's Window.Update).
package picker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/klipy"
)

// maxMedia bounds the animations kept in memory: a page or two of tiles.
const maxMedia = 96

// maxDownload bounds one animation's file. KLIPY's full-size GIFs run to
// several MB (a 480×360 clip is often over 4 MiB), so the cap is generous.
// What matters is that an over-limit body fails loudly instead of being
// cut short: a truncated GIF cannot be decoded at all.
const maxDownload = 32 << 20

// maxParallel is how many animations download at once. A page of favorites
// asks for dozens; all at once they starve each other until each times out.
const maxParallel = 4

// downloadTimeout bounds one animation's whole download.
const downloadTimeout = 40 * time.Second

// Controller is the picker's model and its loading.
type Controller struct {
	client *klipy.Client
	http   *http.Client
	update func(func())
	path   string // where favorites and recents are kept

	saved saved

	// Per-tab models; Model returns the current one.
	gifs, stickers kit.PickerModel

	// What the models show, to notice when the state asks for another.
	shownTab           int
	shownQuery, browse string
	shown              bool
	page               int
	gen                int // bumps on every new query; stale results are dropped
	debounce           *time.Timer
	cats               []klipy.Category
	catsLoaded         bool
	trendPreview       string // the top trending GIF, the Trending tile's picture

	items map[string]klipy.Item // by tile ID, to send and favorite
	media *mediaCache
	sem   chan struct{} // a slot per download running

	wantMu sync.Mutex
	wanted map[string]bool // URLs something shows or waits for

	// Local folders as a source (folders.go).
	folders     []string
	local       []klipy.Item
	scanned     time.Time
	scanning    bool
	rescanAgain bool
	wasOpen     bool
}

// saved is what persists across runs.
type saved struct {
	CustomerID     string       `json:"customer_id"`
	KlipyKey       string       `json:"klipy_key,omitempty"` // the user's own, from Settings → GIFs
	Favorites      []klipy.Item `json:"favorites"`
	RecentEmoji    []string     `json:"recent_emoji"`
	RecentStickers []klipy.Item `json:"recent_stickers"`
	// RecentReactions are the emoji the user reacted with, newest first: the
	// hover toolbar offers them.
	RecentReactions []string `json:"recent_reactions,omitempty"`
}

// RecentReactions are the emoji the user reacted with lately, newest first.
func (c *Controller) RecentReactions() []string { return c.saved.RecentReactions }

// UsedReaction notes an emoji the user reacted with (a custom emoji, an
// mxc:// key, is not kept here: the toolbar shows plain emoji).
func (c *Controller) UsedReaction(key string) {
	if key == "" || strings.HasPrefix(key, "mxc://") {
		return
	}
	c.saved.RecentReactions = kit.PushRecent(c.saved.RecentReactions, key, 12)
	c.save()
}

// New returns a controller over the user's saved favorites and recents,
// kept in their config directory, as the app uses it.
func New(client *klipy.Client) *Controller {
	dir, _ := os.UserConfigDir()
	return newAt(client, filepath.Join(dir, "slopement", "picker.json"))
}

// newAt returns a controller that keeps its state at path, loading what
// is already there. Tests give it a scratch directory, so they never
// touch the user's saved favorites.
func newAt(client *klipy.Client, path string) *Controller {
	c := &Controller{
		client: client,
		http:   &http.Client{Timeout: downloadTimeout},
		path:   path,
		items:  map[string]klipy.Item{},
		media:  newMediaCache(maxMedia),
		sem:    make(chan struct{}, maxParallel),
	}
	c.load()
	if client != nil {
		client.SetKey(c.saved.KlipyKey) // a key from the environment stays
	}
	return c
}

// SetKlipyKey is the user's own KLIPY key (Settings → GIFs): kept on this
// device, used at once. "" removes it. A key in the environment
// (KLIPY_API_KEY) is used instead while there is one.
func (c *Controller) SetKlipyKey(key string) {
	c.saved.KlipyKey = strings.TrimSpace(key)
	c.save()
	if c.client != nil {
		c.client.SetKey(c.saved.KlipyKey)
	}
	// What the picker showed (or failed to) was without it: ask again.
	c.catsLoaded, c.shown = false, false
	c.gen++
}

// KlipyKey is the key the user entered ("" if none).
func (c *Controller) KlipyKey() string { return c.saved.KlipyKey }

// KlipyKeyFromEnvironment says the key in use comes from KLIPY_API_KEY, so
// the one entered in Settings is not.
func (c *Controller) KlipyKeyFromEnvironment() bool {
	return c.client != nil && c.client.KeyFromEnvironment()
}

// HasKlipyKey says GIF and sticker search is available.
func (c *Controller) HasKlipyKey() bool { return c.client != nil && c.client.HasKey() }

// NewWithKey returns a controller over a KLIPY client of the environment's
// key, with the install's anonymous customer ID.
func NewWithKey() *Controller {
	c := New(nil)
	if c.saved.CustomerID == "" {
		b := make([]byte, 12)
		rand.Read(b)
		c.saved.CustomerID = hex.EncodeToString(b)
		c.save()
	}
	c.client = klipy.New(klipy.Config{CustomerID: c.saved.CustomerID})
	c.client.SetKey(c.saved.KlipyKey)
	return c
}

// SetUpdate gives the function that runs work on the UI thread.
func (c *Controller) SetUpdate(update func(func())) { c.update = update }

func (c *Controller) post(fn func()) {
	if c.update != nil {
		c.update(fn)
	}
}

func (c *Controller) load() {
	b, err := os.ReadFile(c.path)
	if err == nil {
		json.Unmarshal(b, &c.saved)
	}
	for i := range c.saved.Favorites {
		f := &c.saved.Favorites[i]
		f.Full.URL = originalFormat(unproxy(f.Full.URL)) // imported as a WebP, which cannot be shown: the GIF
		f.Thumb.URL = originalFormat(unproxy(f.Thumb.URL))
		if f.Thumb.URL == f.Full.URL {
			f.Thumb.URL = thumbURL(f.Full.URL) // imported before thumbnails: the full GIF is no tile
		}
	}
}

func (c *Controller) save() {
	b, err := json.MarshalIndent(c.saved, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(c.path), 0o700)
	os.WriteFile(c.path, b, 0o600)
}

// Model returns the model of the state's tab, starting the loads the state
// asks for. Call it in the view, before kit.Picker.
func (c *Controller) Model(st *kit.PickerState) *kit.PickerModel {
	if !st.Open {
		c.wasOpen = false
		return &c.gifs
	}
	if !c.wasOpen {
		c.wasOpen = true
		c.rescanIfStale() // a file added since is looked for as the picker opens
	}
	if !c.shown || st.Tab != c.shownTab || st.Query != c.shownQuery || st.Browse != c.browse {
		queryChanged := st.Query != c.shownQuery && c.shown && st.Tab == c.shownTab
		c.shown, c.shownTab, c.shownQuery, c.browse = true, st.Tab, st.Query, st.Browse
		c.refresh(queryChanged)
	}
	switch st.Tab {
	case kit.TabStickers:
		c.stickers.RecentStickers = c.tiles(c.saved.RecentStickers)
		return &c.stickers
	case kit.TabEmoji:
		c.gifs.RecentEmoji = c.saved.RecentEmoji
		m := &kit.PickerModel{RecentEmoji: c.saved.RecentEmoji}
		return m
	}
	c.gifs.Front = c.front()
	// Stars follow the favorites as they change.
	for i := range c.gifs.Results {
		c.gifs.Results[i].Favorite = c.isFavorite(c.gifs.Results[i].ID)
	}
	return &c.gifs
}

// refresh starts loading what the state shows; a typed query waits 300 ms
// for the typing to pause.
func (c *Controller) refresh(typed bool) {
	c.gen++
	c.page = 1
	if c.debounce != nil {
		c.debounce.Stop()
	}
	m := c.model()
	if m == nil {
		return
	}
	hasKey := c.client != nil && c.client.HasKey()
	m.Notice = ""
	if c.shownTab == kit.TabGIFs && c.shownQuery == "" {
		if hasKey && !c.catsLoaded {
			c.fetchCategories()
		}
		switch c.browse {
		case "":
			m.Results, m.Loading = nil, false
			if !hasKey && len(c.saved.Favorites) == 0 && len(c.local) == 0 {
				m.Notice = noKeyNotice
			}
			return
		case "favorites":
			m.Title, m.Results, m.HasMore, m.Loading = "Favorites", c.tiles(c.saved.Favorites), false, false
			if len(m.Results) == 0 {
				m.Notice = "Star a GIF to keep it here"
			}
			return
		case "folders":
			m.Results, m.HasMore, m.Loading = nil, false, false
			c.refreshLocalViews()
			return
		}
	}
	if !hasKey {
		// Without a key there is KLIPY to search no more, but the folders
		// still are.
		m.Results, m.HasMore, m.Loading = nil, false, false
		if c.shownTab == kit.TabGIFs && c.shownQuery != "" {
			m.Title, m.Results = "My folders", c.tiles(c.localMatches(c.shownQuery, 60))
		}
		if len(m.Results) == 0 {
			m.Notice = noKeyNotice
		}
		return
	}
	m.Loading = true
	if typed {
		gen := c.gen
		c.debounce = time.AfterFunc(300*time.Millisecond, func() {
			c.post(func() {
				if gen == c.gen {
					c.fetch(gen)
				}
			})
		})
		return
	}
	c.fetch(c.gen)
}

func (c *Controller) model() *kit.PickerModel {
	switch c.shownTab {
	case kit.TabGIFs:
		return &c.gifs
	case kit.TabStickers:
		return &c.stickers
	}
	return nil
}

func (c *Controller) kind() klipy.Kind {
	if c.shownTab == kit.TabStickers {
		return klipy.Stickers
	}
	return klipy.GIFs
}

// fetch loads page c.page of what is shown, replacing the results on the
// first page and adding to them after.
func (c *Controller) fetch(gen int) {
	kind, query, browse, page := c.kind(), c.shownQuery, c.browse, c.page
	title := ""
	switch {
	case query != "":
	case browse == "trending" || kind == klipy.Stickers:
		title = "Trending"
	case browse != "":
		query, title = browse, browse
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var p klipy.Page
		var err error
		if query == "" {
			p, err = c.client.Trending(ctx, kind, page, 24)
		} else {
			p, err = c.client.Search(ctx, kind, query, page, 24)
		}
		c.post(func() {
			if gen != c.gen {
				return // the user has moved on
			}
			m := c.model()
			if m == nil {
				return
			}
			m.Loading = false
			if err != nil {
				m.Notice = errorNotice(err)
				return
			}
			tiles := make([]kit.PickerTile, 0, len(p.Items))
			for _, it := range p.Items {
				c.items[it.ID] = it
				tiles = append(tiles, c.tile(it))
			}
			if page == 1 {
				if kind == klipy.GIFs && query != "" && c.shownQuery == query {
					// The user's own files lead the search, found by name.
					tiles = append(c.tiles(c.localMatches(query, 12)), tiles...)
				}
				m.Results = tiles
			} else {
				m.Results = append(m.Results, tiles...)
			}
			m.Title, m.HasMore = title, p.HasNext
		})
	}()
}

const noKeyNotice = "Add your own KLIPY key in Settings → GIFs to search GIFs and stickers"

func errorNotice(err error) string {
	if errors.Is(err, klipy.ErrNoKey) {
		return noKeyNotice
	}
	return "Couldn't reach KLIPY. Check your connection."
}

func (c *Controller) fetchCategories() {
	c.catsLoaded = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cats, err := c.client.Categories(ctx, klipy.GIFs)
		top, terr := c.client.Trending(ctx, klipy.GIFs, 1, 1)
		c.post(func() {
			if err != nil {
				c.catsLoaded = false
				c.gifs.Notice = errorNotice(err)
				return
			}
			c.cats = cats
			if terr == nil && len(top.Items) > 0 {
				c.trendPreview = top.Items[0].Thumb.URL
			}
		})
	}()
}

// front is the GIF tab's front page: favorites, trending, categories.
func (c *Controller) front() []kit.PickerTile {
	hasKey := c.client != nil && c.client.HasKey()
	if !hasKey && len(c.saved.Favorites) == 0 && len(c.local) == 0 {
		return nil
	}
	out := make([]kit.PickerTile, 0, len(c.cats)+3)
	fav := kit.PickerTile{ID: "front:favorites", Label: "Favorites", Kind: kit.TileFavorites}
	if len(c.saved.Favorites) > 0 {
		fav.Player, fav.Still = c.mediaOf(c.saved.Favorites[0].Thumb.URL)
	}
	out = append(out, fav)
	if len(c.local) > 0 {
		folders := kit.PickerTile{ID: "front:folders", Label: "My folders", Kind: kit.TileFolders}
		folders.Player, folders.Still = c.mediaOf(c.local[0].Thumb.URL)
		out = append(out, folders)
	}
	if !hasKey {
		return out
	}
	trend := kit.PickerTile{ID: "front:trending", Label: "Trending GIFs", Kind: kit.TileTrending}
	trend.Player, trend.Still = c.mediaOf(c.trendPreview)
	out = append(out, trend)
	for _, cat := range c.cats {
		t := kit.PickerTile{ID: "cat:" + cat.Query, Label: cat.Name, Query: cat.Query, Kind: kit.TileCategory}
		t.Player, t.Still = c.mediaOf(cat.Preview)
		out = append(out, t)
	}
	return out
}

func (c *Controller) tiles(items []klipy.Item) []kit.PickerTile {
	out := make([]kit.PickerTile, 0, len(items))
	for _, it := range items {
		c.items[it.ID] = it
		out = append(out, c.tile(it))
	}
	return out
}

func (c *Controller) tile(it klipy.Item) kit.PickerTile {
	t := kit.PickerTile{ID: it.ID, Label: it.Title, W: float32(it.Thumb.Width), H: float32(it.Thumb.Height),
		Favorite: c.isFavorite(it.ID)}
	// A tile starts loading when it scrolls near the view (Want), not when
	// the page is built: a page of favorites would otherwise ask for every
	// one at once.
	url := it.Thumb.URL
	if e, ok := c.media.get(url); ok {
		t.Player = e.player
	}
	t.Failed, t.Gone = c.media.failed(url), c.media.isGone(url)
	t.Want = func() { c.mediaOf(url) }
	t.Drop = func() { c.setWanted(url, false) }
	return t
}

// mediaOf returns the animation of url once loaded, starting its load.
func (c *Controller) mediaOf(url string) (*ui.VideoPlayer, ui.ImageSource) {
	if url == "" {
		return nil, nil
	}
	if e, ok := c.media.get(url); ok {
		return e.player, nil
	}
	c.setWanted(url, true)
	if !c.media.start(url) {
		return nil, nil
	}
	go func() {
		c.sem <- struct{}{} // at most maxParallel downloads at once
		if !c.isWanted(url) {
			// Its tile scrolled away while it waited its turn: don't
			// download what nobody sees. (It starts over if it comes back.)
			<-c.sem
			c.post(func() { c.media.cancel(url); c.refreshTiles() })
			return
		}
		var data []byte
		var err error
		if IsLocalURL(url) {
			data, err = readLocal(url)
		} else {
			data, err = c.download(url)
		}
		<-c.sem
		var d ui.VideoDecoder
		if err == nil {
			d, err = newDecoder(data)
		}
		c.post(func() {
			if err == nil {
				var pl *ui.VideoPlayer
				if pl, err = ui.NewVideoPlayer(d); err == nil {
					pl.SetLoop(true).SetMuted(true)
					c.media.put(url, pl)
					c.refreshTiles()
					return
				}
			}
			c.failed(url, isGone(err))
		})
	}()
	return nil, nil
}

// setWanted says whether something still shows url's animation: a tile in
// the view, or a message. A load that has not begun when nothing wants it
// any more is dropped.
func (c *Controller) setWanted(url string, on bool) {
	c.wantMu.Lock()
	defer c.wantMu.Unlock()
	if c.wanted == nil {
		c.wanted = map[string]bool{}
	}
	if on {
		c.wanted[url] = true
	} else {
		delete(c.wanted, url)
	}
}

func (c *Controller) isWanted(url string) bool {
	c.wantMu.Lock()
	defer c.wantMu.Unlock()
	return c.wanted[url]
}

// failed notes a failed load: the tile says so, and a tile or message
// still on screen tries again after a wait.
func (c *Controller) failed(url string, gone bool) {
	if gone {
		c.media.markGone(url) // it will not come back: no retries, no tap to retry
		c.refreshTiles()
		return
	}
	wait := c.media.fail(url)
	c.refreshTiles()
	if wait > 0 {
		time.AfterFunc(wait, func() { c.post(c.refreshTiles) }) // wakes the view, which asks again
	}
}

// Retry loads a tile's animation again at once: a tap on a tile that
// could not load.
func (c *Controller) Retry(t *kit.PickerTile) {
	if it, ok := c.items[t.ID]; ok && c.media.retry(it.Thumb.URL) {
		c.mediaOf(it.Thumb.URL)
		c.refreshTiles()
	}
}

// Animation returns the looping animation of a GIF's URL once loaded
// (nil until then, while it loads): for a sent GIF in the timeline.
func (c *Controller) Animation(url string) *ui.VideoPlayer {
	p, _ := c.mediaOf(url)
	return p
}

// refreshTiles gives the shown tiles the animations loaded since.
func (c *Controller) refreshTiles() {
	for _, m := range []*kit.PickerModel{&c.gifs, &c.stickers} {
		for i := range m.Results {
			t := &m.Results[i]
			if it, ok := c.items[t.ID]; ok {
				if e, ok := c.media.get(it.Thumb.URL); ok {
					t.Player = e.player
				}
				t.Failed, t.Gone = c.media.failed(it.Thumb.URL), c.media.isGone(it.Thumb.URL)
			}
		}
	}
}

// statusError is a download the server refused.
type statusError struct {
	code int
	text string
}

func (e *statusError) Error() string { return e.text }

// isGone reports whether a load failed for good: the server says the file is
// forbidden or gone (a Discord attachment link that has expired is a 403), or
// the local file is missing. A timeout or a 5xx is not: it may work later.
func isGone(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.code == http.StatusForbidden || se.code == http.StatusNotFound || se.code == http.StatusGone
	}
	return errors.Is(err, os.ErrNotExist)
}

func (c *Controller) download(url string) ([]byte, error) {
	res, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, &statusError{res.StatusCode, res.Status}
	}
	if res.ContentLength > maxDownload {
		return nil, fmt.Errorf("animation is %d bytes, over the %d-byte limit", res.ContentLength, maxDownload)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("animation is over the %d-byte limit", maxDownload)
	}
	return data, nil
}

func (c *Controller) isFavorite(id string) bool {
	for _, f := range c.saved.Favorites {
		if f.ID == id {
			return true
		}
	}
	return false
}

// Chosen is what the user picked, to send.
type Chosen struct {
	Emoji string
	Item  *klipy.Item
}

// Handle applies the picker's result: favorites, recents, paging. It
// returns what to send, if anything.
func (c *Controller) Handle(st *kit.PickerState, r kit.PickerResult) (out Chosen) {
	if r.Retry != nil {
		c.Retry(r.Retry)
	}
	if r.More {
		if m := c.model(); m != nil && m.HasMore && !m.Loading {
			c.page++
			m.Loading = true
			c.fetch(c.gen)
		}
	}
	if t := r.ToggleFav; t != nil {
		if it, ok := c.items[t.ID]; ok {
			if c.isFavorite(it.ID) {
				favs := c.saved.Favorites[:0]
				for _, f := range c.saved.Favorites {
					if f.ID != it.ID {
						favs = append(favs, f)
					}
				}
				c.saved.Favorites = favs
			} else {
				c.saved.Favorites = append([]klipy.Item{it}, c.saved.Favorites...)
			}
			c.save()
		}
	}
	if r.Emoji != "" {
		c.saved.RecentEmoji = kit.PushRecent(c.saved.RecentEmoji, r.Emoji, 18)
		c.save()
		out.Emoji = r.Emoji
	}
	if r.Item != nil && r.Retry == nil && !r.Item.Gone { // a tap on a tile that failed to load retries it; one that is gone sends nothing
		if it, ok := c.items[r.Item.ID]; ok {
			if it.Kind == klipy.Stickers {
				c.saved.RecentStickers = pushItem(c.saved.RecentStickers, it, 8)
				c.save()
			}
			out.Item = &it
			c.registerShare(it, st.Query)
		}
		st.Query, st.Browse = "", ""
	}
	return out
}

// registerShare tells KLIPY an item of its was sent, with the search that led
// to it (KLIPY's terms ask for it, and it improves the results). Items that
// are not KLIPY's (Discord imports, GIFs starred from a message) have no
// slug and are not reported. It runs in the background and never fails the send.
func (c *Controller) registerShare(it klipy.Item, query string) {
	if c.client == nil || !c.client.HasKey() || it.Slug == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = c.client.Share(ctx, it.Kind, it.Slug, query)
	}()
}

func pushItem(list []klipy.Item, it klipy.Item, max int) []klipy.Item {
	out := []klipy.Item{it}
	for _, x := range list {
		if x.ID != it.ID && len(out) < max {
			out = append(out, x)
		}
	}
	return out
}

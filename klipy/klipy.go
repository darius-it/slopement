// Package klipy is a small client of KLIPY (klipy.com), the GIF and sticker
// provider of the composer's picker, on KLIPY's native API:
//
//	GET https://api.klipy.com/api/v1/{key}/{gifs|stickers}/{trending|search|categories}
//
// verified against live responses (2026-11-21): items are in data.data,
// paging in data.has_next, each item's renditions in file.{hd,md,sm,xs}.
// {gif,webp,jpg,mp4,webm,png}, and items whose type is not the kind's
// (ads) are skipped.
//
// The key comes from Config.Key or KLIPY_API_KEY (.env in development).
package klipy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Kind is a kind of content.
type Kind int

const (
	GIFs Kind = iota
	Stickers
)

func (k Kind) path() string {
	if k == Stickers {
		return "stickers"
	}
	return "gifs"
}

func (k Kind) itemType() string {
	if k == Stickers {
		return "sticker"
	}
	return "gif"
}

// ErrNoKey is returned while no API key is configured.
var ErrNoKey = errors.New("klipy: no API key (set KLIPY_API_KEY in .env)")

// Config configures a Client.
type Config struct {
	// Key is the API key; empty reads KLIPY_API_KEY.
	Key string
	// CustomerID identifies the user to KLIPY for recents and ranking,
	// anonymously: a random ID kept per install, never the MXID.
	CustomerID string
	// Locale, as "en_US"; empty is KLIPY's default.
	Locale string
	// BaseURL overrides https://api.klipy.com, for tests.
	BaseURL string
	// HTTP overrides the HTTP client.
	HTTP *http.Client
}

// Client queries KLIPY. It is safe for concurrent use; call it off the UI
// thread and publish results with Window.Update.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a client.
func New(cfg Config) *Client {
	if cfg.Key == "" {
		cfg.Key = os.Getenv("KLIPY_API_KEY")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.klipy.com"
	}
	h := cfg.HTTP
	if h == nil {
		h = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{cfg: cfg, http: h}
}

// HasKey reports whether a key is configured.
func (c *Client) HasKey() bool { return c.cfg.Key != "" }

// Rendition is one file of an item.
type Rendition struct {
	URL           string `json:"url"`
	Width, Height int
	Size          int `json:"size"`
}

type rendition struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Size   int    `json:"size"`
}

// Item is a GIF or sticker.
type Item struct {
	ID    string
	Slug  string
	Title string
	Kind  Kind
	// Thumb is the smallest animated rendition, for the picker's grid
	// (xs gif, about 90 px).
	Thumb Rendition
	// Still is a still frame of Thumb's size (xs jpg, or png for stickers).
	Still Rendition
	// Full is what is sent: md gif (or webp for stickers, which keep
	// their transparency).
	Full Rendition
	// BlurPreview is KLIPY's tiny placeholder, if any (a data URL).
	BlurPreview string
}

// Page is a page of results.
type Page struct {
	Items   []Item
	Page    int
	HasNext bool
}

// Category is a search term with a picture, as "lol".
type Category struct {
	Name, Query string
	// Preview is an animated GIF of the category.
	Preview string
}

type apiItem struct {
	ID          json.Number                     `json:"id"`
	Slug        string                          `json:"slug"`
	Title       string                          `json:"title"`
	Type        string                          `json:"type"`
	BlurPreview string                          `json:"blur_preview"`
	File        map[string]map[string]rendition `json:"file"`
}

type apiPage struct {
	Result bool `json:"result"`
	Data   struct {
		Data        []apiItem `json:"data"`
		CurrentPage int       `json:"current_page"`
		HasNext     bool      `json:"has_next"`
	} `json:"data"`
}

func pick(f map[string]map[string]rendition, sizes []string, formats ...string) Rendition {
	for _, s := range sizes {
		for _, fm := range formats {
			if r, ok := f[s][fm]; ok && r.URL != "" {
				return Rendition{URL: r.URL, Width: r.Width, Height: r.Height, Size: r.Size}
			}
		}
	}
	return Rendition{}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if c.cfg.Key == "" {
		return ErrNoKey
	}
	if c.cfg.CustomerID != "" {
		q.Set("customer_id", c.cfg.CustomerID)
	}
	if c.cfg.Locale != "" {
		q.Set("locale", c.cfg.Locale)
	}
	u := c.cfg.BaseURL + "/api/v1/" + url.PathEscape(c.cfg.Key) + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return redact(err, c.cfg.Key)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("klipy: %s: %s", path, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// redact keeps the key out of errors, which may be logged.
func redact(err error, key string) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("klipy: %s: %w", ue.Op, ue.Err)
	}
	return err
}

func (c *Client) page(ctx context.Context, kind Kind, endpoint string, q url.Values, page, perPage int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 24
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	var ap apiPage
	if err := c.get(ctx, "/"+kind.path()+"/"+endpoint, q, &ap); err != nil {
		return Page{}, err
	}
	p := Page{Page: ap.Data.CurrentPage, HasNext: ap.Data.HasNext, Items: make([]Item, 0, len(ap.Data.Data))}
	for _, a := range ap.Data.Data {
		if a.Type != "" && a.Type != kind.itemType() {
			continue // ads and other kinds
		}
		it := Item{ID: a.ID.String(), Slug: a.Slug, Title: a.Title, Kind: kind, BlurPreview: a.BlurPreview}
		small := []string{"xs", "sm", "md"}
		it.Thumb = pick(a.File, small, "gif")
		if kind == Stickers {
			it.Still = pick(a.File, small, "png", "webp")
			// A GIF first: it animates everywhere and mygo can show it (it
			// cannot decode KLIPY's animated WebPs, so a sticker sent as one
			// would be blank here). WebP and PNG are the fallbacks.
			it.Full = pick(a.File, []string{"md", "sm", "hd"}, "gif", "webp", "png")
		} else {
			it.Still = pick(a.File, small, "jpg")
			it.Full = pick(a.File, []string{"md", "sm", "hd"}, "gif")
		}
		if it.Full.URL == "" || it.Thumb.URL == "" {
			continue
		}
		p.Items = append(p.Items, it)
	}
	return p, nil
}

// Share tells KLIPY an item was sent (POST …/{gifs|stickers}/share/{slug}),
// as its terms ask: it improves ranking. query is the search that led to it,
// "" when it came from trending, recents or favorites. The user is only the
// install's anonymous customer ID. A failure is returned but is not worth
// more than a log line.
func (c *Client) Share(ctx context.Context, kind Kind, slug, query string) error {
	if c.cfg.Key == "" {
		return ErrNoKey
	}
	if slug == "" {
		return errors.New("klipy: nothing to share without a slug")
	}
	body, _ := json.Marshal(map[string]string{"customer_id": c.cfg.CustomerID, "q": query})
	u := c.cfg.BaseURL + "/api/v1/" + url.PathEscape(c.cfg.Key) + "/" + kind.path() + "/share/" + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return redact(err, c.cfg.Key)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("klipy: share: %s", res.Status)
	}
	return nil
}

// Trending returns the trending items of kind.
func (c *Client) Trending(ctx context.Context, kind Kind, page, perPage int) (Page, error) {
	return c.page(ctx, kind, "trending", url.Values{}, page, perPage)
}

// Search returns the items of kind matching query.
func (c *Client) Search(ctx context.Context, kind Kind, query string, page, perPage int) (Page, error) {
	return c.page(ctx, kind, "search", url.Values{"q": {query}}, page, perPage)
}

// Categories returns the categories of kind, as the picker's tiles.
func (c *Client) Categories(ctx context.Context, kind Kind) ([]Category, error) {
	var out struct {
		Data struct {
			Categories []struct {
				Category   string `json:"category"`
				Query      string `json:"query"`
				PreviewURL string `json:"preview_url"`
			} `json:"categories"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/"+kind.path()+"/categories", url.Values{}, &out); err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(out.Data.Categories))
	for _, t := range out.Data.Categories {
		cats = append(cats, Category{Name: t.Category, Query: t.Query, Preview: t.PreviewURL})
	}
	return cats, nil
}

// Package klipy is a small client of KLIPY (klipy.com), the GIF, sticker,
// clip and meme provider of the composer's picker.
//
// KLIPY serves Tenor's v2 API at api.klipy.com (its migration guide: swap
// the base URL and the key), which this client speaks for GIFs and
// stickers. Clips, memes and AI emoji have only KLIPY's own API, whose
// parameters must be checked against real responses with a key before
// they are wired up (see TODO below); until then they report
// ErrUnsupported and the picker shows its placeholders.
//
// The key comes from Config.Key or the KLIPY_API_KEY environment variable.
// Test keys (100 calls an hour) come from https://partner.klipy.com/api-keys.
package klipy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Clips
	Memes
)

// ErrNoKey is returned while no API key is configured.
var ErrNoKey = errors.New("klipy: no API key (set KLIPY_API_KEY)")

// ErrUnsupported is returned for kinds not wired up yet.
var ErrUnsupported = errors.New("klipy: kind not supported yet")

// Config configures a Client.
type Config struct {
	// Key is the API key; empty reads KLIPY_API_KEY.
	Key string
	// ClientKey names the app to KLIPY, as Tenor's client_key.
	ClientKey string
	// Locale, as "en_US"; empty is KLIPY's default.
	Locale string
	// ContentFilter is "off", "low", "medium" or "high"; empty is medium.
	ContentFilter string
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
	if cfg.ClientKey == "" {
		cfg.ClientKey = "slopement"
	}
	if cfg.ContentFilter == "" {
		cfg.ContentFilter = "medium"
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
	URL    string
	Width  int
	Height int
	Size   int
}

// Item is a GIF or sticker.
type Item struct {
	ID    string
	Title string
	// Preview is small, for the grid (tinygif / tinywebp / nanogif).
	Preview Rendition
	// Full is what is sent (gif, or webp for stickers).
	Full Rendition
	// MP4 is a light animated preview where offered.
	MP4 Rendition
}

// Page is a page of results; Next continues it, empty at the end.
type Page struct {
	Items []Item
	Next  string
}

// Category is a search term with a picture, as "lol".
type Category struct {
	Name, Search string
	Image        string
}

type tenorMedia struct {
	URL  string `json:"url"`
	Dims []int  `json:"dims"`
	Size int    `json:"size"`
}

type tenorResult struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	Description  string                `json:"content_description"`
	MediaFormats map[string]tenorMedia `json:"media_formats"`
}

type tenorPage struct {
	Results []tenorResult `json:"results"`
	Next    string        `json:"next"`
}

func rendition(m map[string]tenorMedia, keys ...string) Rendition {
	for _, k := range keys {
		if v, ok := m[k]; ok && v.URL != "" {
			r := Rendition{URL: v.URL, Size: v.Size}
			if len(v.Dims) == 2 {
				r.Width, r.Height = v.Dims[0], v.Dims[1]
			}
			return r
		}
	}
	return Rendition{}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if c.cfg.Key == "" {
		return ErrNoKey
	}
	q.Set("key", c.cfg.Key)
	q.Set("client_key", c.cfg.ClientKey)
	if c.cfg.Locale != "" {
		q.Set("locale", c.cfg.Locale)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("klipy: %s: %s", path, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) page(ctx context.Context, path string, kind Kind, q url.Values, limit int, pos string) (Page, error) {
	switch kind {
	case GIFs:
		q.Set("media_filter", "tinygif,nanogif,gif,tinymp4")
	case Stickers:
		q.Set("searchfilter", "sticker")
		q.Set("media_filter", "tinywebp_transparent,webp_transparent,tinygif_transparent,gif_transparent")
	default:
		// TODO(klipy): clips and memes use KLIPY's own API
		// (/clips-api, /memes-api in docs.klipy.com); verify params and
		// response fields against a real response before wiring up.
		return Page{}, ErrUnsupported
	}
	q.Set("contentfilter", c.cfg.ContentFilter)
	if limit <= 0 {
		limit = 24
	}
	q.Set("limit", strconv.Itoa(limit))
	if pos != "" {
		q.Set("pos", pos)
	}
	var tp tenorPage
	if err := c.get(ctx, path, q, &tp); err != nil {
		return Page{}, err
	}
	p := Page{Next: tp.Next, Items: make([]Item, 0, len(tp.Results))}
	for _, r := range tp.Results {
		it := Item{ID: r.ID, Title: r.Title}
		if it.Title == "" {
			it.Title = r.Description
		}
		if kind == Stickers {
			it.Preview = rendition(r.MediaFormats, "tinywebp_transparent", "tinygif_transparent", "webp_transparent")
			it.Full = rendition(r.MediaFormats, "webp_transparent", "gif_transparent")
		} else {
			it.Preview = rendition(r.MediaFormats, "tinygif", "nanogif", "gif")
			it.Full = rendition(r.MediaFormats, "gif", "tinygif")
			it.MP4 = rendition(r.MediaFormats, "tinymp4")
		}
		if it.Full.URL == "" {
			continue // nothing sendable
		}
		p.Items = append(p.Items, it)
	}
	return p, nil
}

// Trending returns the featured items of kind.
func (c *Client) Trending(ctx context.Context, kind Kind, limit int, pos string) (Page, error) {
	return c.page(ctx, "/v2/featured", kind, url.Values{}, limit, pos)
}

// Search returns the items of kind matching query.
func (c *Client) Search(ctx context.Context, kind Kind, query string, limit int, pos string) (Page, error) {
	return c.page(ctx, "/v2/search", kind, url.Values{"q": {query}}, limit, pos)
}

// Categories returns the categories of GIFs, as tiles of the picker.
func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	var out struct {
		Tags []struct {
			Name   string `json:"name"`
			Search string `json:"searchterm"`
			Image  string `json:"image"`
		} `json:"tags"`
	}
	if err := c.get(ctx, "/v2/categories", url.Values{"type": {"featured"}}, &out); err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(out.Tags))
	for _, t := range out.Tags {
		cats = append(cats, Category{Name: t.Name, Search: t.Search, Image: t.Image})
	}
	return cats, nil
}

// RegisterShare tells KLIPY an item was sent, which its terms ask for and
// which improves its results. Errors can be ignored.
func (c *Client) RegisterShare(ctx context.Context, id, query string) error {
	var out json.RawMessage
	q := url.Values{"id": {id}}
	if query != "" {
		q.Set("q", query)
	}
	return c.get(ctx, "/v2/registershare", q, &out)
}

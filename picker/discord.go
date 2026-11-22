package picker

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"slopement/klipy"
)

// Discord's favorite GIFs, as darius-it/discord-gif-exporter writes them
// (github.com/darius-it/discord-gif-exporter): its JSON is Discord's
// FrecencyUserSettings.favoriteGifs.gifs, an object from each GIF's
// original URL to how Discord stores it:
//
//	{"https://tenor.com/view/cat-…": {"format": 2, "src": "//media.tenor.com/abcAAPo/cat.mp4",
//	                                  "width": 498, "height": 280, "order": 7}, …}
//
// format is 1 for an image, 2 for a video (Tenor's and Giphy's GIFs come
// as MP4s); src may lack its scheme.

type discordFavorite struct {
	Format int    `json:"format"`
	Src    string `json:"src"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Order  int    `json:"order"`
}

// ErrNotDiscordExport is returned for a file that isn't the exporter's.
var ErrNotDiscordExport = errors.New("not a Discord favorite GIFs export")

// parseDiscordFavorites reads an export into favorites, in Discord's order
// (oldest first, as Discord numbers them; reversed so the newest leads).
func parseDiscordFavorites(data []byte) ([]klipy.Item, error) {
	var m map[string]discordFavorite
	if err := json.Unmarshal(data, &m); err != nil || len(m) == 0 {
		return nil, ErrNotDiscordExport
	}
	type entry struct {
		key string
		f   discordFavorite
	}
	list := make([]entry, 0, len(m))
	for k, f := range m {
		if f.Src == "" && !strings.HasPrefix(k, "http") {
			return nil, ErrNotDiscordExport
		}
		list = append(list, entry{k, f})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].f.Order > list[j].f.Order })
	items := make([]klipy.Item, 0, len(list))
	for _, e := range list {
		u := discordGIF(e.key, e.f.Src)
		if u == "" {
			continue // a video we can't show as a GIF
		}
		if expiredLink(u, now()) {
			continue // a Discord attachment link past its expiry: it would only ever show as gone
		}
		sum := sha1.Sum([]byte(e.key))
		r := klipy.Rendition{URL: u, Width: e.f.Width, Height: e.f.Height}
		th := r
		th.URL = thumbURL(u)
		items = append(items, klipy.Item{
			ID:    "discord:" + hex.EncodeToString(sum[:8]),
			Title: titleOf(e.key),
			Kind:  klipy.GIFs,
			Thumb: th, Still: th, Full: r,
		})
	}
	return items, nil
}

// discordGIF finds a GIF for a favorite: its src or URL when they are
// GIFs, Tenor's GIF rendition of its MP4 (the four letters ending the
// media ID name the format: AAPo is MP4, AAAC GIF), Giphy's giphy.gif
// beside giphy.mp4; "" when there is none.
func discordGIF(key, src string) string {
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	src = originalFormat(unproxy(src))
	key = originalFormat(key)
	for _, u := range []string{src, key} {
		if strings.HasSuffix(strings.ToLower(strings.SplitN(u, "?", 2)[0]), ".gif") {
			return u
		}
	}
	p, err := url.Parse(src)
	if err != nil {
		return ""
	}
	host := strings.ToLower(p.Host)
	segs := strings.Split(strings.Trim(p.Path, "/"), "/")
	switch {
	case strings.HasSuffix(host, "tenor.com") && len(segs) == 2 && len(segs[0]) > 4:
		id := segs[0][:len(segs[0])-4] + "AAAC"
		name := strings.TrimSuffix(segs[1], path.Ext(segs[1])) + ".gif"
		return "https://media.tenor.com/" + id + "/" + name
	case strings.HasSuffix(host, "giphy.com") && strings.HasSuffix(p.Path, ".mp4"):
		return "https://" + p.Host + strings.TrimSuffix(p.Path, ".mp4") + ".gif"
	}
	return ""
}

// originalFormat undoes Discord's conversion of an attachment: a link with
// ?format=webp&animated=true serves an animated WebP, which mygo cannot show;
// without them Discord serves the file as it was uploaded (the GIF). Any
// other address is returned as it is.
func originalFormat(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	host := strings.ToLower(p.Host)
	if host != "cdn.discordapp.com" && host != "media.discordapp.net" {
		return u
	}
	q := p.Query()
	if q.Get("format") != "webp" {
		return u
	}
	if !strings.HasSuffix(strings.ToLower(p.Path), ".gif") {
		return u // a still or a WebP upload: the conversion is all there is
	}
	q.Del("format")
	q.Del("animated")
	p.RawQuery = q.Encode()
	return p.String()
}

// now is the clock, a test's to set.
var now = time.Now

// expiredLink reports whether u is a Discord attachment link whose signed
// expiry (the hex Unix time in its ex= parameter) has passed: Discord answers
// 403 for it from then on.
func expiredLink(u string, at time.Time) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	host := strings.ToLower(p.Host)
	if host != "cdn.discordapp.com" && host != "media.discordapp.net" {
		return false
	}
	ex, err := strconv.ParseInt(p.Query().Get("ex"), 16, 64)
	return err == nil && ex > 0 && time.Unix(ex, 0).Before(at)
}

// thumbURL is a small version of the GIF at u to show as a tile, where the
// host has one: Tenor's tinygif (the media ID's last letter says which
// rendition: C is the full GIF, M the tiny one, a seventh of the size).
// Anything else is u itself.
func thumbURL(u string) string {
	p, err := url.Parse(u)
	if err != nil || !strings.HasSuffix(strings.ToLower(p.Host), "tenor.com") {
		return u
	}
	segs := strings.Split(strings.Trim(p.Path, "/"), "/")
	if len(segs) != 2 || !strings.HasSuffix(segs[0], "AAAAC") || !strings.HasSuffix(segs[1], ".gif") {
		return u
	}
	segs[0] = strings.TrimSuffix(segs[0], "C") + "M"
	return "https://" + p.Host + "/" + strings.Join(segs, "/")
}

// unproxy undoes Discord's media proxy, which stores a link's picture as
// images-ext-N.discordapp.net/external/<hash>/https/<the real address>: most
// of a real export's Tenor favorites are that. Anything else is returned as is.
func unproxy(src string) string {
	p, err := url.Parse(src)
	if err != nil || !strings.HasPrefix(strings.ToLower(p.Host), "images-ext-") || !strings.HasSuffix(strings.ToLower(p.Host), ".discordapp.net") {
		return src
	}
	rest, ok := strings.CutPrefix(p.Path, "/external/")
	if !ok {
		return src
	}
	// <hash>/[the proxy's own options, as one encoded segment/]https/<address>
	_, after, ok := strings.Cut(rest, "/") // past the hash
	if !ok {
		return src
	}
	after = "/" + after
	i, j := strings.Index(after, "/https/"), strings.Index(after, "/http/")
	var addr string
	switch {
	case i >= 0 && (j < 0 || i < j):
		addr = after[i+len("/https/"):]
	case j >= 0:
		addr = after[j+len("/http/"):]
	}
	if addr == "" {
		return src
	}
	out := "https://" + addr
	if p.RawQuery != "" {
		out += "?" + p.RawQuery
	}
	return out
}

// titleOf makes a title of a favorite's URL: "cat-funny-1234" → "cat funny".
func titleOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "Discord favorite"
	}
	base := path.Base(u.Path)
	base = strings.TrimSuffix(base, path.Ext(base))
	words := strings.FieldsFunc(base, func(r rune) bool { return r == '-' || r == '_' })
	if n := len(words); n > 1 && strings.IndexFunc(words[n-1], func(r rune) bool { return r < '0' || r > '9' }) < 0 {
		words = words[:n-1] // a trailing ID
	}
	if len(words) == 0 {
		return "Discord favorite"
	}
	return strings.Join(words, " ")
}

// ImportDiscordFavorites adds the GIFs of a discord-gif-exporter export to
// the favorites, after those already there, skipping duplicates. It
// returns how many it added.
func (c *Controller) ImportDiscordFavorites(data []byte) (int, error) {
	items, err := parseDiscordFavorites(data)
	if err != nil {
		return 0, err
	}
	have := map[string]bool{}
	for _, f := range c.saved.Favorites {
		have[f.ID] = true
		have[f.Full.URL] = true
	}
	added := 0
	for _, it := range items {
		if have[it.ID] || have[it.Full.URL] {
			continue
		}
		have[it.ID] = true
		c.saved.Favorites = append(c.saved.Favorites, it)
		c.items[it.ID] = it
		added++
	}
	if added > 0 {
		c.save()
		if c.browse == "favorites" {
			c.gifs.Results = c.tiles(c.saved.Favorites)
			c.gifs.Notice = ""
		}
	}
	return added, nil
}

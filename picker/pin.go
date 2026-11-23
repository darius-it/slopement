package picker

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"slopement/klipy"
)

// Discord's attachment links are signed and last a day (the hex Unix time in
// ex=): a favorite imported from Discord worked yesterday and is gone today.
// So a favorite whose link will expire is copied to this device while the link
// still works, and the favorite then points at the copy (a file:// item, like
// the folders'), which never expires.

// expiringLink reports whether u is a Discord attachment link that works now
// but carries an expiry.
func expiringLink(u string, at time.Time) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	host := strings.ToLower(p.Host)
	if host != "cdn.discordapp.com" && host != "media.discordapp.net" {
		return false
	}
	if !strings.HasPrefix(p.Path, "/attachments/") {
		return false
	}
	ex, err := strconv.ParseInt(p.Query().Get("ex"), 16, 64)
	return err == nil && ex > 0 && time.Unix(ex, 0).After(at)
}

// pinDir is where the copies are kept, beside the picker's own file.
func (c *Controller) pinDir() string { return filepath.Join(filepath.Dir(c.path), "gifs") }

// PinExpiring copies the favorites whose links will expire, in the background,
// and points them at the copies as each is saved. Safe to call again: only
// what is still linked is fetched.
func (c *Controller) PinExpiring() {
	type job struct{ id, url string }
	var jobs []job
	for _, f := range c.saved.Favorites {
		if expiringLink(f.Full.URL, now()) {
			jobs = append(jobs, job{f.ID, f.Full.URL})
		}
	}
	if len(jobs) == 0 {
		return
	}
	if c.pinning {
		c.pinAgain = true // an import during a run: look again when it ends
		return
	}
	c.pinning = true
	run := c.update
	if run == nil {
		run = func(f func()) { f() }
	}
	go func() {
		defer run(func() {
			c.pinning = false
			c.dedupeFavorites() // a GIF imported again, or under a new signed link, is one favorite
			if c.pinAgain {
				c.pinAgain = false
				c.PinExpiring()
			}
		})
		for _, j := range jobs {
			data, err := c.download(j.url)
			if err != nil || !isPicture(data) {
				continue // it stays as it was: gone soon, or already
			}
			sum := sha1.Sum(data) // named by what it is: the same GIF twice is one file
			name := filepath.Join(c.pinDir(), hex.EncodeToString(sum[:8])+pictureExt(data))
			if err := os.MkdirAll(c.pinDir(), 0o700); err != nil {
				return
			}
			tmp := name + ".part"
			if os.WriteFile(tmp, data, 0o600) != nil || os.Rename(tmp, name) != nil {
				os.Remove(tmp)
				continue
			}
			j := j
			run(func() { c.pointAt(j.id, localScheme+name) })
		}
	}()
}

// pointAt makes a favorite's pictures the local file.
func (c *Controller) pointAt(id, local string) {
	for i := range c.saved.Favorites {
		f := &c.saved.Favorites[i]
		if f.ID != id {
			continue
		}
		f.Full.URL, f.Thumb.URL, f.Still.URL = local, local, local
		c.items[id] = *f
		c.save()
		if c.browse == "favorites" {
			c.gifs.Results = c.tiles(c.saved.Favorites)
		}
		return
	}
}

// unavailable says a favorite's picture cannot come back: its signed link has
// expired, or a load of it failed for good.
func (c *Controller) unavailable(url string) bool {
	return expiredLink(url, now()) || c.media.isGone(url)
}

// UnavailableCount is how many favorites show as unavailable.
func (c *Controller) UnavailableCount() int {
	n := 0
	for _, f := range c.saved.Favorites {
		if !IsLocalURL(f.Full.URL) && (c.unavailable(f.Full.URL) || c.unavailable(f.Thumb.URL)) {
			n++
		}
	}
	return n
}

// RemoveUnavailable drops the favorites that cannot come back, and says how
// many it dropped.
func (c *Controller) RemoveUnavailable() int {
	keep := c.saved.Favorites[:0:0]
	for _, f := range c.saved.Favorites {
		if !IsLocalURL(f.Full.URL) && (c.unavailable(f.Full.URL) || c.unavailable(f.Thumb.URL)) {
			delete(c.items, f.ID)
			continue
		}
		keep = append(keep, f)
	}
	n := len(c.saved.Favorites) - len(keep)
	if n > 0 {
		c.saved.Favorites = keep
		c.save()
		if c.browse == "favorites" {
			c.gifs.Results = c.tiles(c.saved.Favorites)
		}
	}
	return n
}

// sourceKey says which GIF a link is, whatever signature it carries: a Discord
// attachment is its path (the ex, is and hm parameters change with every
// export); any other link is itself.
func sourceKey(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	host := strings.ToLower(p.Host)
	if (host == "cdn.discordapp.com" || host == "media.discordapp.net") && strings.HasPrefix(p.Path, "/attachments/") {
		return "discord:" + p.Path
	}
	return u
}

// favoriteIdentity is what makes two favorites the same GIF: a copy on this
// device by its bytes, a link by its source.
func favoriteIdentity(f klipy.Item, hashes map[string]string) string {
	if !IsLocalURL(f.Full.URL) {
		return "src:" + sourceKey(f.Full.URL)
	}
	path := LocalPath(f.Full.URL)
	if h, ok := hashes[path]; ok {
		return h
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "file:" + path
	}
	sum := sha1.Sum(data)
	h := "sha:" + hex.EncodeToString(sum[:])
	hashes[path] = h
	return h
}

// dedupeFavorites keeps the first of favorites that are the same GIF, and
// removes the copies on this device nothing points at any more. It says how
// many it dropped.
func (c *Controller) dedupeFavorites() int {
	hashes := map[string]string{}
	seen := map[string]bool{}
	keep := c.saved.Favorites[:0:0]
	for _, f := range c.saved.Favorites {
		id := favoriteIdentity(f, hashes)
		if seen[id] {
			delete(c.items, f.ID)
			continue
		}
		seen[id] = true
		keep = append(keep, f)
	}
	n := len(c.saved.Favorites) - len(keep)
	if n == 0 {
		return 0
	}
	c.saved.Favorites = keep
	c.save()
	// A copy no favorite points at is not kept.
	used := map[string]bool{}
	for _, f := range keep {
		if IsLocalURL(f.Full.URL) {
			used[LocalPath(f.Full.URL)] = true
		}
	}
	if entries, err := os.ReadDir(c.pinDir()); err == nil {
		for _, e := range entries {
			if p := filepath.Join(c.pinDir(), e.Name()); !used[p] && !strings.HasSuffix(p, ".part") {
				os.Remove(p)
			}
		}
	}
	if c.browse == "favorites" {
		c.gifs.Results = c.tiles(c.saved.Favorites)
	}
	return n
}

// FavoriteCount is how many favorites there are.
func (c *Controller) FavoriteCount() int { return len(c.saved.Favorites) }

// ResetFavorites removes every favorite and the copies kept for them, and says
// how many favorites there were. An import afterwards starts clean. (A copy
// being written at that moment is left to the next reset.)
func (c *Controller) ResetFavorites() int {
	n := len(c.saved.Favorites)
	for _, f := range c.saved.Favorites {
		delete(c.items, f.ID)
	}
	c.saved.Favorites = nil
	c.pinAgain = false
	c.save()
	if entries, err := os.ReadDir(c.pinDir()); err == nil {
		for _, e := range entries {
			os.Remove(filepath.Join(c.pinDir(), e.Name()))
		}
	}
	if c.browse == "favorites" {
		c.gifs.Results = c.tiles(c.saved.Favorites)
	}
	return n
}

// isPicture says data starts like a GIF, PNG, JPEG or WebP file.
func isPicture(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF8")) || bytes.HasPrefix(data, []byte("\x89PNG")) ||
		bytes.HasPrefix(data, []byte("\xff\xd8\xff")) || (len(data) > 12 && string(data[8:12]) == "WEBP")
}

func pictureExt(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("GIF8")):
		return ".gif"
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		return ".png"
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return ".jpg"
	}
	return ".webp"
}

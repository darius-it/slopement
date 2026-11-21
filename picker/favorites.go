package picker

import (
	"crypto/sha1"
	"encoding/hex"

	"slopement/klipy"
)

// GIFs seen in the timeline join the favorites by their URL: the star on
// a GIF in a message. They go in the same list as the picker's own and
// Discord's, newest first.

// FavoriteURL reports whether the GIF at url is a favorite.
func (c *Controller) FavoriteURL(url string) bool {
	for _, f := range c.saved.Favorites {
		if f.Full.URL == url {
			return true
		}
	}
	return false
}

// SetFavoriteURL adds the GIF at url (w×h, 0 if unknown) to the favorites,
// or removes it.
func (c *Controller) SetFavoriteURL(url string, w, h int, on bool) {
	if on == c.FavoriteURL(url) {
		return
	}
	if !on {
		out := c.saved.Favorites[:0]
		for _, f := range c.saved.Favorites {
			if f.Full.URL != url {
				out = append(out, f)
			}
		}
		c.saved.Favorites = out
	} else {
		sum := sha1.Sum([]byte(url))
		r := klipy.Rendition{URL: url, Width: w, Height: h}
		it := klipy.Item{ID: "url:" + hex.EncodeToString(sum[:8]), Title: titleOf(url), Kind: klipy.GIFs, Thumb: r, Still: r, Full: r}
		c.saved.Favorites = append([]klipy.Item{it}, c.saved.Favorites...)
		c.items[it.ID] = it
	}
	c.save()
	if c.browse == "favorites" {
		c.gifs.Results = c.tiles(c.saved.Favorites)
	}
}

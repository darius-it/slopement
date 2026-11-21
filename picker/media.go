package picker

import (
	"container/list"

	"github.com/egoist/mygo/ui"
)

// mediaCache keeps the newest animations by URL, closing the oldest
// beyond its limit. It is used on the UI thread only.
type mediaCache struct {
	max     int
	order   *list.List // of string URLs, newest first
	entries map[string]*mediaEntry
}

type mediaEntry struct {
	player  *ui.VideoPlayer
	loading bool
	failed  bool
	el      *list.Element
}

func newMediaCache(max int) *mediaCache {
	return &mediaCache{max: max, order: list.New(), entries: map[string]*mediaEntry{}}
}

// get returns the loaded entry of url, marking it as used.
func (m *mediaCache) get(url string) (*mediaEntry, bool) {
	e, ok := m.entries[url]
	if !ok || e.player == nil {
		return nil, false
	}
	m.order.MoveToFront(e.el)
	return e, true
}

// start reports whether url should load now: not loading, loaded or
// failed already.
func (m *mediaCache) start(url string) bool {
	if _, ok := m.entries[url]; ok {
		return false
	}
	m.entries[url] = &mediaEntry{loading: true}
	return true
}

func (m *mediaCache) fail(url string) {
	if e, ok := m.entries[url]; ok {
		e.loading, e.failed = false, true
	}
}

func (m *mediaCache) put(url string, p *ui.VideoPlayer) {
	e := m.entries[url]
	if e == nil {
		e = &mediaEntry{}
		m.entries[url] = e
	}
	e.player, e.loading = p, false
	e.el = m.order.PushFront(url)
	for m.order.Len() > m.max {
		last := m.order.Back()
		u := last.Value.(string)
		m.order.Remove(last)
		if old := m.entries[u]; old != nil && old.player != nil {
			old.player.Close()
		}
		delete(m.entries, u)
	}
}

// Len is the number of animations held.
func (m *mediaCache) Len() int { return m.order.Len() }

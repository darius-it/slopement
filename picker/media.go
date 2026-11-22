package picker

import (
	"container/list"
	"time"

	"github.com/egoist/mygo/ui"
)

// mediaCache keeps the newest animations by URL, closing the oldest
// beyond its limit. It is used on the UI thread only.
type mediaCache struct {
	max     int
	order   *list.List // of string URLs, newest first
	entries map[string]*mediaEntry
	now     func() time.Time // a test's clock
}

type mediaEntry struct {
	player  *ui.VideoPlayer
	loading bool
	failed  bool
	gone    bool      // failed for good (forbidden, not found)
	tries   int       // failed loads in a row
	retryAt time.Time // when a failed load may start again
	el      *list.Element
}

// autoRetries is how many times a failed load starts again by itself; after
// that it waits for a tap on the tile.
const autoRetries = 3

// backoff is the wait before the nth failed load starts again: 2 s, 6 s, 18 s.
func backoff(tries int) time.Duration {
	d := 2 * time.Second
	for i := 1; i < tries; i++ {
		d *= 3
	}
	return d
}

func newMediaCache(max int) *mediaCache {
	return &mediaCache{max: max, order: list.New(), entries: map[string]*mediaEntry{}, now: time.Now}
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

// failed reports whether url's load failed and is not loading again.
func (m *mediaCache) failed(url string) bool {
	e, ok := m.entries[url]
	return ok && e.failed && !e.loading
}

// start reports whether url should load now: not loading or loaded
// already, and, when it failed, past its wait.
func (m *mediaCache) start(url string) bool {
	e, ok := m.entries[url]
	if !ok {
		m.entries[url] = &mediaEntry{loading: true}
		return true
	}
	if e.player != nil || e.loading {
		return false
	}
	if e.tries > autoRetries || m.now().Before(e.retryAt) {
		return false
	}
	e.loading = true
	return true
}

// fail records a failed load and says how long until it may start again
// by itself (0: it won't, it waits for a tap).
func (m *mediaCache) fail(url string) time.Duration {
	e, ok := m.entries[url]
	if !ok {
		return 0
	}
	e.loading, e.failed = false, true
	e.tries++
	if e.tries > autoRetries {
		return 0
	}
	wait := backoff(e.tries)
	e.retryAt = m.now().Add(wait)
	return wait
}

// markGone records a load that failed for good: it is not retried, by itself
// or by a tap.
func (m *mediaCache) markGone(url string) {
	if e, ok := m.entries[url]; ok {
		e.loading, e.failed, e.gone = false, true, true
		e.tries = autoRetries + 1
	}
}

// isGone reports whether url failed for good.
func (m *mediaCache) isGone(url string) bool {
	e, ok := m.entries[url]
	return ok && e.gone
}

// cancel forgets a load that never began, so it can start afresh.
func (m *mediaCache) cancel(url string) {
	if e, ok := m.entries[url]; ok && e.player == nil && e.loading {
		delete(m.entries, url)
	}
}

// retry lets a failed url load again now, as a tap on its tile asks.
func (m *mediaCache) retry(url string) bool {
	e, ok := m.entries[url]
	if !ok || !e.failed || e.loading || e.gone {
		return false
	}
	e.tries, e.retryAt = 0, time.Time{}
	return true
}

func (m *mediaCache) put(url string, p *ui.VideoPlayer) {
	e := m.entries[url]
	if e == nil {
		e = &mediaEntry{}
		m.entries[url] = e
	}
	e.player, e.loading, e.failed, e.gone, e.tries = p, false, false, false, 0
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

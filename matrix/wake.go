package matrix

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Coming back fast. A phone that was locked, or an app that was in the
// background, finds its sync dead: the system froze the request (or closed
// its socket), and mautrix's syncer then waits a fixed 10 seconds between
// tries, so the offline bar stayed long after the network was there again.
//
// Three things make it quick, none of them costing anything while all is well:
//   - Wake, called when the app comes to the front: the sync in flight is
//     dropped and a new one starts at once (the first asks the server for no
//     wait, so it answers straight away), and the server is asked whether it
//     can be reached, which clears the bar on its answer.
//   - A failed sync is tried again after 1, 2, 4, 8 seconds, then every 10,
//     rather than every 10 from the first.
//   - The connection watcher's answer, when the server is back while the sync
//     waits out its gap, starts the sync at once too.

// checkedTransport is the account client's transport. Over HTTP/2 every
// request shares one connection, and one that died quietly (the network
// changed, the computer slept) went on being used: cancelling a request does
// not close it, so each new one hung too, for minutes. A connection silent
// for a while is now pinged, and closed when the ping gets no answer.
func checkedTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.HTTP2 = &http.HTTP2Config{SendPingTimeout: pingAfter, PingTimeout: pingWait}
	return t
}

const (
	pingAfter = 10 * time.Second // silence before a ping
	pingWait  = 5 * time.Second  // for its answer
)

// syncRetry is the wait before trying a failed sync again: the n-th failure
// in a row (from 1).
func syncRetry(n int) time.Duration {
	d := time.Second
	for i := 1; i < n && d < syncRetryLongest; i++ {
		d *= 2
	}
	return min(d, syncRetryLongest)
}

const syncRetryLongest = 10 * time.Second

// syncKick holds what restarts the sync.
type syncKick struct {
	mu     sync.Mutex
	cancel context.CancelFunc // the running sync's
	fails  int                // network failures in a row
	wake   chan struct{}      // tells the connection watcher to ask now
	lastOK time.Time          // when a sync last answered
}

// syncFresh is how recently a sync must have answered for Wake to leave it
// alone: a healthy sync answers at least every 30 seconds (its long poll), so
// an older answer means it is stuck or was frozen.
const syncFresh = 35 * time.Second

// syncContext is the context for one run of the sync loop: Wake ends it.
func (s *Source) syncContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	s.kick.mu.Lock()
	s.kick.cancel = cancel
	s.kick.mu.Unlock()
	return ctx, cancel
}

// restartSync drops the sync in flight, or its wait, and starts a new one.
func (s *Source) restartSync() {
	s.kick.mu.Lock()
	cancel := s.kick.cancel
	s.kick.fails = 0
	s.kick.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// noteSyncFailure counts a network failure and says how long to wait.
func (s *Source) noteSyncFailure() time.Duration {
	s.kick.mu.Lock()
	defer s.kick.mu.Unlock()
	s.kick.fails++
	return syncRetry(s.kick.fails)
}

// noteSyncOK resets the count: the sync answered.
func (s *Source) noteSyncOK() {
	s.kick.mu.Lock()
	s.kick.fails, s.kick.lastOK = 0, time.Now()
	s.kick.mu.Unlock()
}

// syncHealthy says the sync answered lately and is not failing.
func (s *Source) syncHealthy(now time.Time) bool {
	s.kick.mu.Lock()
	defer s.kick.mu.Unlock()
	return s.kick.fails == 0 && !s.kick.lastOK.IsZero() && now.Sub(s.kick.lastOK) < syncFresh
}

// syncFailing says the last sync failed in the network.
func (s *Source) syncFailing() bool {
	s.kick.mu.Lock()
	defer s.kick.mu.Unlock()
	return s.kick.fails > 0
}

// wakeChan is the watcher's: Wake sends on it.
func (s *Source) wakeChan() chan struct{} {
	s.kick.mu.Lock()
	defer s.kick.mu.Unlock()
	if s.kick.wake == nil {
		s.kick.wake = make(chan struct{}, 1)
	}
	return s.kick.wake
}

// Wake is called when the app comes back to the front (unlocked, or switched
// to): the sync starts again at once rather than finishing a request the
// system may have frozen, and the server is asked whether it can be reached.
// A sync that answered lately is left alone: on a computer the app becomes
// active at every switch to it, and that costs nothing. Any goroutine.
func (s *Source) Wake() {
	if !s.started || s.syncHealthy(time.Now()) {
		return
	}
	s.restartSync()
	select {
	case s.wakeChan() <- struct{}{}:
	default: // one is waiting already
	}
}

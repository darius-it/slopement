package matrix

import (
	"context"
	"time"
)

// The connection watcher. A sync waiting on a dead connection can hang for
// minutes, and a failed one is retried only every 10 seconds, so the sync alone
// says "offline" late and "online" later.
//
// No polling: the operating system says when the network changes (netwatch_*.go:
// a kernel socket the watcher sleeps on, costing nothing while nothing changes),
// and only then is the server asked one tiny question (the supported versions:
// no account, nothing sent). While the answer is "unreachable" it is asked again
// with growing gaps until the server answers: that is the only timer, and it
// runs only while offline. Where the system gives no such event, the sync's own
// failures still raise the bar, as before.
const (
	netSettle    = time.Second      // events come in bursts: one question per burst
	retryFirst   = 2 * time.Second  // first gap while offline
	retryLongest = 30 * time.Second // gaps double up to this
	probeTimeout = 5 * time.Second
)

// probe asks the server whether it can be reached.
func (s *Source) probe(ctx context.Context) error {
	if s.probeFn != nil {
		return s.probeFn(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	_, err := s.cli.Versions(ctx)
	return err
}

// watchConnection runs until ctx ends.
func (s *Source) watchConnection(ctx context.Context) {
	changes := s.netChanges
	if changes == nil {
		changes = netChanges
	}
	events := changes(ctx) // nil where the system has none: the select never takes it
	settle, first := netSettle, retryFirst
	if s.watchTiming != nil {
		settle, first = s.watchTiming[0], s.watchTiming[1]
	}

	var settling, retrying <-chan time.Time
	gap := first
	check := func() {
		err := s.probe(ctx)
		if ctx.Err() != nil {
			return
		}
		s.post(func() { s.noteConnection(err) })
		if isNetworkError(err) {
			retrying = time.After(gap)
			if gap *= 2; gap > retryLongest {
				gap = retryLongest
			}
			return
		}
		retrying, gap = nil, first
		// The server is back while the sync waits out its gap: it goes now
		// (wake.go), so the messages come with the bar going.
		if s.syncFailing() {
			s.restartSync()
		}
	}
	wake := s.wakeChan()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if settling == nil {
				settling = time.After(settle)
			}
		case <-settling:
			settling = nil
			check()
		case <-wake: // the app came back to the front: ask now, no settling
			settling, retrying, gap = nil, nil, first
			check()
		case <-retrying:
			retrying = nil
			check()
		}
	}
}

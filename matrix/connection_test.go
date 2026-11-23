package matrix

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func downErr() error { return &net.OpError{Op: "dial", Err: errors.New("network is unreachable")} }

// The watcher asks the server nothing until the system says the network changed;
// then one question per burst of events, and while the answer is "unreachable",
// again with growing gaps until it is reachable.
func TestWatchConnectionIsEventDriven(t *testing.T) {
	s, _ := testSynced()
	var ui sync.Mutex // stands for the UI thread: updates run, and the test reads, under it
	s.update = func(fn func()) { ui.Lock(); defer ui.Unlock(); fn() }
	var down atomic.Bool
	var asked atomic.Int32
	s.probeFn = func(context.Context) error {
		asked.Add(1)
		if down.Load() {
			return downErr()
		}
		return nil
	}
	events := make(chan struct{}, 8)
	s.netChanges = func(context.Context) <-chan struct{} { return events }
	s.watchTiming = &[2]time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.watchConnection(ctx)

	offline := func() bool { ui.Lock(); defer ui.Unlock(); return s.Offline() }
	waitFor := func(want bool) {
		t.Helper()
		for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(2 * time.Millisecond) {
			if offline() == want {
				return
			}
		}
		t.Fatalf("Offline() is %v, want %v", offline(), want)
	}

	// Nothing happens, nothing is asked: no polling.
	time.Sleep(150 * time.Millisecond)
	if asked.Load() != 0 {
		t.Fatalf("asked %d times with no network event", asked.Load())
	}

	// The network goes: a burst of events is one question, and the bar is up.
	down.Store(true)
	for i := 0; i < 5; i++ {
		events <- struct{}{}
	}
	waitFor(true)
	// It stays unreachable: asked again by itself, until it is back.
	before := asked.Load()
	time.Sleep(150 * time.Millisecond)
	if asked.Load() <= before {
		t.Fatal("not asked again while offline")
	}
	down.Store(false)
	waitFor(false) // found by the retry alone, with no further event

	// Online and quiet again: no more questions.
	time.Sleep(60 * time.Millisecond)
	settled := asked.Load()
	time.Sleep(150 * time.Millisecond)
	if asked.Load() != settled {
		t.Fatal("still asking while online and nothing changed")
	}
}

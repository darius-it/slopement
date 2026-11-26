package matrix

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A failed sync is tried again soon, then less often: 1, 2, 4, 8, then 10 s.
func TestSyncRetryBacksOff(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 10 * time.Second, 10 * time.Second}
	for i, w := range want {
		if got := syncRetry(i + 1); got != w {
			t.Errorf("failure %d: %v, want %v", i+1, got, w)
		}
	}
	s, _ := testSynced()
	s.noteSyncFailure()
	if d := s.noteSyncFailure(); d != 2*time.Second {
		t.Fatalf("second failure waits %v", d)
	}
	s.noteSyncOK()
	if d := s.noteSyncFailure(); d != time.Second {
		t.Fatalf("after an answer the count did not start over: %v", d)
	}
}

// Wake ends the sync run (so the loop starts a new one at once) and has the
// server asked, when the sync is stuck or failing; a sync that answered lately
// is left alone, so a computer switching to the app costs nothing.
func TestWakeRestartsAStuckSync(t *testing.T) {
	s, _ := testSynced()
	var ui sync.Mutex
	s.update = func(fn func()) { ui.Lock(); defer ui.Unlock(); fn() }
	var asked atomic.Int32
	s.probeFn = func(context.Context) error { asked.Add(1); return nil }
	s.netChanges = func(context.Context) <-chan struct{} { return nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.watchConnection(ctx)
	s.started = true

	// Healthy: nothing.
	s.noteSyncOK()
	run, stop := s.syncContext(ctx)
	defer stop()
	s.Wake()
	time.Sleep(50 * time.Millisecond)
	if run.Err() != nil || asked.Load() != 0 {
		t.Fatalf("a healthy sync was woken (ended %v, asked %d)", run.Err() != nil, asked.Load())
	}

	// Stuck (the last answer is old): the run ends, the server is asked.
	s.kick.mu.Lock()
	s.kick.lastOK = time.Now().Add(-time.Minute)
	s.kick.mu.Unlock()
	s.Wake()
	select {
	case <-run.Done():
	case <-time.After(time.Second):
		t.Fatal("the sync run did not end")
	}
	for end := time.Now().Add(time.Second); asked.Load() == 0 && time.Now().Before(end); time.Sleep(2 * time.Millisecond) {
	}
	if asked.Load() == 0 {
		t.Fatal("the server was not asked")
	}
}

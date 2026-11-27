package matrix

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"maunium.net/go/mautrix"
)

// The sync goes on after the filter upload fails (a connection not back yet),
// and a run started again carries on from the last answer's token without
// uploading the filter again.
func TestSyncLoopRecoversAndResumes(t *testing.T) {
	var (
		mu      sync.Mutex
		filters int
		since   []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost { // the filter
			if filters++; filters == 1 {
				panic(http.ErrAbortHandler) // the connection drops
			}
			fmt.Fprint(w, `{"filter_id":"1"}`)
			return
		}
		since = append(since, r.URL.Query().Get("since"))
		n := len(since)
		if n > 1 {
			time.Sleep(10 * time.Millisecond) // not a busy loop
		}
		fmt.Fprintf(w, `{"next_batch":"s%d"}`, n)
	}))
	defer srv.Close()

	cli, err := mautrix.NewClient(srv.URL, "@me:test", "token")
	if err != nil {
		t.Fatal(err)
	}
	cli.Store = &tokenStore{SyncStore: mautrix.NewMemorySyncStore(), token: "start"}
	s, _ := testSynced()
	s.cli = cli
	s.update = func(fn func()) {}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.syncLoop(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	asked := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), since...)
	}
	await := func(n int) {
		t.Helper()
		for end := time.Now().Add(5 * time.Second); len(asked()) < n; time.Sleep(5 * time.Millisecond) {
			if time.Now().After(end) {
				t.Fatalf("only %d syncs after the failed filter, want %d", len(asked()), n)
			}
		}
	}

	await(3)
	s.restartSync()
	before := len(asked())
	await(before + 2)

	got := asked()
	if got[0] != "start" {
		t.Errorf("first sync since %q", got[0])
	}
	for i := 1; i < len(got); i++ {
		if got[i] == "start" {
			t.Fatalf("a new run went back to the starting token: %v", got)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if filters != 2 {
		t.Errorf("filter uploaded %d times, want 2 (one failed)", filters)
	}
}

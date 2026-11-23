package matrix

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"
)

// A call that was never left shows as "in a call" for everyone: clearing sets
// the user's own call memberships to empty — and only theirs, and only those
// that are not empty already.
func TestClearCallMembershipEmptiesOnlyMyOwn(t *testing.T) {
	var mu sync.Mutex
	var puts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/state") {
			_, _ = w.Write([]byte(`[
			 {"type":"m.rtc.member","state_key":"_@me:x_D","sender":"@me:x","content":{"slot_id":"s"},"event_id":"$1","origin_server_ts":1},
			 {"type":"m.rtc.member","state_key":"_@me:x_OLD","sender":"@me:x","content":{},"event_id":"$2","origin_server_ts":2},
			 {"type":"m.rtc.member","state_key":"_@kai:x_E","sender":"@kai:x","content":{"slot_id":"s"},"event_id":"$3","origin_server_ts":3},
			 {"type":"org.matrix.msc3401.call.member","state_key":"@me:x","sender":"@me:x","content":{"m.calls":[]},"event_id":"$4","origin_server_ts":4}]`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		puts = append(puts, r.URL.Path+" "+string(body))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"event_id":"$new"}`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	s.me.ID = "@me:x"
	cli, _ := mautrix.NewClient(srv.URL, "@me:x", "token")
	s.cli = cli

	cli.DeviceID = "D"
	// After a call: only this device's own membership (key with its device ID).
	if n, err := s.ClearCallMembership(context.Background(), "!r:x", true); err != nil || n != 1 {
		t.Fatalf("this device only: cleared %d, %v", n, err)
	}
	puts = nil
	n, err := s.ClearCallMembership(context.Background(), "!r:x", false)
	if err != nil || n != 2 {
		t.Fatalf("cleared %d, %v", n, err)
	}
	joined := strings.Join(puts, "\n")
	if !strings.Contains(joined, "m.rtc.member/_@me:x_D {}") && !strings.Contains(joined, "m.rtc.member/_@me:x_D") {
		t.Fatalf("my live membership was not emptied: %s", joined)
	}
	if strings.Contains(joined, "kai") || strings.Contains(joined, "_OLD") {
		t.Fatalf("somebody else's, or an empty one, was touched: %s", joined)
	}
}

package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// A message that did not send can be sent again: the failed copy goes, and the
// message comes back as a new one that sends, here to a server that is back.
func TestRetrySendsAFailedMessageAgain(t *testing.T) {
	s, _ := testSynced()
	s.me.ID = "@me:x"
	down, _ := mautrix.NewClient("http://127.0.0.1:1", "@me:x", "t") // nothing listens
	s.cli = down
	r := s.roomOf("!r")

	s.Send("!r", "hello there", "")
	waitFor(t, "the failure", func() bool { return len(r.events) == 1 && r.events[0].Failed })
	failedID := r.events[0].ID
	waitFor(t, "how to send it again", func() bool { return s.CanRetry("!r", failedID) })
	if s.CanRetry("!r", "$not-mine") {
		t.Fatal("something unknown can be retried")
	}

	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/send/m.room.message/") {
			got.Store(req.URL.Path)
			_, _ = w.Write([]byte(`{"event_id":"$delivered"}`))
			return
		}
		http.NotFound(w, req)
	}))
	defer srv.Close()
	up, _ := mautrix.NewClient(srv.URL, "@me:x", "t")
	s.cli = up // the connection is back

	s.Retry("!r", failedID)
	if r.indexOf(failedID) >= 0 {
		t.Fatal("the failed copy is still there")
	}
	waitFor(t, "the new send to land", func() bool {
		return len(r.events) == 1 && r.events[0].ID == "$delivered" && !r.events[0].Pending && !r.events[0].Failed
	})
	if r.events[0].Body != "hello there" {
		t.Fatalf("what was sent again: %q", r.events[0].Body)
	}
	if got.Load() == nil {
		t.Fatal("the server was not asked")
	}
	if s.CanRetry("!r", failedID) {
		t.Fatal("it can be retried a second time after it went")
	}
	s.Retry("!r", failedID) // nothing to do, and nothing breaks
	if len(r.events) != 1 {
		t.Fatalf("a retry of nothing added %d events", len(r.events)-1)
	}
	var _ model.Retrier = s
}

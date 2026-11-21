package matrix

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// A fake homeserver that records each request's path and body, and can
// refuse them all.
type recordServer struct {
	mu     sync.Mutex
	reqs   []string
	bodies []string
	refuse bool
}

func (f *recordServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Method+" "+r.URL.Path)
	f.bodies = append(f.bodies, string(body))
	refuse := f.refuse
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if refuse {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errcode":"M_FORBIDDEN","error":"You do not have permission"}`))
		return
	}
	_, _ = w.Write([]byte(`{"event_id":"$ok"}`))
}

func (f *recordServer) find(substr string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.reqs {
		if strings.Contains(r, substr) {
			return f.bodies[i], true
		}
	}
	return "", false
}

func actionSource(t *testing.T, fake *recordServer) (*Source, *room, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	r.appendEvent(model.Event{ID: "$mine", Sender: "@me:example.org", Kind: model.EventText, Body: "tyop", Time: at(1)}, "You", true)
	r.appendEvent(model.Event{ID: "$other", Sender: "@kai:example.org", Kind: model.EventText, Body: "hello", Time: at(2)}, "kai", false)
	return s, r, srv.Close
}

// Edit shows at once and goes as a replacement of the message; Delete takes
// the message out at once; Pin writes the room's pinned list.
func TestMessageActions(t *testing.T) {
	fake := &recordServer{}
	s, r, done := actionSource(t, fake)
	defer done()

	s.Edit("!a", "$mine", "typo")
	if r.events[0].Body != "typo" || !r.events[0].Edited {
		t.Fatalf("not edited at once: %+v", r.events[0])
	}
	waitFor(t, "the edit to be sent", func() bool { _, ok := fake.find("/send/m.room.message/"); return ok })
	body, _ := fake.find("/send/m.room.message/")
	var edit map[string]any
	_ = json.Unmarshal([]byte(body), &edit)
	rel, _ := edit["m.relates_to"].(map[string]any)
	newc, _ := edit["m.new_content"].(map[string]any)
	if edit["body"] != "* typo" || rel["rel_type"] != "m.replace" || rel["event_id"] != "$mine" || newc["body"] != "typo" {
		t.Fatalf("edit event = %s", body)
	}

	s.Pin("!a", "$other", true)
	if !s.IsPinned("!a", "$other") || len(s.Pinned("!a")) != 1 {
		t.Fatal("not pinned at once")
	}
	waitFor(t, "the pin", func() bool { _, ok := fake.find("m.room.pinned_events"); return ok })
	pin, _ := fake.find("m.room.pinned_events")
	if !strings.Contains(pin, `"$other"`) {
		t.Fatalf("pinned content = %s", pin)
	}
	s.Pin("!a", "$other", false)
	if s.IsPinned("!a", "$other") {
		t.Fatal("still pinned")
	}

	s.Redact("!a", "$other")
	if r.indexOf("$other") >= 0 {
		t.Fatal("the deleted message is still shown")
	}
	waitFor(t, "the redaction", func() bool { _, ok := fake.find("/redact/"); return ok })
}

// A refused action says so in the room, quietly, in a line that is never
// stored.
func TestActionFailureIsSaidInTheRoom(t *testing.T) {
	fake := &recordServer{refuse: true}
	s, r, done := actionSource(t, fake)
	defer done()

	s.Pin("!a", "$other", true)
	waitFor(t, "the notice", func() bool {
		for _, e := range r.events {
			if e.Kind == model.EventSystem && strings.Contains(e.Body, "Could not change the pins") {
				return true
			}
		}
		return false
	})
	for _, e := range r.record().Events {
		if e.Kind == model.EventSystem {
			t.Fatal("a notice was stored")
		}
	}
}

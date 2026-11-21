package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func TestParseStatus(t *testing.T) {
	for msg, want := range map[string]model.Status{
		"":                      {},
		"   ":                   {},
		"🎧 Focusing on the kit": {Emoji: "🎧", Text: "Focusing on the kit"},
		"🎧":                     {Emoji: "🎧"},
		"👩🏽‍💻 coding":             {Emoji: "👩🏽‍💻", Text: "coding"},
		"🇩🇪 unterwegs":          {Emoji: "🇩🇪", Text: "unterwegs"},
		"Out for lunch":         {Text: "Out for lunch"},
		"5 min away":            {Text: "5 min away"},
		"wfh 🏠":                 {Text: "wfh 🏠"},
		"  ✈️  flying  ":        {Emoji: "✈️", Text: "flying"},
	} {
		if got := parseStatus(msg); got != want {
			t.Errorf("parseStatus(%q) = %+v, want %+v", msg, got, want)
		}
	}
	if got := statusMessage(model.Status{Emoji: "🎧", Text: "Focusing"}); got != "🎧 Focusing" {
		t.Errorf("statusMessage = %q", got)
	}
}

func presenceEvent(who string, p event.Presence, msg string) *event.Event {
	return &event.Event{Sender: id.UserID(who), Type: event.EphemeralEventPresence,
		Content: event.Content{Parsed: &event.PresenceEventContent{Presence: p, StatusMessage: msg, Displayname: "Kai", LastActiveAgo: 5000}}}
}

// A presence event's status message is the person's status; the next one
// without a message clears it; the user's own lands on the user.
func TestPresenceEventsCarryStatus(t *testing.T) {
	s, _ := testSynced()
	s.onPresence(nil, presenceEvent("@kai:example.org", event.PresenceOnline, "🎧 Focusing on the kit"))
	p, _ := s.Person("@kai:example.org")
	if p.Presence != model.Online || p.Status.Emoji != "🎧" || p.Status.Text != "Focusing on the kit" {
		t.Fatalf("person = %+v", p)
	}
	if p.LastSeen.IsZero() {
		t.Fatal("last active was dropped")
	}
	s.onPresence(nil, presenceEvent("@kai:example.org", event.PresenceUnavailable, ""))
	if p, _ := s.Person("@kai:example.org"); p.Presence != model.Away || p.Status.Set(time.Now()) {
		t.Fatalf("after: %+v", p)
	}
	s.onPresence(nil, presenceEvent("@me:example.org", event.PresenceOnline, "🌴 on holiday"))
	if me := s.Me(); me.Status.Emoji != "🌴" || me.Status.Text != "on holiday" {
		t.Fatalf("me = %+v", me)
	}
}

// A resumed sync sends no presence for what did not change, so the status of
// each DM's other person (and the user) is asked for at the start. A server
// with presence off says no, and the rest is not asked.
func TestFetchPresenceAtStart(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "kai") {
			_, _ = w.Write([]byte(`{"presence":"online","last_active_ago":1000,"status_msg":"📚 studying"}`))
			return
		}
		_, _ = w.Write([]byte(`{"presence":"offline","status_msg":""}`))
	}))
	defer srv.Close()

	s, _ := testSynced()
	s.cli, _ = mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.rooms["!dm"] = &room{id: "!dm", dm: true, partner: "@kai:example.org"}
	s.people["@kai:example.org"] = &model.Person{ID: "@kai:example.org", Name: "Kai"}
	s.fetchPresence(t.Context())
	waitFor(t, "kai's status", func() bool { p, _ := s.Person("@kai:example.org"); return p.Status.Text == "studying" })
	if p, _ := s.Person("@kai:example.org"); p.Presence != model.Online || p.Status.Emoji != "📚" || p.Name != "Kai" {
		t.Fatalf("kai = %+v", p)
	}
	mu.Lock()
	n := len(asked)
	mu.Unlock()
	if n != 2 {
		t.Fatalf("asked %d times (%v), want the DM's person and the user", n, asked)
	}

	// Presence off on the server: one refusal ends it.
	off := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, "off")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errcode":"M_FORBIDDEN","error":"Presence is disabled on this server"}`))
	}))
	defer off.Close()
	mu.Lock()
	asked = nil
	mu.Unlock()
	s2, _ := testSynced()
	s2.cli, _ = mautrix.NewClient(off.URL, "@me:example.org", "token")
	s2.rooms["!dm"] = &room{id: "!dm", dm: true, partner: "@kai:example.org"}
	s2.rooms["!dm2"] = &room{id: "!dm2", dm: true, partner: "@sam:example.org"}
	s2.fetchPresence(t.Context())
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 1 {
		t.Fatalf("asked %d times with presence off, want 1", len(asked))
	}
}

// The user's own status shows at once and goes out as the presence's status
// message; one with an end time clears itself.
func TestSetStatus(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 512)
		n, _ := r.Body.Read(b)
		mu.Lock()
		bodies = append(bodies, r.Method+" "+r.URL.Path+" "+string(b[:n]))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	s.cli, _ = mautrix.NewClient(srv.URL, "@me:example.org", "token")

	s.SetStatus(model.Status{Emoji: "🎧", Text: "Focusing"})
	if me := s.Me(); me.Status.Emoji != "🎧" {
		t.Fatalf("not shown at once: %+v", me.Status)
	}
	waitFor(t, "the status to go out", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(bodies) == 1 && strings.Contains(bodies[0], "🎧 Focusing") && strings.HasPrefix(bodies[0], "PUT ") && strings.Contains(bodies[0], "/presence/")
	})

	s.SetStatus(model.Status{Emoji: "☕", Text: "break", Until: time.Now().Add(150 * time.Millisecond)})
	waitFor(t, "the status to end", func() bool { return !s.Me().Status.Set(time.Now()) })
	waitFor(t, "the clearing to go out", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(bodies) >= 3 && !strings.Contains(bodies[len(bodies)-1], "break")
	})
}

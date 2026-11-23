package matrix

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// Nicknames and notes are one account data event: asked for at the start (a
// resumed sync does not resend it), and written whole when one changes.
func TestPersonNotes(t *testing.T) {
	var mu sync.Mutex
	var written string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/account_data/com.slopement.person_notes"):
			_, _ = w.Write([]byte(`{"@kai:example.org":{"nick":"Kaio","note":"from the climbing gym"}}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/account_data/com.slopement.person_notes"):
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			written = string(b)
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND"}`))
		}
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.fetchAccountData(ctx)
	if n := s.PersonNotes()["@kai:example.org"]; n.Nick != "Kaio" || n.Note != "from the climbing gym" {
		t.Fatalf("not back after a restart: %+v", n)
	}

	old := s.PersonNotes()
	s.SetPersonNote("@ada:example.org", model.PersonNote{Nick: "  Ada L  ", Note: ""})
	if got := s.PersonNotes()["@ada:example.org"]; got.Nick != "Ada L" {
		t.Fatalf("not shown at once, trimmed: %+v", got)
	}
	if _, ok := old["@ada:example.org"]; ok {
		t.Fatal("the map the app holds was changed in place")
	}
	waitFor(t, "the notes to be saved", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(written, "Ada L") && strings.Contains(written, "Kaio")
	})

	// Both empty takes them away.
	s.SetPersonNote("@kai:example.org", model.PersonNote{})
	if _, ok := s.PersonNotes()["@kai:example.org"]; ok {
		t.Fatal("an emptied note stayed")
	}
}

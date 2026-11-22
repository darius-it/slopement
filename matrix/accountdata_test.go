package matrix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix"
)

// After a restart the sync resumes and does not resend account data set long
// ago: the colors chosen for names, the rail's order, the status. They are
// asked for at the start.
func TestAccountDataIsFetchedAtStart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/account_data/com.slopement.user_colors"):
			_, _ = w.Write([]byte(`{"@kai:example.org":3,"@ada:example.org":5}`))
		case strings.HasSuffix(r.URL.Path, "/account_data/com.slopement.rail_order"):
			_, _ = w.Write([]byte(`{"order":["!b","!a"]}`))
		case strings.HasSuffix(r.URL.Path, "/account_data/com.slopement.status"):
			_, _ = w.Write([]byte(`{"emoji":"🎧","text":"Focusing"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"Account data not found"}`))
		}
	}))
	defer srv.Close()

	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.fetchAccountData(ctx)

	if got := s.UserColors(); got["@kai:example.org"] != 3 || got["@ada:example.org"] != 5 {
		t.Fatalf("the colors chosen are not back: %v", got)
	}
	if len(s.railOrder) != 2 || s.railOrder[0] != "!b" {
		t.Fatalf("the rail's order is not back: %v", s.railOrder)
	}
	if s.savedStatus.Text != "Focusing" {
		t.Fatalf("the saved status is not back: %+v", s.savedStatus)
	}
}

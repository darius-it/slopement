package matrix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// limitServer answers the media config where the test says: the authenticated
// v1 endpoint, the older v3 one, or neither.
func limitServer(v1, v3 string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/media/config") && v1 != "":
			_, _ = w.Write([]byte(v1))
		case strings.HasSuffix(r.URL.Path, "/v3/config") && v3 != "":
			_, _ = w.Write([]byte(v3))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"errcode":"M_UNRECOGNIZED","error":"no"}`))
		}
	}))
}

func limitOf(t *testing.T, v1, v3 string) int64 {
	t.Helper()
	srv := limitServer(v1, v3)
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.fetchUploadLimit(context.Background())
	return s.UploadLimit()
}

// The server's limit is read from the authenticated endpoint, else the older
// one, else stays unknown; what the app sends never exceeds its own 50 MB.
func TestUploadLimitFromTheServer(t *testing.T) {
	if got := limitOf(t, `{"m.upload.size": 10485760}`, ""); got != 10<<20 {
		t.Fatalf("v1: %d", got)
	}
	if got := limitOf(t, "", `{"m.upload.size": 26214400}`); got != 25<<20 {
		t.Fatalf("v3 fallback: %d", got)
	}
	if got := limitOf(t, "", ""); got != 0 {
		t.Fatalf("no endpoint: %d", got)
	}
	if got := limitOf(t, `{}`, ""); got != 0 {
		t.Fatalf("a server that names no limit: %d", got)
	}
}

func TestSendLimitAndWording(t *testing.T) {
	s, _ := testSynced()
	if s.sendLimit() != maxUpload {
		t.Fatal("unknown limit is not the app's")
	}
	s.uploadLimit = 10 << 20
	if s.sendLimit() != 10<<20 {
		t.Fatal("the server's smaller limit is not used")
	}
	s.uploadLimit = 500 << 20
	if s.sendLimit() != maxUpload {
		t.Fatal("a larger server limit raised the app's")
	}
	if tooLargeText(10<<20) != "Too large to send (over 10 MB)" || tooLargeText(1<<20+512<<10) != "Too large to send (over 1.5 MB)" {
		t.Fatalf("%q %q", tooLargeText(10<<20), tooLargeText(1<<20+512<<10))
	}
}

// A file over the server's limit that slipped past the check (the limit was not
// known yet) is marked failed, and the reason says the server's limit.
func TestSendFileOverTheServerLimitFails(t *testing.T) {
	fake := &sendServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.uploadLimit = 4
	r := s.roomOf("!a")
	s.SendFile("!a", model.File{Name: "big.bin", Data: []byte("12345678")}, "")
	waitFor(t, "the failure", func() bool { return len(r.events) == 1 && r.events[0].Failed })
	if !strings.Contains(r.events[0].Error, "Too large to send") || fake.uploaded != nil {
		t.Fatalf("error %q, uploaded %d bytes", r.events[0].Error, len(fake.uploaded))
	}
}

// A proxy refusing with a bare 413 (no Matrix error) reads as "too large".
func TestBare413IsTooLarge(t *testing.T) {
	err := mautrix.HTTPError{Response: &http.Response{StatusCode: 413}}
	if got := sendReason(err); got != "It is too large for the server." {
		t.Fatalf("%q", got)
	}
}

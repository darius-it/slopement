package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
)

// writeJSON answers w with v as JSON.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// fakePasswordServer is a homeserver that takes password logins.
func fakePasswordServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/_matrix/client/v3/login" && r.Method == http.MethodGet:
			writeJSON(w, map[string]any{"flows": []map[string]string{{"type": "m.login.password"}}})
		case r.URL.Path == "/_matrix/client/v3/login" && r.Method == http.MethodPost:
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			ident, _ := req["identifier"].(map[string]any)
			if req["type"] != "m.login.password" || req["password"] != "hunter2" || ident["user"] != "ada" {
				w.WriteHeader(http.StatusForbidden)
				writeJSON(w, map[string]any{"errcode": "M_FORBIDDEN", "error": "no"})
				return
			}
			writeJSON(w, map[string]any{
				"user_id": "@ada:example.org", "device_id": "PWDEV", "access_token": "pw-token",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbePassword(t *testing.T) {
	openTest(t)
	srv := fakePasswordServer(t)

	s, err := Probe(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != KindPassword || s.URL != srv.URL {
		t.Fatalf("server = %+v", s)
	}
}

func TestProbeUnreachable(t *testing.T) {
	openTest(t)
	// A port nothing listens on.
	if _, err := Probe(context.Background(), "http://127.0.0.1:9"); err == nil {
		t.Fatal("an unreachable server was probed successfully")
	}
}

func TestLoginPassword(t *testing.T) {
	openTest(t)
	srv := fakePasswordServer(t)
	ctx := context.Background()

	s, err := LoginPassword(ctx, mustProbe(t, ctx, srv.URL), "ada", "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != "@ada:example.org" || s.DeviceID != "PWDEV" {
		t.Fatalf("session = %+v", s)
	}
	// The token went to the keychain, the record to the account's
	// directory, and Sessions lists it.
	if tok, err := TokensOf(string(s.UserID), string(s.DeviceID)); err != nil || tok.Access != "pw-token" {
		t.Fatalf("tokens = %+v, %v", tok, err)
	}
	logins, err := Sessions()
	if err != nil || len(logins) != 1 || logins[0].UserID != s.UserID {
		t.Fatalf("sessions = %+v, %v", logins, err)
	}
}

// fakeOAuthServer is a homeserver behind MAS: auth metadata, dynamic
// client registration, the token endpoint, and whoami.
func fakeOAuthServer(t *testing.T, registered *atomic.Int32) *httptest.Server {
	t.Helper()
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_matrix/client/v3/login":
			writeJSON(w, map[string]any{"flows": []map[string]string{}})
		case "/_matrix/client/v1/auth_metadata":
			writeJSON(w, map[string]any{
				"issuer":                           srv.URL + "/",
				"authorization_endpoint":           srv.URL + "/authorize",
				"registration_endpoint":            srv.URL + "/register",
				"token_endpoint":                   srv.URL + "/token",
				"response_types_supported":         []string{"code"},
				"response_modes_supported":         []string{"query"},
				"grant_types_supported":            []string{"authorization_code", "refresh_token"},
				"code_challenge_methods_supported": []string{"S256"},
			})
		case "/register":
			if r.Method != http.MethodPost {
				http.NotFound(w, r)
				return
			}
			registered.Add(1)
			writeJSON(w, map[string]any{"client_id": "MASCLIENT"})
		case "/token":
			if r.Method != http.MethodPost {
				http.NotFound(w, r)
				return
			}
			r.ParseForm()
			if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "abc" ||
				r.Form.Get("code_verifier") == "" || r.Form.Get("redirect_uri") == "" {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]any{"error": "invalid_request"})
				return
			}
			writeJSON(w, map[string]any{
				"access_token": "oidc-token", "token_type": "Bearer",
				"expires_in": 3600, "refresh_token": "oidc-refresh",
			})
		case "/_matrix/client/v3/account/whoami":
			writeJSON(w, map[string]any{"user_id": "@mas:example.org", "device_id": "MASDEV"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mustProbe(t *testing.T, ctx context.Context, input string) *Server {
	t.Helper()
	s, err := Probe(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLoginOIDC(t *testing.T) {
	openTest(t)
	ctx := context.Background()
	var registered atomic.Int32
	srv := fakeOAuthServer(t, &registered)

	// The browser: take the authorization URL, come back to its redirect
	// with the code the server would grant.
	open := func(authURL string) error {
		go func() {
			u, err := url.Parse(authURL)
			if err != nil {
				return
			}
			q := u.Query()
			if q.Get("client_id") != "MASCLIENT" || q.Get("code_challenge") == "" {
				return
			}
			http.Get(q.Get("redirect_uri") + "?code=abc&state=" + url.QueryEscape(q.Get("state")))
		}()
		return nil
	}

	s, err := LoginOIDC(ctx, mustProbe(t, ctx, srv.URL), open)
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != "@mas:example.org" || s.DeviceID != "MASDEV" {
		t.Fatalf("session = %+v", s)
	}
	if s.ServerURL != srv.URL {
		t.Fatalf("server url = %q", s.ServerURL)
	}
	tok, err := TokensOf(string(s.UserID), string(s.DeviceID))
	if err != nil || tok.Access != "oidc-token" || tok.Refresh != "oidc-refresh" {
		t.Fatalf("tokens = %+v, %v", tok, err)
	}
	if tok.Expires.IsZero() {
		t.Error("the token's expiry was not saved")
	}
	if registered.Load() != 1 {
		t.Fatalf("registered %d times, want 1", registered.Load())
	}

	// The registration is cached: a second login on the same loopback
	// port reuses it.
	s2, err := LoginOIDC(ctx, mustProbe(t, ctx, srv.URL), open)
	if err != nil {
		t.Fatal(err)
	}
	if s2.UserID != "@mas:example.org" || registered.Load() != 1 {
		t.Fatalf("second login: %v, registered %d times", s2, registered.Load())
	}
	logins, err := Sessions()
	if err != nil || len(logins) != 1 || logins[0].UserID != "@mas:example.org" {
		t.Fatalf("sessions = %+v, %v", logins, err)
	}
}

// TestLoginOIDCWrongState checks the callback's CSRF check: an answer
// from another window is refused.
func TestLoginOIDCWrongState(t *testing.T) {
	openTest(t)
	ctx := context.Background()
	var registered atomic.Int32
	srv := fakeOAuthServer(t, &registered)

	open := func(authURL string) error {
		go func() {
			u, _ := url.Parse(authURL)
			// Not the state we sent.
			http.Get(u.Query().Get("redirect_uri") + "?code=abc&state=nope")
		}()
		return nil
	}
	if _, err := LoginOIDC(ctx, mustProbe(t, ctx, srv.URL), open); err == nil {
		t.Fatal("a callback with the wrong state was accepted")
	}
}

// TestDial restores a session's client: its token is set, and an OIDC
// session's refresh machinery knows the client ID.
func TestDial(t *testing.T) {
	openTest(t)
	ctx := context.Background()
	var registered atomic.Int32
	srv := fakeOAuthServer(t, &registered)

	open := func(authURL string) error {
		go func() {
			u, _ := url.Parse(authURL)
			http.Get(u.Query().Get("redirect_uri") + "?code=abc&state=" + url.QueryEscape(u.Query().Get("state")))
		}()
		return nil
	}
	s, err := LoginOIDC(ctx, mustProbe(t, ctx, srv.URL), open)
	if err != nil {
		t.Fatal(err)
	}

	cli, err := Dial(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if cli.AccessToken != "oidc-token" || cli.UserID != s.UserID {
		t.Fatalf("client token or user wrong: %q %q", cli.AccessToken, cli.UserID)
	}
	if cli.DeviceID != s.DeviceID {
		t.Fatalf("device = %q, want %q (the crypto helper needs it)", cli.DeviceID, s.DeviceID)
	}

	// Without a token, signing in again is asked for — the stores are
	// never wiped by Dial failing.
	if err := DeleteTokens(string(s.UserID), string(s.DeviceID)); err != nil {
		t.Fatal(err)
	}
	if _, err := Dial(ctx, s); !errors.Is(err, ErrNoToken) {
		t.Fatalf("dial without tokens: %v, want ErrNoToken", err)
	}
}

// TestLiveProbe asks the real matrix.org what login it takes; LIVE=1 to
// run. The requirements expect MAS there.
func TestLiveProbe(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE not set")
	}
	s, err := Probe(context.Background(), "matrix.org")
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != KindOIDC {
		t.Fatalf("matrix.org takes login kind %d, want OIDC", s.Kind)
	}
	t.Logf("matrix.org: %s (login kind %d)", s.URL, s.Kind)
}

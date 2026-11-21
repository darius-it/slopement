package login

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/matrix"
)

// fakePasswordServer is a homeserver that takes password logins, with
// ada/hunter2 the one that works.
func fakePasswordServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/_matrix/client/v3/login" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"flows": []map[string]string{{"type": "m.login.password"}}})
		case r.URL.Path == "/_matrix/client/v3/login" && r.Method == http.MethodPost:
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			ident, _ := req["identifier"].(map[string]any)
			if req["password"] != "hunter2" || ident["user"] != "ada" {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]any{"errcode": "M_FORBIDDEN", "error": "no"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"user_id": "@ada:example.org", "device_id": "PWDEV", "access_token": "pw-token",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSignInPassword drives the whole screen: type a server, probe it,
// fill the form, sign in; a wrong password shows one quiet line, the
// right one hands the session over.
func TestSignInPassword(t *testing.T) {
	srv := fakePasswordServer(t)
	l := New()
	posted := make(chan func(), 16)
	l.SetUpdate(func(f func()) { posted <- f })
	signed := make(chan *matrix.Session, 1)
	l.OnSignedIn = func(s *matrix.Session) { signed <- s }

	tt := ui.NewTester(l.View, 800, 600)
	tt.Frame()

	// Type the server and probe it.
	focusField(t, tt, "Other server")
	tt.Type(srv.URL)
	tt.Key(0, ui.KeyEnter)
	drain(t, posted, 5*time.Second)
	tt.Frame()
	if !tt.HasText("Username") || !tt.HasText("Password") {
		t.Fatal("the password step did not come up after the probe")
	}

	// A wrong password is one quiet line, and the form stays.
	focusField(t, tt, "Username")
	tt.Type("ada")
	focusField(t, tt, "Password")
	tt.Type("nope")
	tt.Key(0, ui.KeyEnter)
	drain(t, posted, 5*time.Second)
	tt.Frame()
	if !tt.HasText("Wrong username or password.") {
		t.Fatal("the wrong password has no quiet line")
	}

	// The right one signs in.
	l.st.Pass = ""
	focusField(t, tt, "Password")
	tt.Type("hunter2")
	tt.Key(0, ui.KeyEnter)
	drain(t, posted, 5*time.Second)
	tt.Frame()
	select {
	case s := <-signed:
		if s.UserID != "@ada:example.org" || s.DeviceID != "PWDEV" {
			t.Fatalf("session = %+v", s)
		}
	default:
		t.Fatal("the session never came back")
	}
}

// focusField clicks the text input named label.
func focusField(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleTextField && a.Name == label {
			tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
			return
		}
	}
	t.Fatalf("no text input named %q", label)
}

// drain applies what the background work posted, until a moment passes
// with nothing new.
func drain(t *testing.T, posted chan func(), within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case f := <-posted:
			f()
		case <-time.After(50 * time.Millisecond):
			return
		case <-deadline:
			t.Fatal("the background work never came back")
		}
	}
}

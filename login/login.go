// Package login is the sign-in screen over the Matrix backend: probe a
// server, sign in — the browser flow (MAS, first-class) or a password —
// and hand the session to the app. Every step's work runs off the UI
// thread and comes back through the window's update function, as the
// repo's rules say.
package login

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"maunium.net/go/mautrix"

	"slopement/kit"
	"slopement/matrix"
)

// Login drives the login screen: kit.Login for the view, package matrix
// for the work.
type Login struct {
	st       kit.LoginState
	sessions []*matrix.Session
	probed   *matrix.Server
	services ui.Services
	update   func(func())

	// OnSignedIn runs on the UI thread with the fresh session; the app
	// takes over from there.
	OnSignedIn func(*matrix.Session)

	onCancel func() // adding an account: back to the app
}

// New returns the login screen with the sessions recorded on this
// device, offered first.
func New() *Login {
	l := &Login{}
	if sessions, err := matrix.Sessions(); err == nil {
		l.sessions = sessions
		for _, s := range sessions {
			l.st.Saved = append(l.st.Saved, kit.SavedLogin{User: string(s.UserID), Server: s.ServerName})
		}
	}
	return l
}

// SetUpdate gives the screen the window's update function: work done off
// the UI thread comes back through it.
func (l *Login) SetUpdate(update func(func())) { l.update = update }

// Resume signs the account used last back in, without asking: the screen
// shows only if that fails (its token is gone, or unreadable). It reports
// whether there was an account to try.
func (l *Login) Resume() bool {
	if len(l.sessions) == 0 {
		return false
	}
	l.continueSession(0)
	return true
}

// BeginAdd turns the screen into the way to add another account to a
// running app: the server step, with a Cancel that calls cancel.
func (l *Login) BeginAdd(cancel func()) {
	l.st = kit.LoginState{Adding: true}
	l.probed = nil
	l.onCancel = cancel
}

// EndAdd leaves the add-an-account mode (the account was added, or another
// opened): no Cancel, and the recorded logins are offered again.
func (l *Login) EndAdd() {
	l.st.Adding = false
	l.onCancel = nil
	l.st.Step, l.st.Err, l.st.Busy = kit.LoginServer, "", false
}

// Reload reads the recorded logins again (after one was added or removed).
func (l *Login) Reload() {
	l.sessions = nil
	l.st.Saved = nil
	if sessions, err := matrix.Sessions(); err == nil {
		l.sessions = sessions
		for _, s := range sessions {
			l.st.Saved = append(l.st.Saved, kit.SavedLogin{User: string(s.UserID), Server: s.ServerName})
		}
	}
}

	// Fail shows what stopped a sign-in from booting, as its quiet line.
func (l *Login) Fail(msg string) {
	l.post(func() { l.st.Err = msg })
}

// View is the sign-in screen.
func (l *Login) View(c *ui.Context) {
	kit.Apply(c)
	l.services = c.Services()

	var act kit.LoginAction
	kit.Login(c, &l.st, &act)
	if l.st.Busy {
		return // one sign-in at a time: a second click races the first's
	}
	switch {
	case act.Cancel && l.onCancel != nil:
		l.onCancel()
	case act.Continue >= 0:
		l.continueSession(act.Continue)
	case act.Probe != "":
		l.probe(act.Probe)
	case act.SignIn:
		l.signIn()
	case act.Browser:
		l.browser()
	}
}

// post runs fn on the UI thread, if the window is still there.
func (l *Login) post(fn func()) {
	if l.update != nil {
		l.update(fn)
	}
}

// continueSession signs a recorded login back in: its tokens are still
// good, or the screen asks to sign in again (the stores are never wiped).
func (l *Login) continueSession(i int) {
	if i >= len(l.sessions) {
		return
	}
	s := l.sessions[i]
	l.st.Busy, l.st.Err = true, ""
	go func() {
		cli, err := matrix.Dial(context.Background(), s)
		l.post(func() {
			l.st.Busy = false
			if err != nil {
				// A token that is gone means signing in again — the
				// stores stay. A token that cannot be read at all is the
				// keychain refusing: an ad-hoc dev build is a new app to
				// macOS, so its signature never matches the one that
				// saved the token. Signing in again is the answer either
				// way, and the Apple Development certificate mygo
				// suggests keeps it from happening twice.
				log.Printf("login: the saved login could not be used: %v", err)
				if errors.Is(err, matrix.ErrNoToken) {
					l.st.Err = "Sign in again — this login has no token."
				} else if errors.Is(err, matrix.ErrKeychain) {
					l.st.Err = "The system keychain would not give the saved login: " + short(err.Error()) +
						". On Linux this needs a Secret Service running and unlocked."
				} else {
					l.st.Err = "The saved login could not be read — sign in again."
				}
				return
			}
			_ = cli // the sync loop takes the client from here
			l.done(s)
		})
	}()
}

// probe resolves what the user typed and moves the screen to the
// server's way in.
func (l *Login) probe(server string) {
	l.st.Busy, l.st.Err, l.st.Field = true, "", ""
	go func() {
		srv, err := matrix.Probe(context.Background(), server)
		l.post(func() {
			l.st.Busy = false
			if err != nil {
				l.st.Err = err.Error()
				return
			}
			l.probed = srv
			l.st.Server = srv.Name
			if srv.Kind == matrix.KindOIDC {
				l.st.Step = kit.LoginBrowser
			} else {
				l.st.Step = kit.LoginPassword
			}
		})
	}()
}

// signIn submits the password form.
func (l *Login) signIn() {
	srv, user, pass := l.probed, l.st.User, l.st.Pass
	if srv == nil || user == "" || pass == "" {
		return
	}
	l.st.Busy, l.st.Err = true, ""
	go func() {
		s, err := matrix.LoginPassword(context.Background(), srv, user, pass)
		l.post(func() {
			l.st.Busy = false
			if err != nil {
				l.st.Err = loginErrText(err)
				return
			}
			l.done(s)
		})
	}()
}

// browser opens the server's MAS flow and waits for the loopback's
// answer. Ten minutes is plenty to find the browser window again.
func (l *Login) browser() {
	if l.probed == nil {
		return
	}
	l.st.Busy, l.st.Err = true, ""
	srv := l.probed
	open := func(url string) error {
		l.services.OpenURL(url)
		return nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		s, err := matrix.LoginOIDC(ctx, srv, open)
		l.post(func() {
			l.st.Busy = false
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					l.st.Err = "The browser login ran out of time. Try again."
				} else {
					l.st.Err = loginErrText(err)
				}
				return
			}
			l.done(s)
		})
	}()
}

// done hands the session to the app.
func (l *Login) done(s *matrix.Session) {
	l.st.Err = ""
	if l.OnSignedIn != nil {
		l.OnSignedIn(s)
	}
}

// loginErrText says what went wrong in one quiet line, without homeserver
// jargon.
func loginErrText(err error) string {
	log.Printf("login: failed: %v", err) // the log has the whole error; the screen, the gist
	var httpErr mautrix.HTTPError
	if errors.As(err, &httpErr) && httpErr.RespError != nil {
		switch httpErr.RespError.ErrCode {
		case "M_FORBIDDEN":
			return "Wrong username or password."
		case "M_LIMIT_EXCEEDED":
			return "Too many tries. Wait a moment and try again."
		case "M_USER_DEACTIVATED":
			return "This account has been deactivated."
		}
		// The server's own words, when it gave any.
		if msg := strings.TrimSpace(httpErr.RespError.Err); msg != "" {
			return "The server refused the sign-in: " + short(msg) + " (" + httpErr.RespError.ErrCode + ")"
		}
	}
	switch {
	case errors.Is(err, matrix.ErrKeychain) && errors.Is(err, errors.ErrUnsupported):
		// There is no credential store at all: on Linux, libsecret is not
		// installed, or no Secret Service is on the session bus.
		return "Signed in, but there is nowhere to keep the login. On Linux, install libsecret (libsecret-1-0 on Debian and Ubuntu) and run a Secret Service: GNOME Keyring, or KWallet with its Secret Service on."
	case errors.Is(err, matrix.ErrKeychain):
		// Signed in, but the system would not keep the login. On Linux that is
		// a locked Secret Service; on macOS the keychain's refusal.
		return "Signed in, but the system keychain refused to keep the login: " + short(err.Error()) +
			". On Linux this needs a Secret Service running and unlocked (GNOME Keyring, or KWallet's Secret Service)."
	case errors.As(err, new(net.Error)), errors.As(err, new(*url.Error)):
		return "Could not reach the server: " + short(err.Error())
	}
	if httpErr.Response != nil {
		return "The server answered " + httpErr.Response.Status + ". Check the server address and try again."
	}
	return "Could not sign in: " + short(err.Error())
}

// short is a message cut to one line of at most 200 characters.
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

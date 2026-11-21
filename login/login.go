// Package login is the sign-in screen over the Matrix backend: probe a
// server, sign in — the browser flow (MAS, first-class) or a password —
// and hand the session to the app. Every step's work runs off the UI
// thread and comes back through the window's update function, as the
// repo's rules say.
package login

import (
	"context"
	"errors"
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

// View is the sign-in screen.
func (l *Login) View(c *ui.Context) {
	kit.Apply(c)
	l.services = c.Services()

	var act kit.LoginAction
	kit.Login(c, &l.st, &act)
	switch {
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
				if errors.Is(err, matrix.ErrNoToken) {
					l.st.Err = "Sign in again — this login's token is gone."
				} else {
					l.st.Err = "Could not reach " + s.ServerName + "."
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
	var httpErr mautrix.HTTPError
	if errors.As(err, &httpErr) && httpErr.RespError != nil {
		switch httpErr.RespError.ErrCode {
		case "M_FORBIDDEN":
			return "Wrong username or password."
		case "M_LIMIT_EXCEEDED":
			return "Too many tries. Wait a moment and try again."
		}
	}
	return "Could not sign in: the server said no."
}

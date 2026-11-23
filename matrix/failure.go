package matrix

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	"maunium.net/go/mautrix"
)

// What a failed send says, in words a person can act on, and whether the
// failure means there is no connection at all.

// sendTimeout is how long a send may take before it counts as failed: the first
// message in a room shares its encryption key with every device, which can
// take a while, but not forever.
const sendTimeout = 90 * time.Second

// isNetworkError says err is the connection failing (no internet, a name that
// does not resolve, a refused connection, a timeout in the network), not the
// server answering "no".
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var he mautrix.HTTPError
	if errors.As(err, &he) && he.Response != nil {
		return false // the server answered
	}
	var dns *net.DNSError
	var op *net.OpError
	var ue *url.Error
	switch {
	case errors.As(err, &dns), errors.As(err, &op):
		return true
	case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.ENETUNREACH),
		errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ETIMEDOUT), errors.Is(err, os.ErrDeadlineExceeded):
		return true
	case errors.As(err, &ue):
		var ne net.Error
		return errors.As(ue.Err, &ne) || errors.Is(ue.Err, context.DeadlineExceeded)
	}
	return false
}

// sendReason says why a send failed.
func sendReason(err error) string {
	var he mautrix.HTTPError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "The server took too long to answer. Check your connection."
	case isNetworkError(err):
		var dns *net.DNSError
		var op *net.OpError
		if errors.As(err, &dns) || (errors.As(err, &op) && op.Op == "dial") {
			return "No internet connection."
		}
		return "Could not reach the server. Check your connection."
	case errors.As(err, &he):
		code := ""
		msg := ""
		if he.RespError != nil {
			code, msg = he.RespError.ErrCode, he.RespError.Err
		}
		status := 0
		if he.Response != nil {
			status = he.Response.StatusCode
		}
		switch {
		case code == "M_FORBIDDEN":
			return "You do not have permission to send here."
		case code == "M_LIMIT_EXCEEDED":
			return "You are sending too fast. Try again in a moment."
		case code == "M_TOO_LARGE":
			return "It is too large for the server."
		case code == "M_UNKNOWN_TOKEN" || code == "M_MISSING_TOKEN" || status == http.StatusUnauthorized:
			return "You were signed out. Sign in again."
		case code == "M_USER_LOCKED" || code == "M_USER_DEACTIVATED":
			return "This account cannot send messages."
		case status >= 500:
			return fmt.Sprintf("The server had a problem (%d). Try again in a moment.", status)
		case msg != "":
			return msg
		}
	}
	return plain(err)
}

// notSent is a failed message's error text: "Not sent: " and why.
func notSent(err error) string { return "Not sent: " + sendReason(err) }

// noteConnection records what a request's outcome says about the connection:
// the connection failing means offline; anything the server answered means it is
// there. UI thread.
func (s *Source) noteConnection(err error) {
	switch {
	case isNetworkError(err):
		s.setOffline(true)
	case err == nil:
		s.setOffline(false)
	default:
		var he mautrix.HTTPError
		if errors.As(err, &he) && he.Response != nil {
			s.setOffline(false) // the server answered: we are online
		}
	}
}

// setOffline changes the connection state and tells the app. UI thread.
func (s *Source) setOffline(off bool) {
	if s.offline == off {
		return
	}
	s.offline = off
	s.changed()
}

// Offline says the connection to the server is down: the last sync or send
// failed in the network, and nothing has succeeded since.
func (s *Source) Offline() bool { return s.offline }

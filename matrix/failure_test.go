package matrix

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

func httpErr(status int, code, msg string) error {
	return mautrix.HTTPError{Response: &http.Response{StatusCode: status}, RespError: &mautrix.RespError{ErrCode: code, Err: msg}}
}

// A failed send says why in words a person can act on, and says whether the
// connection is what failed.
func TestSendFailuresAreSaidPlainly(t *testing.T) {
	dns := &url.Error{Op: "Post", URL: "https://x", Err: &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "x", IsNotFound: true}}}
	refused := &url.Error{Op: "Post", URL: "https://x", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}}
	reset := &url.Error{Op: "Post", URL: "https://x", Err: &net.OpError{Op: "read", Err: syscall.ECONNRESET}}
	for _, tc := range []struct {
		err     error
		want    string
		network bool
	}{
		{dns, "No internet connection.", true},
		{refused, "No internet connection.", true},
		{reset, "Could not reach the server", true},
		{context.DeadlineExceeded, "took too long", false},
		{fmt.Errorf("sending: %w", context.DeadlineExceeded), "took too long", false},
		{httpErr(403, "M_FORBIDDEN", "You don't have permission"), "do not have permission", false},
		{httpErr(429, "M_LIMIT_EXCEEDED", "slow down"), "too fast", false},
		{httpErr(413, "M_TOO_LARGE", "big"), "too large", false},
		{httpErr(401, "M_UNKNOWN_TOKEN", "bad token"), "signed out", false},
		{httpErr(502, "", ""), "The server had a problem (502)", false},
		{httpErr(400, "M_BAD_JSON", "that was not JSON"), "that was not JSON", false},
		{errors.New("something odd"), "something odd", false},
	} {
		if got := sendReason(tc.err); !strings.Contains(got, tc.want) {
			t.Errorf("%v: %q, want it to say %q", tc.err, got, tc.want)
		}
		if isNetworkError(tc.err) != tc.network {
			t.Errorf("%v: network %v, want %v", tc.err, !tc.network, tc.network)
		}
	}
	if got := notSent(dns); got != "Not sent: No internet connection." {
		t.Fatalf("notSent: %q", got)
	}
}

// A send that cannot reach the server marks the message as failed with the
// reason, and says the connection is down; the next thing that gets an answer
// says it is back.
func TestFailedSendMarksTheMessageAndTheConnection(t *testing.T) {
	s, _ := testSynced()
	cli, _ := mautrix.NewClient("http://127.0.0.1:1", "@me:x", "t") // nothing listens: refused at once
	s.cli = cli
	s.me.ID = "@me:x"
	r := s.roomOf("!r")
	s.Send("!r", "hello", "")
	waitFor(t, "the failure", func() bool { return len(r.events) == 1 && r.events[0].Failed })
	if r.events[0].Pending {
		t.Fatalf("events: %+v", r.events)
	}
	if got := r.events[0].Error; !strings.HasPrefix(got, "Not sent: ") || strings.Count(got, "Not sent") != 1 {
		t.Fatalf("the error reads %q", got)
	}
	if !s.Offline() {
		t.Fatal("a send that could not reach the server did not say the connection is down")
	}
	// The server answering (even with a refusal) says the connection is there.
	s.noteConnection(httpErr(403, "M_FORBIDDEN", "no"))
	if s.Offline() {
		t.Fatal("an answer from the server did not clear it")
	}
}

var _ model.Connectivity = (*Source)(nil)

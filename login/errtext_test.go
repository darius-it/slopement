package login

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/matrix"
)

// A sign-in that fails says why, not only "the server said no": the keychain
// refusing, the network, or the server's own words.
func TestLoginErrorsSayWhy(t *testing.T) {
	respErr := func(code, msg string) error {
		return mautrix.HTTPError{Response: &http.Response{Status: "400 Bad Request"},
			RespError: &mautrix.RespError{ErrCode: code, Err: msg}}
	}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"wrong password", respErr("M_FORBIDDEN", "Invalid username/password"), "Wrong username or password."},
		{"rate limit", respErr("M_LIMIT_EXCEEDED", "slow down"), "Too many tries"},
		{"deactivated", respErr("M_USER_DEACTIVATED", "gone"), "deactivated"},
		{"the server's words", respErr("M_UNKNOWN", "Login is disabled for this client"), "Login is disabled for this client"},
		{"keychain", fmt.Errorf("%w: locked", matrix.ErrKeychain), "keychain"},
		{"no keychain at all", fmt.Errorf("%w: %w", matrix.ErrKeychain, errors.ErrUnsupported), "libsecret"},
		{"network", &url.Error{Op: "Post", URL: "https://x/login", Err: errors.New("dial tcp: no such host")}, "Could not reach the server"},
		{"anything else", errors.New("something odd happened"), "something odd happened"},
	} {
		got := loginErrText(tc.err)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q does not say %q", tc.name, got, tc.want)
		}
		if strings.Contains(got, "the server said no") {
			t.Errorf("%s: the catch-all hides the reason: %q", tc.name, got)
		}
	}
	if strings.Contains(loginErrText(errors.New(strings.Repeat("x", 500))), strings.Repeat("x", 300)) {
		t.Error("a long error is not cut")
	}
}

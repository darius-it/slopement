package matrix

import (
	"context"
	"fmt"
	"strings"

	"maunium.net/go/mautrix"
)

// LoginKind is how a server logs users in.
type LoginKind int

const (
	// KindOIDC is MAS or another OAuth/OIDC provider: the browser flow,
	// the first-class path (matrix.org works this way).
	KindOIDC LoginKind = iota
	// KindPassword is a username and a password form.
	KindPassword
)

// Server is a homeserver to log into, as the login screen shows it: what
// the user typed, where its client API really is, and how it logs in.
type Server struct {
	Name string // what the user typed
	URL  string // the client API's base URL, discovered
	Kind LoginKind
}

// Probe resolves what the user typed — a server name ("matrix.org") or a
// full URL — and asks how it logs users in: MAS/OIDC first, then a
// password form. Reachability comes with the answer, as the login screen
// needs both at once.
func Probe(ctx context.Context, input string) (*Server, error) {
	name := strings.TrimSpace(input)
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		return nil, fmt.Errorf("type a server, as matrix.org")
	}

	url := ""
	if strings.Contains(name, "://") {
		url = name // the user knew exactly where to go
	} else if wk, err := mautrix.DiscoverClientAPI(ctx, name); err == nil && wk.Homeserver.BaseURL != "" {
		url = wk.Homeserver.BaseURL // .well-known points elsewhere
	} else {
		url = "https://" + name // the common case: the server is its own
	}
	cli, err := mautrix.NewClient(url, "", "")
	if err != nil {
		return nil, fmt.Errorf("%q is not a server address", input)
	}
	flows, err := cli.GetLoginFlows(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s", name)
	}

	srv := &Server{Name: name, URL: url}
	// MAS advertises itself where the spec says; a server without it
	// answers with an error, and we take that answer as "no".
	if _, err := cli.OAuthGetServerMetadata(ctx); err == nil {
		srv.Kind = KindOIDC
		return srv, nil
	}
	if !flows.HasFlow(mautrix.AuthTypePassword) {
		return nil, fmt.Errorf("%s takes neither OIDC nor password login", name)
	}
	srv.Kind = KindPassword
	return srv, nil
}

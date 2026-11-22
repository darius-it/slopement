package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
	"maunium.net/go/mautrix/oauth"
)

// deviceName is what the server shows for this login in its sessions
// list.
const deviceName = "slopement"

// callbackPort is the loopback the browser returns to. A fixed port
// reuses the client registered last time (MAS matches the redirect
// exactly); a busy one falls back to any.
const callbackPort = 42310

// LoginPassword signs in with a username and a password and records the
// session: the token goes to the keychain, the record to the account's
// directory.
func LoginPassword(ctx context.Context, srv *Server, username, password string) (*Session, error) {
	cli, err := mautrix.NewClient(srv.URL, "", "")
	if err != nil {
		return nil, err
	}
	resp, err := cli.Login(ctx, &mautrix.ReqLogin{
		Type:                     mautrix.AuthTypePassword,
		Identifier:               mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: username},
		Password:                 password,
		InitialDeviceDisplayName: deviceName,
		StoreCredentials:         true,
	})
	if err != nil {
		return nil, err
	}
	s := &Session{
		UserID: resp.UserID, DeviceID: resp.DeviceID,
		ServerName: srv.Name, ServerURL: srv.URL,
	}
	if err := SaveTokens(string(s.UserID), string(s.DeviceID), Tokens{Access: resp.AccessToken}); err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

// LoginOIDC opens the server's browser flow (MAS: the first-class path,
// the one matrix.org takes) and waits for the loopback callback. open
// shows the authorization URL — the app opens the system's browser with
// it. It records the session like LoginPassword, refresh token included.
func LoginOIDC(ctx context.Context, srv *Server, open func(url string) error) (*Session, error) {
	cli, err := mautrix.NewClient(srv.URL, "", "")
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(callbackPort)))
	if err != nil {
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, err
		}
	}
	defer ln.Close()
	redirect := "http://" + ln.Addr().String() + "/callback"

	clientID, err := oauthClient(ctx, cli, redirect)
	if err != nil {
		return nil, err
	}

	// Who we are is only known after the flow (whoami); the hook that
	// saves renewed tokens needs it, so the first exchange is saved here,
	// the later refreshes through the hook.
	var user id.UserID
	var device id.DeviceID
	cli.SaveNewToken = func(ctx context.Context, refresh, access string, expiry time.Time) error {
		if user == "" {
			return nil
		}
		return SaveTokens(string(user), string(device), Tokens{Access: access, Refresh: refresh, Expires: expiry})
	}

	auth, err := cli.OAuthGetAuthorizationURL(ctx, oauth.GetAuthorizationURLParams{
		ClientID:    clientID,
		RedirectURI: redirect,
		// The device scope binds a session to an existing device; a
		// fresh login lets the server make one, which whoami reports.
		Scopes: oauth.ScopeList{oauth.ScopeClientAPI, oauth.ScopeOpenID},
	})
	if err != nil {
		return nil, err
	}
	if open != nil {
		if err = open(auth.URL); err != nil {
			return nil, err
		}
	}
	code, err := awaitCallback(ctx, ln, auth.State)
	if err != nil {
		return nil, err
	}

	tok, err := cli.OAuthExchangeToken(ctx, oauth.ExchangeTokenParams{
		ClientID:         clientID,
		Code:             code,
		CodeVerifier:     auth.CodeVerifier,
		RedirectURI:      redirect,
		StoreCredentials: true,
	})
	if err != nil {
		return nil, err
	}
	who, err := cli.Whoami(ctx)
	if err != nil {
		return nil, err
	}
	user, device = who.UserID, who.DeviceID

	s := &Session{
		UserID: user, DeviceID: device,
		ServerName: srv.Name, ServerURL: srv.URL,
	}
	if err := SaveTokens(string(user), string(device), Tokens{
		Access: tok.AccessToken, Refresh: tok.RefreshToken,
		Expires: time.Now().Add(tok.ExpiresIn.Duration),
	}); err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

// awaitCallback waits for the browser's return to the loopback: the
// code the server granted, checked against the state we sent.
func awaitCallback(ctx context.Context, ln net.Listener, state string) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	serve := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fail := func(e error) {
			io.WriteString(w, "<!doctype html><p style=\"font:15px system-ui;padding:24px\">"+e.Error()+"</p>")
			select {
			case errCh <- e:
			default:
			}
		}
		if e := q.Get("error"); e != "" {
			fail(fmt.Errorf("the server refused the login: %s", e))
			return
		}
		if q.Get("state") != state {
			fail(fmt.Errorf("the login answer is not for this window"))
			return
		}
		io.WriteString(w, `<!doctype html><p style="font:15px system-ui;padding:24px">You can return to slopement.</p><script>close()</script>`)
		select {
		case codeCh <- q.Get("code"):
		default:
		}
	})}
	serveErr := make(chan error, 1)
	go func() { serveErr <- serve.Serve(ln) }()
	defer serve.Close()

	select {
	case code := <-codeCh:
		if code == "" {
			return "", fmt.Errorf("the server sent no code")
		}
		return code, nil
	case err := <-errCh:
		return "", err
	case err := <-serveErr:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// oauthReg is one MAS dynamic-client registration, cached per issuer as
// MSC2966 suggests, in <config>/oauth-clients.json.
type oauthReg struct {
	ClientID string `json:"client_id"`
	Redirect string `json:"redirect_uri"`
}

func oauthClientsPath() (string, error) {
	root, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "oauth-clients.json"), nil
}

func loadOAuthClients() map[string]oauthReg {
	path, err := oauthClientsPath()
	if err != nil {
		return map[string]oauthReg{}
	}
	out := map[string]oauthReg{}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &out)
	}
	return out
}

func saveOAuthClients(clients map[string]oauthReg) error {
	path, err := oauthClientsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// oauthClient returns the client ID to authorize as, registering with
// the server the first time (MSC2966). A registration is only reused
// when this run's loopback matches the redirect it was made with.
func oauthClient(ctx context.Context, cli *mautrix.Client, redirect string) (string, error) {
	meta, err := cli.OAuthGetServerMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("reading the server's login setup: %w", err)
	}
	clients := loadOAuthClients()
	if reg := clients[meta.Issuer]; reg.ClientID != "" && reg.Redirect == redirect {
		return reg.ClientID, nil
	}
	resp, err := cli.OAuthRegisterClient(ctx, &oauth.ClientMetadata{
		ApplicationType:         oauth.ApplicationTypeNative,
		ClientName:              deviceName,
		ClientURI:               "https://slopement.app", // ours once there is a site
		RedirectURIs:            []string{redirect},
		GrantTypes:              []oauth.GrantType{oauth.GrantTypeAuthorizationCode, oauth.GrantTypeRefreshToken},
		ResponseTypes:           []oauth.ResponseType{oauth.ResponseTypeCode},
		TokenEndpointAuthMethod: oauth.AuthMethodNone,
	})
	if err != nil {
		return "", fmt.Errorf("registering with the server's login: %w", err)
	}
	clients[meta.Issuer] = oauthReg{ClientID: resp.ClientID, Redirect: redirect}
	if err := saveOAuthClients(clients); err != nil {
		return "", err
	}
	return resp.ClientID, nil
}

// Dial connects a recorded session: a client with its token, an OIDC
// session's set up to refresh it — and to save the renewed one to the
// keychain — ready for the sync loop.
func Dial(ctx context.Context, s *Session) (*mautrix.Client, error) {
	tok, err := TokensOf(string(s.UserID), string(s.DeviceID))
	if err != nil {
		return nil, fmt.Errorf("reading the saved login: %w", err)
	}
	cli, err := mautrix.NewClient(s.ServerURL, s.UserID, tok.Access)
	if err != nil {
		return nil, err
	}
	cli.Log = clientLogger()
	// NewClient takes no device ID; without it the crypto helper (rightly)
	// refuses the client as not logged in.
	cli.DeviceID = s.DeviceID
	cli.SaveNewToken = func(ctx context.Context, refresh, access string, expiry time.Time) error {
		return SaveTokens(string(s.UserID), string(s.DeviceID), Tokens{Access: access, Refresh: refresh, Expires: expiry})
	}
	if tok.Refresh != "" {
		// The refresh needs the metadata and the client ID again — the
		// client is a fresh one every run.
		if meta, err := cli.OAuthGetServerMetadata(ctx); err == nil {
			cli.OAuthSetServerMetadata(meta)
			if reg := loadOAuthClients()[meta.Issuer]; reg.ClientID != "" {
				cli.OAuthSetTokens(reg.ClientID, tok.Refresh, tok.Access, tok.Expires)
			}
		}
	}
	return cli, nil
}

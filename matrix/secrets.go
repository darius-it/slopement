package matrix

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo"
)

// What an account keeps in the OS keychain (never in a database): its
// tokens, and the pickle key that encrypts the crypto material in
// crypto.db. Entries are keyed by user ID and device ID, as the
// requirements say; the service is the app's own identifier.

// secretStore is the keychain seam: mygo's Secrets in the app, a fake in
// tests, so no test touches the user's real keychain.
type secretStore interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

var secrets secretStore = mygo.Secrets

// ErrNoToken is returned when the account has no token saved: a revoked
// login (M_UNKNOWN_TOKEN soft-logout keeps the stores) shows sign-in
// again, never wiping the keys.
var ErrNoToken = errors.New("no saved token; sign in again")

// secretKey names one account's secret of a kind.
func secretKey(userID, deviceID, kind string) string {
	return fmt.Sprintf("account/%s/%s/%s", userID, deviceID, kind)
}

// Tokens is what the keychain holds for one login: an access token and,
// when the server renews it (MAS), the refresh token and when the access
// one expires.
type Tokens struct {
	Access  string    `json:"access"`
	Refresh string    `json:"refresh,omitempty"`
	Expires time.Time `json:"expires,omitempty"`
}

// SaveTokens stores a login's tokens.
func SaveTokens(userID, deviceID string, t Tokens) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return secrets.Set(secretKey(userID, deviceID, "tokens"), b)
}

// TokensOf returns a login's saved tokens.
func TokensOf(userID, deviceID string) (Tokens, error) {
	b, err := secrets.Get(secretKey(userID, deviceID, "tokens"))
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return Tokens{}, fmt.Errorf("%w for %s/%s", ErrNoToken, userID, deviceID)
	}
	if err != nil {
		return Tokens{}, err
	}
	var t Tokens
	if err := json.Unmarshal(b, &t); err != nil {
		return Tokens{}, err
	}
	return t, nil
}

// DeleteTokens forgets a login's tokens, as a logout does.
func DeleteTokens(userID, deviceID string) error {
	return secrets.Delete(secretKey(userID, deviceID, "tokens"))
}

// PickleKey returns the key that encrypts the account's crypto material
// in crypto.db, making one up the first time. Losing it is fine — the
// sessions are re-established by verifying again; that is also why it is
// per device, not some global secret.
func PickleKey(userID, deviceID string) ([]byte, error) {
	key := secretKey(userID, deviceID, "pickle-key")
	if b, err := secrets.Get(key); err == nil {
		return b, nil
	} else if !errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := secrets.Set(key, b); err != nil {
		return nil, err
	}
	return b, nil
}

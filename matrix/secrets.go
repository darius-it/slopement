package matrix

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/egoist/mygo"
)

// What an account keeps in the OS keychain (never in a database): its
// access token, and the pickle key that encrypts the crypto material in
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

// ErrNoToken is returned when the account has no access token saved.
var ErrNoToken = errors.New("no saved access token")

// secretKey names one account's secret of a kind.
func secretKey(userID, deviceID, kind string) string {
	return fmt.Sprintf("account/%s/%s/%s", userID, deviceID, kind)
}

// SaveToken stores an account's access token.
func SaveToken(userID, deviceID, token string) error {
	return secrets.Set(secretKey(userID, deviceID, "token"), []byte(token))
}

// Token returns the account's saved access token.
func Token(userID, deviceID string) (string, error) {
	b, err := secrets.Get(secretKey(userID, deviceID, "token"))
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return "", fmt.Errorf("%w for %s/%s", ErrNoToken, userID, deviceID)
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DeleteToken forgets an account's access token, as a logout does.
func DeleteToken(userID, deviceID string) error {
	return secrets.Delete(secretKey(userID, deviceID, "token"))
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

package matrix

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

// DevSecretsEnv, set to "1", keeps the account's secrets (tokens, the key
// that seals the crypto store) in a file in the config directory instead of
// the OS keychain. For development only: a development build is signed ad
// hoc, so macOS takes every rebuild for a new app and asks for the keychain
// again each time. The file is private to the user (0600) but not
// encrypted — a released build never uses it.
const DevSecretsEnv = "SLOPEMENT_DEV_SECRETS"

// pickSecrets is the store the package uses: the OS keychain, or in
// development (DevSecretsEnv) the file.
func pickSecrets() secretStore {
	if os.Getenv(DevSecretsEnv) == "1" {
		return &fileSecrets{fallback: mygo.Secrets}
	}
	return mygo.Secrets
}

// fileSecrets is the development store: one JSON file of base64 values.
// A value it does not have yet is looked for in fallback (the keychain,
// where an earlier run kept it) and copied over — one last keychain prompt
// per item, and no sign-in again.
type fileSecrets struct {
	mu       sync.Mutex
	fallback secretStore
}

func (f *fileSecrets) path() (string, error) {
	root, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "dev-secrets.json"), nil
}

func (f *fileSecrets) load() (map[string]string, string, error) {
	path, err := f.path()
	if err != nil {
		return nil, "", err
	}
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, path, nil
		}
		return nil, path, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, path, err
		}
	}
	return m, path, nil
}

func (f *fileSecrets) save(m map[string]string, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (f *fileSecrets) Set(key string, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, path, err := f.load()
	if err != nil {
		return err
	}
	m[key] = base64.StdEncoding.EncodeToString(value)
	return f.save(m, path)
}

func (f *fileSecrets) Get(key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, _, err := f.load()
	if err != nil {
		return nil, err
	}
	v, ok := m[key]
	if !ok {
		if f.fallback != nil {
			if b, ferr := f.fallback.Get(key); ferr == nil {
				m[key] = base64.StdEncoding.EncodeToString(b)
				if path, perr := f.path(); perr == nil {
					_ = f.save(m, path)
				}
				return b, nil
			}
		}
		return nil, mygo.ErrSecretNotFound
	}
	return base64.StdEncoding.DecodeString(v)
}

func (f *fileSecrets) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, path, err := f.load()
	if err != nil {
		return err
	}
	delete(m, key)
	return f.save(m, path)
}

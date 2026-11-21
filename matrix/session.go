package matrix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"maunium.net/go/mautrix/id"
)

// Session is a login recorded in the config directory: who, on which
// device, on which server. The tokens stay in the keychain (secrets.go);
// this is only what the app needs to find them again.
type Session struct {
	UserID     id.UserID   `json:"user_id"`
	DeviceID   id.DeviceID `json:"device_id"`
	ServerName string      `json:"server_name"` // what the user typed
	ServerURL  string      `json:"server_url"`  // the resolved client API
}

// accountFile is the session's record in its account directory.
const accountFile = "account.json"

// save records the session in its account's directory, next to the
// stores: what a later boot reads to know who is logged in here.
func (s *Session) save() error {
	dir, err := AccountDir(string(s.UserID))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, accountFile), b, 0o600)
}

// Sessions lists the logins recorded in the config directory, the most
// recent first: one boots straight into it, several show a picker.
func Sessions() ([]*Session, error) {
	root, err := configDir()
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(root, "accounts", "*", accountFile))
	if err != nil {
		return nil, err
	}
	out := make([]*Session, 0, len(matches))
	mtime := map[*Session]int64{}
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s := &Session{}
		if json.Unmarshal(b, s) != nil || s.UserID == "" {
			continue
		}
		out = append(out, s)
		if st, err := os.Stat(f); err == nil {
			mtime[s] = st.ModTime().Unix()
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return mtime[out[i]] > mtime[out[j]] })
	// The one used last comes first, whatever else was touched since: it is
	// the one the app opens on its own.
	if last := LastUsed(); last != "" {
		sort.SliceStable(out, func(i, j int) bool { return string(out[i].UserID) == last && string(out[j].UserID) != last })
	}
	return out, nil
}

// lastUsedFile is where the account used last is remembered, in the config
// directory (not in an account's: it names which account).
const lastUsedFile = "last-account"

// SetLastUsed remembers which account the app was on, so the next launch
// opens it without asking.
func SetLastUsed(userID string) error {
	root, err := configDir()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, lastUsedFile), []byte(userID), 0o600)
}

// LastUsed is the account the app was on last ("" for none).
func LastUsed() string {
	root, err := configDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(root, lastUsedFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Delete forgets a session's record — the tokens with it, as a logout
// does. The stores stay (a soft logout keeps the keys); wiping them is a
// separate, explicit step.
func (s *Session) Delete() error {
	dir, err := AccountDir(string(s.UserID))
	if err != nil {
		return err
	}
	if err = DeleteTokens(string(s.UserID), string(s.DeviceID)); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, accountFile))
}

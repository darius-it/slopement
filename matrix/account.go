package matrix

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"go.mau.fi/util/dbutil"

	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/id"
	"maunium.net/go/mautrix/sqlstatestore"

	// The SQLite driver: pure Go, so the whole app builds without cgo.
	_ "modernc.org/sqlite"
)

// Account is one logged-in user: their directory, the session lock on it,
// and the two stores — the room state and the crypto material. Open takes
// the lock and upgrades the schemas; Close releases everything.
type Account struct {
	UserID   id.UserID
	DeviceID id.DeviceID
	Dir      string

	// State is the room state store (rooms, members, messages' metadata).
	State *sqlstatestore.SQLStateStore
	// Crypto is the E2EE store (olm and megolm sessions, devices, trust).
	Crypto *crypto.SQLCryptoStore

	stateDB, cryptoDB *dbutil.Database
	lock               *sessionLock
}

// configDir is where slopement keeps per-user data; tests point it at a
// scratch directory.
var configDir = defaultConfigDir

func defaultConfigDir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "slopement"), nil
}

// AccountDir is an account's directory, named by the hash of its user ID
// (as the requirements say: accounts/<sha256(user_id)>/). It may not
// exist yet.
func AccountDir(userID string) (string, error) {
	sum := sha256.Sum256([]byte(userID))
	root, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "accounts", hex.EncodeToString(sum[:16])), nil
}

// dbDSN is a SQLite DSN for modernc's driver: WAL so a reader never
// blocks the sync loop's writes, and a busy timeout for the rare
// contention anyway.
func dbDSN(path string) string {
	q := make(url.Values, 2)
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "journal_mode(WAL)")
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: q.Encode()}
	return u.String()
}

// Open prepares an account: its directory, the session lock on it, and
// the state and crypto stores, upgraded to the schemas this build knows.
// It fails with ErrLocked when another process holds the account.
//
// The token is not loaded here — a client needs it, the stores don't.
func Open(ctx context.Context, userID id.UserID, deviceID id.DeviceID) (*Account, error) {
	dir, err := AccountDir(string(userID))
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock, err := lockSession(dir)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Account, error) {
		lock.release()
		return nil, err
	}

	// The crypto material is encrypted with the account's pickle key,
	// which lives in the keychain, made on first use.
	pickle, err := PickleKey(string(userID), string(deviceID))
	if err != nil {
		return fail(fmt.Errorf("reading the crypto key: %w", err))
	}

	stateDB, err := dbutil.NewWithDialect(dbDSN(filepath.Join(dir, "state.db")), "sqlite")
	if err != nil {
		return fail(fmt.Errorf("opening state.db: %w", err))
	}
	state := sqlstatestore.NewSQLStateStore(stateDB, dbutil.NoopLogger, false)
	if err = state.Upgrade(ctx); err != nil {
		stateDB.Close()
		return fail(fmt.Errorf("upgrading state.db: %w", err))
	}

	cryptoDB, err := dbutil.NewWithDialect(dbDSN(filepath.Join(dir, "crypto.db")), "sqlite")
	if err != nil {
		stateDB.Close()
		return fail(fmt.Errorf("opening crypto.db: %w", err))
	}
	store := crypto.NewSQLCryptoStore(cryptoDB, dbutil.NoopLogger, string(userID), deviceID, pickle)
	if err = store.DB.Upgrade(ctx); err != nil {
		cryptoDB.Close()
		stateDB.Close()
		return fail(fmt.Errorf("upgrading crypto.db: %w", err))
	}

	return &Account{
		UserID: userID, DeviceID: deviceID, Dir: dir,
		State: state, Crypto: store,
		stateDB: stateDB, cryptoDB: cryptoDB, lock: lock,
	}, nil
}

// Close releases the account's databases and the session lock. The
// account can be opened again after it.
func (a *Account) Close() error {
	var err, err2 error
	if err2 = a.stateDB.Close(); err == nil {
		err = err2
	}
	if err2 = a.cryptoDB.Close(); err == nil {
		err = err2
	}
	if err2 = a.lock.release(); err == nil {
		err = err2
	}
	return err
}

package matrix

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	// Cache is the room cache (cache.db): what the sync has told the app,
	// so a launch resumes instead of starting over.
	Cache *Cache

	stateDB, cryptoDB *dbutil.Database
	lock              *sessionLock
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
	u := url.URL{Scheme: "file", Path: dsnPath(path), RawQuery: q.Encode()}
	return u.String()
}

// dsnPath is a file path as the path of a SQLite file: URI. A Windows path
// starts with its drive, "C:\Users\me\state.db", which in "file://C:/…" would
// read as a host: the URI needs a slash first, "file:///C:/Users/me/state.db".
func dsnPath(path string) string {
	p := filepath.ToSlash(path)
	if len(p) >= 2 && p[1] == ':' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z') {
		return "/" + strings.ReplaceAll(p, `\`, "/")
	}
	return p
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
	accountID := string(userID) + "/" + string(deviceID)

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
	store := crypto.NewSQLCryptoStore(cryptoDB, dbutil.NoopLogger,
		// The crypto material is per device: a new login starts with
		// nothing of the last one's, its sync token included — a fresh
		// device must see the whole server state. (Rows older than this
		// keying, under the user alone, are moved by
		// migrateCryptoAccount below.)
		accountID, deviceID, pickle)
	if err = store.DB.Upgrade(ctx); err != nil {
		cryptoDB.Close()
		stateDB.Close()
		return fail(fmt.Errorf("upgrading crypto.db: %w", err))
	}
	if err = migrateCryptoAccount(ctx, cryptoDB, string(userID), accountID); err != nil {
		cryptoDB.Close()
		stateDB.Close()
		return fail(fmt.Errorf("migrating the crypto rows: %w", err))
	}

	cache, err := openCache(dir)
	if err != nil {
		cryptoDB.Close()
		stateDB.Close()
		return fail(err)
	}

	return &Account{
		UserID: userID, DeviceID: deviceID, Dir: dir,
		State: state, Crypto: store, Cache: cache,
		stateDB: stateDB, cryptoDB: cryptoDB, lock: lock,
	}, nil
}

// migrateCryptoAccount moves the crypto rows of an older build — keyed by
// the user alone — to the per-device key this one uses: the same device's
// identity, sessions and secrets, only renamed, so resuming a saved login
// keeps working instead of tripping mautrix's "keys on the server"
// guard. A half-created per-device row from a boot that failed the guard
// is cleared first; new databases have nothing to move.
func migrateCryptoAccount(ctx context.Context, db *dbutil.Database, userID, deviceKey string) error {
	var old, new bool
	if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM crypto_account WHERE account_id=$1)", userID).Scan(&old); err != nil || !old {
		return err // nothing to move (or unreadable, which Open reports)
	}
	if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM crypto_account WHERE account_id=$1)", deviceKey).Scan(&new); err != nil {
		return err
	}
	if new {
		// The failed boot left a fresh identity this device never used;
		// the row it should have is the old one, renamed below.
		if _, err := db.Exec(ctx, "DELETE FROM crypto_account WHERE account_id=$1", deviceKey); err != nil {
			return fmt.Errorf("clearing the half-created row: %w", err)
		}
	}
	for _, table := range []string{
		"crypto_account", "crypto_olm_session",
		"crypto_megolm_inbound_session", "crypto_megolm_outbound_session",
		"crypto_secrets",
	} {
		if _, err := db.Exec(ctx, "UPDATE "+table+" SET account_id=$1 WHERE account_id=$2", deviceKey, userID); err != nil {
			return fmt.Errorf("moving %s: %w", table, err)
		}
	}
	return nil
}

// Close releases the account's databases and the session lock. The
// account can be opened again after it.
func (a *Account) Close() error {
	var err, err2 error
	if a.Cache != nil {
		err = a.Cache.close()
	}
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

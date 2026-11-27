package matrix

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// The room cache: what the sync loop has told the app, kept in cache.db
// so a launch shows the rooms and their timelines at once and resumes the
// sync from where it stopped, instead of downloading the account again.
//
// The sync token and the rooms are written in one transaction, so the
// cache is always a state the server's token continues from; a cache that
// is not trusted (another build's schema, another device) is dropped and
// the sync starts from nothing, as a first login does.

// cacheVersion is bumped when the shape of what is stored changes.
//
// 2: rooms carry their avatar (m.room.avatar), which a resumed sync would
// never send again for rooms cached without it.
//
// 3: edits are kept by their target; earlier caches hold edits that were
// shown as messages of their own.
//
// 4: reactions are kept by the event that made each (so one can be taken
// back), not by who made it.
//
// 5: events carry their formatted body (or are marked plain), so lists and
// formatting show as the sender wrote them.
const cacheVersion = "5"

// Cache is the room cache of one account.
type Cache struct {
	db  *sql.DB
	gcm cipher.AEAD

	writes chan cacheWrite
	done   chan struct{}
	once   sync.Once
}

// cacheWrite is one transaction: the token the state continues from, the
// rooms that changed, the rooms that are gone, and the other records.
type cacheWrite struct {
	token string
	rooms map[string][]byte
	drop  []string
	meta  map[string][]byte
	ack   chan struct{} // closed when written (Flush); may be nil
}

// cacheSnapshot is what a launch reads back.
type cacheSnapshot struct {
	token  string
	rooms  map[string][]byte
	people  []byte
	direct  []byte
	invites []byte
}

// setKey sets the key the cached rooms and people are sealed with
// (AES-GCM): they hold decrypted messages, which must not sit on disk in
// the clear. The key is the account's, from the keychain; without one
// (tests) nothing is sealed.
func (c *Cache) setKey(key []byte) {
	if len(key) == 0 {
		return
	}
	sum := sha256.Sum256(append([]byte("slopement cache"), key...))
	if block, err := aes.NewCipher(sum[:]); err == nil {
		c.gcm, _ = cipher.NewGCM(block)
	}
}

func (c *Cache) seal(b []byte) []byte {
	if c.gcm == nil {
		return b
	}
	nonce := make([]byte, c.gcm.NonceSize())
	_, _ = rand.Read(nonce)
	return c.gcm.Seal(nonce, nonce, b, nil)
}

func (c *Cache) open(b []byte) ([]byte, error) {
	if c.gcm == nil {
		return b, nil
	}
	n := c.gcm.NonceSize()
	if len(b) < n {
		return nil, errors.New("cache record too short")
	}
	return c.gcm.Open(nil, b[:n], b[n:], nil)
}

// openCache opens (creating) the account's cache.db and starts its writer.
func openCache(dir string) (*Cache, error) {
	db, err := sql.Open("sqlite", dbDSN(filepath.Join(dir, "cache.db")))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writes one at a time; the writer is the one user
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS rooms (id TEXT PRIMARY KEY, data BLOB NOT NULL)`,
	} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, fmt.Errorf("preparing cache.db: %w", err)
		}
	}
	c := &Cache{db: db, writes: make(chan cacheWrite, 64), done: make(chan struct{})}
	go c.writer()
	return c, nil
}

// load reads the cache back. ok is false (and the cache emptied) when
// there is nothing to resume from or it belongs to another build or
// device.
func (c *Cache) load(ctx context.Context, device id.DeviceID) (snap cacheSnapshot, ok bool) {
	meta := func(key string) (string, bool) {
		var v []byte
		if err := c.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key=?`, key).Scan(&v); err != nil {
			return "", false
		}
		return string(v), true
	}
	if v, found := meta("version"); !found || v != cacheVersion {
		c.reset(ctx)
		return snap, false
	}
	if d, _ := meta("device"); d != string(device) {
		c.reset(ctx)
		return snap, false
	}
	token, found := meta("next_batch")
	if !found || token == "" {
		return snap, false
	}
	snap.token = token
	for key, dst := range map[string]*[]byte{"people": &snap.people, "direct": &snap.direct, "invites": &snap.invites} {
		if v, found := meta(key); found {
			plain, err := c.open([]byte(v))
			if err != nil {
				c.reset(ctx) // sealed with another key: nothing here can be read
				return cacheSnapshot{}, false
			}
			*dst = plain
		}
	}
	snap.rooms = map[string][]byte{}
	rows, err := c.db.QueryContext(ctx, `SELECT id, data FROM rooms`)
	if err != nil {
		c.reset(ctx)
		return cacheSnapshot{}, false
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			c.reset(ctx)
			return cacheSnapshot{}, false
		}
		plain, err := c.open(data)
		if err != nil {
			c.reset(ctx)
			return cacheSnapshot{}, false
		}
		snap.rooms[id] = plain
	}
	if rows.Err() != nil {
		c.reset(ctx)
		return cacheSnapshot{}, false
	}
	return snap, true
}

// reset forgets everything stored: the next launch syncs from nothing.
func (c *Cache) reset(ctx context.Context) {
	_, _ = c.db.ExecContext(ctx, `DELETE FROM rooms`)
	_, _ = c.db.ExecContext(ctx, `DELETE FROM meta`)
}

// write queues one transaction for the writer.
func (c *Cache) write(w cacheWrite) {
	defer func() { _ = recover() }() // a write after close is dropped
	c.writes <- w
}

// flush waits for what is queued to be on disk.
func (c *Cache) flush() {
	ack := make(chan struct{})
	c.write(cacheWrite{ack: ack})
	select {
	case <-ack:
	case <-c.done:
	}
}

func (c *Cache) writer() {
	defer close(c.done)
	for w := range c.writes {
		if err := c.apply(w); err != nil {
			fmt.Fprintln(os.Stderr, "slopement: writing the room cache:", err)
		}
		if w.ack != nil {
			close(w.ack)
		}
	}
}

func (c *Cache) apply(w cacheWrite) error {
	if w.token == "" && len(w.rooms) == 0 && len(w.drop) == 0 && len(w.meta) == 0 {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id, data := range w.rooms {
		if _, err = tx.Exec(`INSERT INTO rooms(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, id, c.seal(data)); err != nil {
			return err
		}
	}
	for _, id := range w.drop {
		if _, err = tx.Exec(`DELETE FROM rooms WHERE id=?`, id); err != nil {
			return err
		}
	}
	put := func(key string, value []byte) error {
		_, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
		return err
	}
	for key, v := range w.meta {
		if key == "people" || key == "direct" || key == "invites" {
			v = c.seal(v) // names, DMs and invitations are as private as the rooms
		}
		if err = put(key, v); err != nil {
			return err
		}
	}
	if w.token != "" {
		if err = put("next_batch", []byte(w.token)); err != nil {
			return err
		}
		if err = put("version", []byte(cacheVersion)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// close drains the writer and closes the database.
func (c *Cache) close() error {
	c.once.Do(func() { close(c.writes) })
	<-c.done
	return c.db.Close()
}

// tokenStore is the sync store the client uses. On disk the sync token is
// the cache's: saved together with the state it continues, never on its own.
// Here it is only held in memory, so that a sync run started again (a wake,
// a dropped connection) goes on from where the last one was rather than from
// where the app started. The filter id is held the same way: the crypto
// store keeps none, and every run would upload the filter before it synced.
// The sync goroutine's alone.
type tokenStore struct {
	mautrix.SyncStore
	token  string
	filter string
}

func (t *tokenStore) LoadNextBatch(context.Context, id.UserID) (string, error) { return t.token, nil }

func (t *tokenStore) SaveNextBatch(_ context.Context, _ id.UserID, token string) error {
	t.token = token
	return nil
}

func (t *tokenStore) LoadFilterID(context.Context, id.UserID) (string, error) { return t.filter, nil }

func (t *tokenStore) SaveFilterID(_ context.Context, _ id.UserID, filter string) error {
	t.filter = filter
	return nil
}

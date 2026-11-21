package matrix

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/egoist/mygo"

	"maunium.net/go/mautrix/id"
)

// fakeSecrets is an in-memory keychain for tests, speaking the real one's
// error contract: a missing key is mygo.ErrSecretNotFound.
type fakeSecrets struct {
	m map[string][]byte
}

func (f *fakeSecrets) Set(key string, value []byte) error {
	f.m[key] = append([]byte(nil), value...)
	return nil
}

func (f *fakeSecrets) Get(key string) ([]byte, error) {
	if b, ok := f.m[key]; ok {
		return append([]byte(nil), b...), nil
	}
	return nil, mygo.ErrSecretNotFound
}

func (f *fakeSecrets) Delete(key string) error {
	delete(f.m, key)
	return nil
}

// openTest swaps the package's seams for one scratch config directory
// (one, not per call) and a fake keychain, and restores them after the
// test.
func openTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldDir, oldSecrets := configDir, secrets
	configDir = func() (string, error) { return dir, nil }
	secrets = &fakeSecrets{m: map[string][]byte{}}
	t.Cleanup(func() { configDir, secrets = oldDir, oldSecrets })
}

const testUser, testDevice = "@ada:example.org", "SLOPEMENT"

func TestAccountOpenLock(t *testing.T) {
	openTest(t)
	ctx := context.Background()

	a, err := Open(ctx, testUser, testDevice)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.UserID != id.UserID(testUser) {
		t.Fatalf("user = %s", a.UserID)
	}
	if _, err = os.Stat(a.Dir); err != nil { // the directory exists
		t.Fatalf("no account directory: %v", err)
	}

	// The state store works: registered round-trips through state.db.
	if err = a.State.MarkRegistered(ctx, id.UserID(testUser)); err != nil {
		t.Fatal(err)
	}
	if ok, err := a.State.IsRegistered(ctx, id.UserID(testUser)); err != nil || !ok {
		t.Fatalf("registered = %v, %v", ok, err)
	}

	// The crypto store's schema is there: the sync batch token lives in
	// the account's row, which the crypto machine makes (with the sync
	// milestone); on a fresh store it reads as none, without an error.
	if b, err := a.Crypto.GetNextBatch(ctx); err != nil || b != "" {
		t.Fatalf("next batch = %q, %v", b, err)
	}

	// One process per account: a second open is refused while the first
	// holds the lock, and works again after Close.
	if _, err = Open(ctx, testUser, testDevice); !errors.Is(err, ErrLocked) {
		t.Fatalf("second open: %v, want ErrLocked", err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	a2, err := Open(ctx, testUser, testDevice)
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	// state.db persisted across the reopen.
	if ok, err := a2.State.IsRegistered(ctx, id.UserID(testUser)); err != nil || !ok {
		t.Fatalf("state did not persist: %v", err)
	}
	a2.Close()
}

func TestPickleKey(t *testing.T) {
	openTest(t)
	k1, err := PickleKey(testUser, testDevice)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("pickle key is %d bytes, want 32", len(k1))
	}
	// Made once, then stable.
	k2, err := PickleKey(testUser, testDevice)
	if err != nil || string(k1) != string(k2) {
		t.Fatalf("pickle key changed: %v", err)
	}
	// Per device.
	k3, err := PickleKey(testUser, "OTHER")
	if err != nil || string(k3) == string(k1) {
		t.Fatalf("devices share a pickle key: %v", err)
	}
}

func TestToken(t *testing.T) {
	openTest(t)
	if _, err := Token(testUser, testDevice); !errors.Is(err, ErrNoToken) {
		t.Fatalf("missing token: %v, want ErrNoToken", err)
	}
	if err := SaveToken(testUser, testDevice, "s3kr3t"); err != nil {
		t.Fatal(err)
	}
	if tok, err := Token(testUser, testDevice); err != nil || tok != "s3kr3t" {
		t.Fatalf("token = %q, %v", tok, err)
	}
	if err := DeleteToken(testUser, testDevice); err != nil {
		t.Fatal(err)
	}
	if _, err := Token(testUser, testDevice); !errors.Is(err, ErrNoToken) {
		t.Fatalf("after delete: %v, want ErrNoToken", err)
	}
}

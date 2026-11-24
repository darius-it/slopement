package matrix

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// The development store keeps secrets in a private file, with the keychain's
// error contract, and survives being opened again.
func TestDevSecretsFile(t *testing.T) {
	openTest(t) // a scratch config directory
	f := &fileSecrets{}
	if _, err := f.Get("a"); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Fatalf("missing key: %v", err)
	}
	if err := f.Set("a", []byte{0, 1, 2, 255}); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("b", []byte("two")); err != nil {
		t.Fatal(err)
	}
	again := &fileSecrets{} // another run
	if got, err := again.Get("a"); err != nil || string(got) != string([]byte{0, 1, 2, 255}) {
		t.Fatalf("a = %v, %v", got, err)
	}
	if err := again.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Get("a"); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Fatalf("deleted key: %v", err)
	}
	if got, _ := again.Get("b"); string(got) != "two" {
		t.Fatalf("b = %q", got)
	}
	path, _ := f.path()
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 || filepath.Base(path) != "dev-secrets.json" {
		t.Fatalf("the file: %v %v", st, err)
	}
}

// A secret the file does not have is taken from the keychain once and kept
// in the file, so the keychain is asked once and not again.
func TestDevSecretsMigrateFromKeychain(t *testing.T) {
	openTest(t)
	keychain := &fakeSecrets{m: map[string][]byte{"tok": []byte("old-token")}}
	f := &fileSecrets{fallback: keychain}
	if got, err := f.Get("tok"); err != nil || string(got) != "old-token" {
		t.Fatalf("first read: %q %v", got, err)
	}
	delete(keychain.m, "tok") // gone from the keychain: the file has it now
	if got, err := f.Get("tok"); err != nil || string(got) != "old-token" {
		t.Fatalf("second read: %q %v", got, err)
	}
	if _, err := f.Get("never"); !errors.Is(err, mygo.ErrSecretNotFound) {
		t.Fatalf("a secret nowhere: %v", err)
	}
}

// A development build and an installed app must not share an account record: they
// keep tokens in different places, so sharing it made each sign in again and
// overwrite the other's device, one new device per switch.
func TestDevBuildHasItsOwnConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv(DevSecretsEnv, "")
	stable, err := defaultConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(DevSecretsEnv, "1")
	dev, err := defaultConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if stable == dev || filepath.Base(stable) != "slopement" || filepath.Base(dev) != "slopement-dev" {
		t.Fatalf("stable %q, dev %q", stable, dev)
	}
	if filepath.Dir(stable) != filepath.Dir(dev) {
		t.Fatal("not side by side")
	}
}

// The last account used comes first in the list of accounts.
func TestLastUsedAccountComesFirst(t *testing.T) {
	openTest(t)
	for _, s := range []*Session{
		{UserID: "@a:x", DeviceID: "A", ServerURL: "https://x"},
		{UserID: "@b:x", DeviceID: "B", ServerURL: "https://x"},
	} {
		if err := s.save(); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetLastUsed("@a:x"); err != nil {
		t.Fatal(err)
	}
	got, _ := Sessions()
	if len(got) != 2 || got[0].UserID != "@a:x" {
		t.Fatalf("sessions = %+v", got)
	}
	_ = SetLastUsed("@b:x")
	if got, _ = Sessions(); got[0].UserID != "@b:x" || LastUsed() != "@b:x" {
		t.Fatalf("after switching: %+v", got)
	}
}

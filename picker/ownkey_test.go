package picker

import (
	"path/filepath"
	"strings"
	"testing"

	"slopement/klipy"
)

// The user's own KLIPY key turns search on at once, is kept for the next run,
// can be removed, and never replaces one from the environment.
func TestUsersOwnKlipyKey(t *testing.T) {
	t.Setenv("KLIPY_API_KEY", "")
	path := filepath.Join(t.TempDir(), "picker.json")

	c := newAt(klipy.New(klipy.Config{}), path)
	if c.HasKlipyKey() {
		t.Fatal("a key from nowhere")
	}
	c.SetKlipyKey("  abc123  ")
	if !c.HasKlipyKey() || c.KlipyKey() != "abc123" {
		t.Fatalf("key = %q, has %v", c.KlipyKey(), c.HasKlipyKey())
	}

	// Next run.
	c2 := newAt(klipy.New(klipy.Config{}), path)
	if !c2.HasKlipyKey() || c2.KlipyKey() != "abc123" || c2.KlipyKeyFromEnvironment() {
		t.Fatalf("after a restart: key %q has %v env %v", c2.KlipyKey(), c2.HasKlipyKey(), c2.KlipyKeyFromEnvironment())
	}

	c2.SetKlipyKey("")
	if c2.HasKlipyKey() {
		t.Fatal("removed, and still there")
	}
	if c3 := newAt(klipy.New(klipy.Config{}), path); c3.HasKlipyKey() {
		t.Fatal("removed, and back after a restart")
	}

	// The environment wins, and stays.
	t.Setenv("KLIPY_API_KEY", "from-env")
	c4 := newAt(klipy.New(klipy.Config{}), path)
	c4.SetKlipyKey("mine")
	if !c4.KlipyKeyFromEnvironment() || !c4.HasKlipyKey() {
		t.Fatal("the environment's key was lost")
	}
}

// Without a key the picker says where to put one; with one it does not.
func TestNoKeyNoticePointsToSettings(t *testing.T) {
	t.Setenv("KLIPY_API_KEY", "")
	c := newAt(klipy.New(klipy.Config{}), filepath.Join(t.TempDir(), "picker.json"))
	if !strings.Contains(noKeyNotice, "Settings → GIFs") {
		t.Fatal(noKeyNotice)
	}
	c.SetKlipyKey("abc")
	if !c.HasKlipyKey() {
		t.Fatal("no key after setting one")
	}
}

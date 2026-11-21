package matrix

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestLockSemantics checks what the session lock excludes, in-process:
// the second lockSession must fail while the first is held.
func TestLockSemantics(t *testing.T) {
	dir := t.TempDir()
	l1, err := lockSession(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.release()

	l2, err := lockSession(dir)
	if err == nil {
		l2.release()
		t.Fatal("a second lock was granted while the first was held")
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock: %v, want ErrLocked", err)
	}

	// The raw syscall, to know flock's own answer.
	f1, _ := os.OpenFile(filepath.Join(dir, "raw.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	defer f1.Close()
	if err = syscall.Flock(int(f1.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	f2, _ := os.OpenFile(filepath.Join(dir, "raw.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	defer f2.Close()
	if err = syscall.Flock(int(f2.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		t.Logf("flock: a second fd in the same process was granted (flock is per description, not per process)")
	} else {
		t.Logf("flock: a second fd in the same process was refused: %v", err)
	}
}

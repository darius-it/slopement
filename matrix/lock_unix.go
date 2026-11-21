//go:build !windows

package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// sessionLock is the account's flock on its directory: held for the
// process's life, so a second launch of the same account fails fast.
type sessionLock struct {
	f *os.File
}

func lockSession(dir string) (*sessionLock, error) {
	f, err := os.OpenFile(filepath.Join(dir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("%w: %v", ErrLocked, err)
	}
	return &sessionLock{f: f}, nil
}

func (l *sessionLock) release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	if err2 := l.f.Close(); err == nil {
		err = err2
	}
	return err
}

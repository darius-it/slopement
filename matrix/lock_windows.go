//go:build windows

package matrix

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// sessionLock is the account's lock on its directory, as the flock of the
// other systems: LockFileEx over the first byte of the lock file, held
// for the process's life, so a second launch of the same account fails
// fast.
type sessionLock struct {
	h windows.Handle
}

func lockSession(dir string) (*sessionLock, error) {
	p, err := windows.UTF16PtrFromString(filepath.Join(dir, lockName))
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	var ov windows.Overlapped
	if err = windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &ov); err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("%w: %v", ErrLocked, err)
	}
	return &sessionLock{h: h}, nil
}

func (l *sessionLock) release() error {
	if l == nil || l.h == 0 {
		return nil
	}
	var ov windows.Overlapped
	err := windows.UnlockFileEx(l.h, 0, 1, 0, &ov)
	if err2 := windows.CloseHandle(l.h); err == nil {
		err = err2
	}
	return err
}

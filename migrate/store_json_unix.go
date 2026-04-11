//go:build !windows

package migrate

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// unixLock wraps an *os.File holding an exclusive flock. Close() releases
// the flock and closes the file.
type unixLock struct {
	f *os.File
}

func (l *unixLock) Close() error {
	if l.f == nil {
		return nil
	}
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	err := l.f.Close()
	l.f = nil
	return err
}

// lockFile opens (or creates) path and takes an exclusive flock on it.
// Blocks until the lock is acquired. Caller must Close() to release.
func lockFile(path string) (io.Closer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &unixLock{f: f}, nil
}

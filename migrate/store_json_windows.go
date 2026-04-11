//go:build windows

package migrate

import (
	"io"
	"sync"
)

// WARN: on Windows the JSON store only synchronizes in-process writers via a
// sync.Mutex keyed on lockfile path. Multi-process access to the same state
// directory on Windows is UNSUPPORTED and will corrupt rows under contention.
// Single-process deployments (the common case) work correctly.

var winLockTab sync.Map // map[string]*sync.Mutex

type winLock struct {
	mu   *sync.Mutex
	held bool
}

func (l *winLock) Close() error {
	if l.held {
		l.mu.Unlock()
		l.held = false
	}
	return nil
}

func lockFile(path string) (io.Closer, error) {
	v, _ := winLockTab.LoadOrStore(path, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return &winLock{mu: mu, held: true}, nil
}

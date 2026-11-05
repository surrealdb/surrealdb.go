// Package wsutil holds what the WebSocket connections share.
package wsutil

import (
	"context"
	"errors"
	"net"
	"sync"
)

// WriteLock lets one write at a time onto a WebSocket, as the WebSocket
// libraries require, and lets a writer give up waiting for it when its context
// ends. The zero value is unlocked.
type WriteLock struct {
	once sync.Once
	ch   chan struct{}
}

// Lock waits for the lock until ctx ends, and returns ctx's error then.
func (l *WriteLock) Lock(ctx context.Context) error {
	l.once.Do(func() { l.ch = make(chan struct{}, 1) })
	select {
	case l.ch <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	// Both cases may have been ready.
	if err := ctx.Err(); err != nil {
		l.Unlock()
		return err
	}
	return nil
}

// Unlock releases the lock.
func (l *WriteLock) Unlock() {
	<-l.ch
}

// IsTimeout reports whether a write failed because its deadline passed. The
// WebSocket libraries do not all keep os.ErrDeadlineExceeded in the error.
func IsTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

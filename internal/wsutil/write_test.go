package wsutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteLock(t *testing.T) {
	var l WriteLock
	require.NoError(t, l.Lock(context.Background()), "the zero value is unlocked")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, l.Lock(ctx), context.DeadlineExceeded, "a second writer waits only as long as its context allows")
	l.Unlock()

	ended, end := context.WithCancel(context.Background())
	end()
	assert.ErrorIs(t, l.Lock(ended), context.Canceled, "an ended context does not take the lock, though it is free")

	require.NoError(t, l.Lock(context.Background()), "the lock is free again")
	l.Unlock()
}

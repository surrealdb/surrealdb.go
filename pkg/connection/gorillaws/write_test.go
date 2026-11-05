package gorillaws_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/internal/wstest"
)

// stuckPayload is larger than the socket buffers can hold, and does not
// compress, so that writing it to a server which reads nothing blocks.
func stuckPayload(t *testing.T) []byte {
	t.Helper()
	payload := make([]byte, 12<<20)
	_, err := rand.Read(payload)
	require.NoError(t, err)
	return payload
}

// A write which the server stops taking used to hold the write lock with no
// deadline, so every later request, and Close, waited for it past their own.
func TestWrite_stuckWriteHoldsUpNothingElse(t *testing.T) {
	srv := wstest.NewStallingServer(t)
	c := connect(t, srv)
	c.Timeout = 0

	stuck := make(chan error, 1)
	go func() {
		_, err := c.Send(context.Background(), "let", "big", stuckPayload(t))
		stuck <- err
	}()
	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Send(ctx, "ping")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second, "the request waits no longer than its context allows")

	closeCtx, cancelClose := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelClose()
	start = time.Now()
	_ = c.Close(closeCtx)
	assert.Less(t, time.Since(start), 2*time.Second, "Close does not wait for the stuck write")

	err = wstest.Recv(t, stuck, "the stuck write to end")
	assert.Error(t, err, "closing ends the stuck write")
}

// A write which the server stops taking ends when its context does, and closes
// the connection, as the message cut short leaves the stream broken.
func TestWrite_stuckWriteEndsWithItsContext(t *testing.T) {
	srv := wstest.NewStallingServer(t)
	c := connect(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Send(ctx, "let", "big", stuckPayload(t))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second)
	wstest.Recv(t, c.Closed(), "the connection to close")

	ctx, cancel = context.WithCancel(context.Background())
	c = connect(t, srv)
	c.Timeout = 0
	stuck := make(chan error, 1)
	go func() {
		_, err := c.Send(ctx, "let", "big", stuckPayload(t))
		stuck <- err
	}()
	time.Sleep(300 * time.Millisecond)
	cancel()
	assert.ErrorIs(t, wstest.Recv(t, stuck, "the canceled write to end"), context.Canceled)
	wstest.Recv(t, c.Closed(), "the connection to close")
}

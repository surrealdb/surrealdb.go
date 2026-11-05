package gws

import (
	"context"
	"crypto/rand"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/internal/wstest"
	"github.com/surrealdb/surrealdb.go/v2/pkg/connection"
)

// stuckPayload does not compress, and is far larger than the socket buffers of
// a connection from connectStalled, so that writing it there blocks.
func stuckPayload(t *testing.T) []byte {
	t.Helper()
	payload := make([]byte, 4<<20)
	_, err := rand.Read(payload)
	require.NoError(t, err)
	return payload
}

// connectStalled connects to a server which reads nothing, with a small send
// buffer, so that a large write blocks whatever the system's buffer sizes. The
// send buffer is the connection's own, which only this package can reach.
func connectStalled(t *testing.T) *Connection {
	t.Helper()
	srv := wstest.NewStallingServer(t)
	u, err := url.ParseRequestURI(srv.WSURL())
	require.NoError(t, err)
	c := New(connection.NewConfig(u))
	require.NoError(t, c.Connect(context.Background()))
	srv.WaitAccepted(t, 1)
	t.Cleanup(func() { _ = c.Close(context.Background()) })

	tcp, ok := c.conn.NetConn().(*net.TCPConn)
	require.True(t, ok)
	require.NoError(t, tcp.SetWriteBuffer(16<<10))
	return c
}

// A write which the server stops taking used to hold the write lock with no
// deadline, so that every later request, and Close, waited for it past their
// own deadlines.
func TestWrite_stuckWriteHoldsUpNothingElse(t *testing.T) {
	c := connectStalled(t)
	c.Timeout = 0

	payload := stuckPayload(t)
	stuck := make(chan error, 1)
	go func() {
		_, err := c.Send(context.Background(), "let", "big", payload)
		stuck <- err
	}()
	time.Sleep(time.Second)

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

	assert.Error(t, wstest.Recv(t, stuck, "the stuck write to end"), "closing ends the stuck write")
}

// A write which the server stops taking ends once the connection's Timeout
// has passed, and closes the connection, as the other end has stopped reading.
func TestWrite_stuckWriteEndsAtTheTimeout(t *testing.T) {
	c := connectStalled(t)
	c.Timeout = 500 * time.Millisecond

	payload := stuckPayload(t)
	start := time.Now()
	_, err := c.Send(context.Background(), "let", "big", payload)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 3*time.Second)
	wstest.Recv(t, c.Closed(), "the connection to close")
}

// A request whose context ends while its write is under way does not stop the
// write: gws closes the connection, which every request shares, when a write
// fails.
func TestWrite_contextEndingMidWriteKeepsTheConnection(t *testing.T) {
	c := connectStalled(t)
	c.Timeout = 0

	payload := stuckPayload(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.Send(ctx, "let", "big", payload)
		done <- err
	}()

	select {
	case <-c.Closed():
		t.Fatal("the connection closed as the request's context ended")
	case err := <-done:
		t.Fatalf("the write was stopped: %v", err)
	case <-time.After(1500 * time.Millisecond):
	}

	require.NoError(t, c.Close(context.Background()))
	assert.Error(t, wstest.Recv(t, done, "the write to end"), "closing ends the write")
}

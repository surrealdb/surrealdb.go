package gorillaws_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/internal/wstest"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
)

func connect(t *testing.T, srv *wstest.Server) *gorillaws.Connection {
	t.Helper()
	u, err := url.ParseRequestURI(srv.WSURL())
	require.NoError(t, err)
	c := gorillaws.New(connection.NewConfig(u))
	accepted := srv.Accepted()
	require.NoError(t, c.Connect(context.Background()))
	// GoAway closes only the connections the server has recorded, which it
	// may not have done yet when Connect returns.
	srv.WaitAccepted(t, accepted+1)
	return c
}

// A close frame from the server used to leave the read loop waiting forever,
// so the connection never noticed it was closed.
func TestServerClose_failsCallsInFlight(t *testing.T) {
	srv := wstest.NewServer(t)
	c := connect(t, srv)

	errc := make(chan error, 1)
	go func() {
		_, err := c.Send(context.Background(), "query", "SLEEP 1h")
		errc <- err
	}()
	wstest.Recv(t, srv.Requests, "the request")
	srv.GoAway()

	select {
	case err := <-errc:
		assert.ErrorIs(t, err, connection.ErrConnectionClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("the call in flight did not fail when the server closed the connection")
	}

	select {
	case <-c.Closed():
	case <-time.After(5 * time.Second):
		t.Fatal("Closed did not report the close")
	}
	assert.True(t, c.IsClosed())
	assert.Error(t, c.CloseError())

	_, err := c.Send(context.Background(), "ping")
	assert.ErrorIs(t, err, connection.ErrConnectionClosed)
}

func TestClose_failsLaterCalls(t *testing.T) {
	srv := wstest.NewServer(t)
	c := connect(t, srv)
	require.NoError(t, c.Close(context.Background()))

	wstest.Recv(t, c.Closed(), "the connection to close")
	assert.NoError(t, c.CloseError(), "Close is not an error")
	_, err := c.Send(context.Background(), "ping")
	assert.ErrorIs(t, err, connection.ErrConnectionClosed)
}

func TestRedial(t *testing.T) {
	srv := wstest.NewServer(t)
	c := connect(t, srv)
	c.Timeout = 3 * time.Second
	srv.GoAway()
	wstest.Recv(t, c.Closed(), "the connection to close")

	next, ok := c.Redial().(*gorillaws.Connection)
	require.True(t, ok)
	assert.Equal(t, c.Timeout, next.Timeout)
	assert.False(t, next.IsClosed())
	require.NoError(t, next.Connect(context.Background()))
	defer next.Close(context.Background())

	go func() { _, _ = next.Send(context.Background(), "ping") }()
	select {
	case <-srv.Requests:
	case <-time.After(5 * time.Second):
		t.Fatal("the new connection sent nothing")
	}
}

// The server answers Close's close frame, which ends the read loop; Close must
// not then fail on a socket the read loop closed first.
func TestClose_racesTheServersAnswer(t *testing.T) {
	srv := wstest.NewServer(t)
	for range 50 {
		c := connect(t, srv)
		require.NoError(t, c.Close(context.Background()))
	}
}

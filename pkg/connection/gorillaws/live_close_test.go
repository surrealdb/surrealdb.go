package gorillaws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// newDroppingServer starts a WebSocket server which, once drop is called,
// ends every connection: with a close frame if closeFrame is set, otherwise
// by closing the TCP connection without one.
func newDroppingServer(t *testing.T, closeFrame bool) (u *url.URL, drop func()) {
	t.Helper()
	dropCh := make(chan struct{})
	upgrader := gorilla.Upgrader{Subprotocols: []string{"cbor"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-dropCh
		if closeFrame {
			_ = conn.WriteControl(gorilla.CloseMessage,
				gorilla.FormatCloseMessage(gorilla.CloseGoingAway, "shutting down"),
				time.Now().Add(time.Second))
		}
		_ = conn.UnderlyingConn().Close()
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse("ws" + srv.URL[len("http"):])
	require.NoError(t, err)
	var once bool
	return u, func() {
		if !once {
			once = true
			close(dropCh)
		}
	}
}

func requireClosed(t *testing.T, ch chan connection.Notification) {
	t.Helper()
	select {
	case _, ok := <-ch:
		require.False(t, ok, "no notification was sent")
	case <-time.After(5 * time.Second):
		t.Fatal("the live query's channel was not closed")
	}
}

// TestLiveNotifications_endWithConnection checks that the channels of live
// queries are closed, and their goroutines stopped, when the connection is
// closed or lost. They used to stay open for ever.
func TestLiveNotifications_endWithConnection(t *testing.T) {
	cases := []struct {
		name       string
		closeFrame bool
		end        func(c *Connection, drop func())
	}{
		{"closed", false, func(c *Connection, _ func()) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = c.Close(ctx)
		}},
		{"lost", false, func(_ *Connection, drop func()) { drop() }},
		{"closed by the server", true, func(_ *Connection, drop func()) { drop() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, drop := newDroppingServer(t, tc.closeFrame)
			c := New(connection.NewConfig(u))
			require.NoError(t, c.Connect(context.Background()))
			t.Cleanup(func() {
				drop()
				_ = c.Close(context.Background())
			})

			ch, err := c.LiveNotifications("lq")
			require.NoError(t, err)

			tc.end(c, drop)

			requireClosed(t, ch)
			assert.Eventually(t, c.IsClosed, 5*time.Second, 10*time.Millisecond)
			_, err = c.LiveNotifications("another")
			assert.ErrorIs(t, err, connection.ErrLiveNotificationsClosed)
			assert.NoError(t, c.CloseLiveNotifications("lq"))
		})
	}
}

// TestLiveNotifications_lostConnectionDeliversQueued loses the connection
// while a live query's consumer is far behind. What was received before the
// loss is still delivered, then the channel is closed; it used to be dropped
// past the channel's buffer. Close stops the delivery at once.
func TestLiveNotifications_lostConnectionDeliversQueued(t *testing.T) {
	const queued = 200
	for _, closeAfter := range []bool{false, true} {
		name := "lost"
		if closeAfter {
			name = "lost then closed"
		}
		t.Run(name, func(t *testing.T) {
			u, drop := newDroppingServer(t, false)
			c := New(connection.NewConfig(u))
			require.NoError(t, c.Connect(context.Background()))
			t.Cleanup(func() {
				drop()
				_ = c.Close(context.Background())
			})

			ch, err := c.LiveNotifications("lq")
			require.NoError(t, err)
			for i := 0; i < queued; i++ {
				require.True(t, c.SendNotification("lq", connection.Notification{Action: connection.CreateAction, Result: i}))
			}

			drop()
			require.Eventually(t, c.IsClosed, 5*time.Second, 10*time.Millisecond)

			if closeAfter {
				require.NoError(t, c.Close(context.Background()))
				for range ch {
				}
				return
			}
			for i := 0; i < queued; i++ {
				select {
				case n, ok := <-ch:
					require.True(t, ok, "notification %d was dropped", i)
					require.Equal(t, i, n.Result)
				case <-time.After(5 * time.Second):
					t.Fatalf("timed out waiting for notification %d", i)
				}
			}
			requireClosed(t, ch)
		})
	}
}

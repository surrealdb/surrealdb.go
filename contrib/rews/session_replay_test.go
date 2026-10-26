package rews

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// recordingConnection is a socket that records the session RPCs it is sent,
// and can hold one of them until it is released.
type recordingConnection struct {
	connection.WebSocketConnection

	mu     sync.Mutex
	calls  []string
	closed bool

	// blockMethod names the RPC whose first call waits for release before
	// returning; entered is closed once it has been called.
	blockMethod string
	blocked     bool
	entered     chan struct{}
	release     chan struct{}

	// failMethods names the RPCs that fail after being recorded.
	failMethods []string
	// tokens, if set, is the pair the refresh variants return.
	tokens *connection.Tokens
}

func (c *recordingConnection) fails(method string) error {
	for _, m := range c.failMethods {
		if m == method {
			return fmt.Errorf("%s failed", method)
		}
	}
	return nil
}

func (c *recordingConnection) pair() *connection.Tokens {
	if c.tokens != nil {
		return c.tokens
	}
	return &connection.Tokens{Access: "token", Refresh: "refresh"}
}

func (c *recordingConnection) record(ctx context.Context, call string) error {
	c.mu.Lock()
	c.calls = append(c.calls, call)
	block := !c.blocked && c.blockMethod != "" && strings.HasPrefix(call, c.blockMethod)
	if block {
		c.blocked = true
	}
	c.mu.Unlock()

	if !block {
		return nil
	}
	close(c.entered)
	// Like a socket waiting for a response that will never come, give up only
	// when the context is done.
	select {
	case <-c.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *recordingConnection) Calls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func (c *recordingConnection) Connect(context.Context) error { return nil }
func (c *recordingConnection) Close(context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}
func (c *recordingConnection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
func (c *recordingConnection) Use(ctx context.Context, ns, db string) error {
	return c.record(ctx, fmt.Sprintf("use %s %s", ns, db))
}
func (c *recordingConnection) Authenticate(ctx context.Context, token string) error {
	if err := c.record(ctx, "authenticate "+token); err != nil {
		return err
	}
	return c.fails("authenticate")
}
func (c *recordingConnection) Invalidate(ctx context.Context) error {
	if err := c.record(ctx, "invalidate"); err != nil {
		return err
	}
	return c.fails("invalidate")
}
func (c *recordingConnection) Let(ctx context.Context, key string, value any) error {
	return c.record(ctx, fmt.Sprintf("let %s %v", key, value))
}
func (c *recordingConnection) Unset(ctx context.Context, key string) error {
	return c.record(ctx, "unset "+key)
}
func (c *recordingConnection) SignIn(ctx context.Context, _ any) (string, error) {
	if err := c.record(ctx, "signin"); err != nil {
		return "", err
	}
	return "token", nil
}
func (c *recordingConnection) SignUp(ctx context.Context, _ any) (string, error) {
	if err := c.record(ctx, "signup"); err != nil {
		return "", err
	}
	return "token", nil
}
func (c *recordingConnection) SignInWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	call := "signin"
	if m, ok := authData.(map[string]any); ok && m["refresh"] != nil {
		call = fmt.Sprintf("signin refresh %v", m["refresh"])
	}
	if err := c.record(ctx, call); err != nil {
		return nil, err
	}
	return c.pair(), nil
}
func (c *recordingConnection) SignUpWithRefresh(ctx context.Context, _ any) (*connection.Tokens, error) {
	if err := c.record(ctx, "signup"); err != nil {
		return nil, err
	}
	return c.pair(), nil
}

// TestInvalidate_failureStillForgetsTokens checks that a reconnection after an
// Invalidate whose RPC failed, as it does on a dropped socket, does not sign the
// session back in.
func TestInvalidate_failureStillForgetsTokens(t *testing.T) {
	ctx := context.Background()
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	next := &recordingConnection{}
	conn := New(func(context.Context) (*recordingConnection, error) {
		return next, nil
	}, 0, surrealcbor.New(), log)
	old := &recordingConnection{failMethods: []string{"invalidate"}}
	conn.WebSocketConnection = old
	conn.state = StateConnected

	require.NoError(t, conn.Use(ctx, "ns", "db"))
	require.NoError(t, conn.Authenticate(ctx, "token"))
	require.Error(t, conn.Invalidate(ctx))

	require.NoError(t, conn.reconnect(ctx))
	require.Equal(t, []string{"use ns db"}, next.Calls())
}

// TestReconnect_refreshTellsTheCaller checks that when a reconnection signs in
// with the refresh token, which the server then revokes, the caller can learn
// the new pair, from OnTokensRefreshed and from Tokens.
func TestReconnect_refreshTellsTheCaller(t *testing.T) {
	ctx := context.Background()
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// The new socket refuses the expired access token, and gives a new pair
	// for the refresh token.
	next := &recordingConnection{
		failMethods: []string{"authenticate"},
		tokens:      &connection.Tokens{Access: "token2", Refresh: "refresh2"},
	}
	conn := New(func(context.Context) (*recordingConnection, error) {
		return next, nil
	}, 0, surrealcbor.New(), log)
	conn.WebSocketConnection = &recordingConnection{}
	conn.state = StateConnected

	var got []connection.Tokens
	conn.OnTokensRefreshed = func(pair connection.Tokens) {
		// The callback runs without the session lock, so it can read Tokens.
		waitCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		current, err := conn.Tokens(waitCtx)
		require.NoError(t, err)
		require.Equal(t, pair, current)
		got = append(got, pair)
	}

	pair, err := conn.SignInWithRefresh(ctx, map[string]any{"NS": "ns", "DB": "db", "AC": "ac"})
	require.NoError(t, err)
	require.Equal(t, connection.Tokens{Access: "token", Refresh: "refresh"}, *pair)

	require.NoError(t, conn.reconnect(ctx))
	require.Equal(t, []string{"authenticate token", "signin refresh refresh"}, next.Calls())
	require.Equal(t, []connection.Tokens{{Access: "token2", Refresh: "refresh2"}}, got)

	tokens, err := conn.Tokens(ctx)
	require.NoError(t, err)
	require.Equal(t, connection.Tokens{Access: "token2", Refresh: "refresh2"}, tokens)
}

// TestReconnect_serialisesReplayWithSessionChanges checks that a session change
// which completes while a reconnection is replaying the session is not undone
// by the replay applying the state from before the change.
func TestReconnect_serialisesReplayWithSessionChanges(t *testing.T) {
	tests := []struct {
		name string
		// block is the RPC on the old socket that is still in flight when the
		// reconnection starts.
		block  string
		change func(ctx context.Context, c *Connection[*recordingConnection]) error
		// want is what the replay must send to the new socket.
		want []string
	}{
		{
			name:   "Invalidate",
			block:  "invalidate",
			change: func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Invalidate(ctx) },
			want:   []string{"use ns db", "let k v1"},
		},
		{
			name:   "Use",
			block:  "use",
			change: func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Use(ctx, "ns2", "db2") },
			want:   []string{"use ns2 db2", "authenticate token", "let k v1"},
		},
		{
			name:   "Let",
			block:  "let",
			change: func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Let(ctx, "k", "v2") },
			want:   []string{"use ns db", "authenticate token", "let k v2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

			next := &recordingConnection{}
			conn := New(func(context.Context) (*recordingConnection, error) {
				return next, nil
			}, 0, surrealcbor.New(), log)

			old := &recordingConnection{}
			conn.WebSocketConnection = old
			conn.state = StateConnected

			require.NoError(t, conn.Use(ctx, "ns", "db"))
			require.NoError(t, conn.Authenticate(ctx, "token"))
			require.NoError(t, conn.Let(ctx, "k", "v1"))

			// Hold the session change in flight on the old socket.
			old.blockMethod = tt.block
			old.entered = make(chan struct{})
			old.release = make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(old.release) }) }
			t.Cleanup(release)
			changed := make(chan error, 1)
			go func() { changed <- tt.change(ctx, conn) }()
			<-old.entered

			// Reconnect while it is in flight. The replay must wait for it
			// rather than replay the state from before it.
			reconnected := make(chan error, 1)
			go func() { reconnected <- conn.reconnect(ctx) }()
			select {
			case err := <-reconnected:
				require.NoError(t, err)
				t.Fatalf("the replay finished while a session change was in flight, sending %q", next.Calls())
			case <-time.After(200 * time.Millisecond):
			}

			release()
			require.NoError(t, <-changed)
			require.NoError(t, <-reconnected)

			require.Equal(t, tt.want, next.Calls())
		})
	}
}

// TestSessionChange_honorsContextWhileWaiting checks that a session change
// waiting for another one to finish gives up when its context is done, rather
// than waiting for as long as the other one takes.
func TestSessionChange_honorsContextWhileWaiting(t *testing.T) {
	changes := map[string]func(ctx context.Context, c *Connection[*recordingConnection]) error{
		"Use":          func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Use(ctx, "ns2", "db2") },
		"Let":          func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Let(ctx, "k", "v") },
		"Unset":        func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Unset(ctx, "k") },
		"Authenticate": func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Authenticate(ctx, "t") },
		"Invalidate":   func(ctx context.Context, c *Connection[*recordingConnection]) error { return c.Invalidate(ctx) },
		"SignIn": func(ctx context.Context, c *Connection[*recordingConnection]) error {
			_, err := c.SignIn(ctx, map[string]any{})
			return err
		},
		"SignUp": func(ctx context.Context, c *Connection[*recordingConnection]) error {
			_, err := c.SignUp(ctx, map[string]any{})
			return err
		},
		"SignInWithRefresh": func(ctx context.Context, c *Connection[*recordingConnection]) error {
			_, err := c.SignInWithRefresh(ctx, map[string]any{})
			return err
		},
		"SignUpWithRefresh": func(ctx context.Context, c *Connection[*recordingConnection]) error {
			_, err := c.SignUpWithRefresh(ctx, map[string]any{})
			return err
		},
	}

	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			conn := New(func(context.Context) (*recordingConnection, error) {
				return &recordingConnection{}, nil
			}, 0, surrealcbor.New(), log)

			// Hold the session state with a Use that does not return.
			old := &recordingConnection{blockMethod: "use", entered: make(chan struct{}), release: make(chan struct{})}
			conn.WebSocketConnection = old
			conn.state = StateConnected
			t.Cleanup(func() { close(old.release) })
			go func() { _ = conn.Use(context.Background(), "ns", "db") }()
			<-old.entered

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- change(ctx, conn) }()

			select {
			case err := <-done:
				require.ErrorIs(t, err, context.DeadlineExceeded)
			case <-time.After(2 * time.Second):
				t.Fatal("the session change kept waiting after its context was done")
			}
		})
	}
}

// TestReconnect_doesNotWaitForAbandonedRPCs checks that a session RPC sent
// with a context that never ends on a socket that then drops, whose response
// will never come, is given up, so that the reconnection replay waiting for
// the session state can run.
func TestReconnect_doesNotWaitForAbandonedRPCs(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	next := &recordingConnection{}
	conn := New(func(context.Context) (*recordingConnection, error) {
		return next, nil
	}, 10*time.Millisecond, surrealcbor.New(), log)

	old := &recordingConnection{blockMethod: "use", entered: make(chan struct{}), release: make(chan struct{})}
	conn.WebSocketConnection = old
	conn.state = StateConnected
	t.Cleanup(func() { close(old.release) })

	used := make(chan error, 1)
	go func() { used <- conn.Use(context.Background(), "ns", "db") }()
	<-old.entered
	require.NoError(t, old.Close(context.Background()))

	reconnected := make(chan error, 1)
	go func() { reconnected <- conn.reconnect(context.Background()) }()
	select {
	case err := <-reconnected:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("the replay is still waiting for a session RPC abandoned by the dropped socket")
	}

	err := <-used
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "the socket the request was sent on closed")
}

// TestReconnect_givesUpReplayWhenSocketDrops checks that a replay whose new
// socket drops before answering gives up, releasing the session state, rather
// than wait for ever.
func TestReconnect_givesUpReplayWhenSocketDrops(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	next := &recordingConnection{blockMethod: "use", entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(next.release) })
	conn := New(func(context.Context) (*recordingConnection, error) {
		return next, nil
	}, 10*time.Millisecond, surrealcbor.New(), log)
	conn.WebSocketConnection = &recordingConnection{}
	conn.state = StateConnected
	require.NoError(t, conn.Use(context.Background(), "ns", "db"))

	reconnected := make(chan error, 1)
	go func() { reconnected <- conn.reconnect(context.Background()) }()
	<-next.entered
	require.NoError(t, next.Close(context.Background()))

	select {
	case err := <-reconnected:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("the replay kept waiting on a socket that dropped")
	}
}

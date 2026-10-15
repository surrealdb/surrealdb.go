package rews

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// TestSessionState_concurrentReplay changes the session state from several
// goroutines while reconnections replay it. Run with -race: the session map and
// fields used to be read and written without a lock.
func TestSessionState_concurrentReplay(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	newMock := func() *mockWebSocketConnection {
		return &mockWebSocketConnection{notifications: make(map[string]chan connection.Notification)}
	}

	// Each reconnection gets a socket of its own, as it does for real.
	conn := New(func(ctx context.Context) (*mockWebSocketConnection, error) {
		return newMock(), nil
	}, 0, surrealcbor.New(), log)
	conn.WebSocketConnection = newMock()
	conn.state = StateConnected

	ctx := context.Background()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("k%d_%d", g, i)
				require.NoError(t, conn.Let(ctx, key, i))
				require.NoError(t, conn.Use(ctx, "ns", key))
				require.NoError(t, conn.Authenticate(ctx, key))
				if i%2 == 0 {
					require.NoError(t, conn.Unset(ctx, key))
				}
			}
		}(g)
	}

	for i := 0; i < 20; i++ {
		// Replays the session state as a reconnection does.
		require.NoError(t, conn.reconnect(ctx))
	}
	wg.Wait()
}

func TestAccessOf(t *testing.T) {
	type auth struct {
		Namespace string `json:"NS,omitempty"`
		Database  string `json:"DB,omitempty"`
		Access    string `json:"AC,omitempty"`
		Username  string `json:"user,omitempty"`
		Password  string `json:"pass,omitempty"`
	}

	want := map[string]any{"NS": "n", "DB": "d", "AC": "a"}
	require.Equal(t, want, accessOf(map[string]any{"NS": "n", "DB": "d", "AC": "a", "user": "u", "pass": "p"}))
	require.Equal(t, want, accessOf(map[string]string{"NS": "n", "DB": "d", "AC": "a", "pass": "p"}))
	require.Equal(t, want, accessOf(auth{Namespace: "n", Database: "d", Access: "a", Username: "u", Password: "p"}))
	require.Equal(t, want, accessOf(&auth{Namespace: "n", Database: "d", Access: "a"}))
	require.Equal(t, map[string]any{"namespace": "n"}, accessOf(map[string]any{"namespace": "n", "refresh": "r"}))
	require.Empty(t, accessOf("nonsense"))
	require.Empty(t, accessOf(nil))
	require.Empty(t, accessOf((*auth)(nil)))
	require.Empty(t, accessOf(map[int]any{1: "n"}))
}

// TestAccessOf_namedMapTypes checks that maps of a named type, which a type
// switch on map[string]any does not match, still yield the access fields.
func TestAccessOf_namedMapTypes(t *testing.T) {
	type credentials map[string]any
	type stringCredentials map[string]string
	type key string
	type keyedCredentials map[key]any

	want := map[string]any{"NS": "n", "DB": "d", "AC": "a"}
	require.Equal(t, want, accessOf(credentials{"NS": "n", "DB": "d", "AC": "a", "user": "u", "pass": "p"}))
	require.Equal(t, want, accessOf(&credentials{"NS": "n", "DB": "d", "AC": "a"}))
	require.Equal(t, want, accessOf(stringCredentials{"NS": "n", "DB": "d", "AC": "a", "pass": "p"}))
	require.Equal(t, want, accessOf(keyedCredentials{"NS": "n", "DB": "d", "AC": "a", "pass": "p"}))
}

// TestAccessOf_embeddedStructs checks that access fields promoted from an
// embedded struct, as the encoder promotes them, are found.
func TestAccessOf_embeddedStructs(t *testing.T) {
	type auth struct {
		Namespace string `json:"NS,omitempty"`
		Database  string `json:"DB,omitempty"`
		Access    string `json:"AC,omitempty"`
		Username  string `json:"user,omitempty"`
		Password  string `json:"pass,omitempty"`
	}
	type embedded struct {
		auth
		Extra string `json:"extra"`
	}
	type embeddedPointer struct {
		*auth
	}
	type twoLevels struct {
		embedded
	}
	type shadowed struct {
		auth
		Namespace string `json:"NS"`
	}
	type named struct {
		auth `json:"creds"`
	}

	creds := auth{Namespace: "n", Database: "d", Access: "a", Username: "u", Password: "p"}
	want := map[string]any{"NS": "n", "DB": "d", "AC": "a"}

	require.Equal(t, want, accessOf(embedded{auth: creds, Extra: "x"}))
	require.Equal(t, want, accessOf(&embedded{auth: creds}))
	require.Equal(t, want, accessOf(embeddedPointer{auth: &creds}))
	require.Equal(t, want, accessOf(twoLevels{embedded{auth: creds}}))
	require.Empty(t, accessOf(embeddedPointer{}))

	// A field of the outer struct hides one of the same name it embeds.
	require.Equal(t, map[string]any{"NS": "outer", "DB": "d", "AC": "a"},
		accessOf(shadowed{auth: creds, Namespace: "outer"}))
	// An embedded struct with a tag name is encoded as a field, not promoted.
	require.Empty(t, accessOf(named{auth: creds}))
}

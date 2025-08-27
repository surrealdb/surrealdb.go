package rews_test

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/internal/fakesdb"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const testTokenSignIn = "test_token_signin"

// TestIntegration tests the reconnection mechanism for live queries using fakesdb
func TestIntegration(t *testing.T) {
	server := fakesdb.NewServer("127.0.0.1:0")
	server.TokenSignIn = testTokenSignIn

	var liveCallCount int32
	var queryCallCount int32
	var selectCount int32
	var retryRequired bool

	// Track live method calls
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("live", func(params []any) bool {
			count := atomic.AddInt32(&liveCallCount, 1)
			t.Logf("Live query request #%d", count)
			return true
		}),
		Result: models.UUID{UUID: uuid.Must(uuid.NewV4())},
	})

	// Track query method calls
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("query", func(params []any) bool {
			if len(params) > 0 {
				if query, ok := params[0].(string); ok {
					if strings.Contains(strings.ToUpper(query), "LIVE SELECT") {
						count := atomic.AddInt32(&queryCallCount, 1)
						t.Logf("LIVE SELECT query request #%d", count)
						return true
					}
				}
			}
			return false
		}),
		Result: []surrealdb.QueryResult[models.UUID]{
			{
				Status: "OK",
				Time:   "1ms",
				Result: models.UUID{UUID: uuid.Must(uuid.NewV4())},
			},
		},
	})

	// Drop connection on 3rd select to trigger reconnection
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("select", func(params []any) bool {
			count := atomic.AddInt32(&selectCount, 1)
			t.Logf("Select request #%d", count)

			if count == 3 && !retryRequired {
				retryRequired = true
				t.Log("*** Dropping connection on 3rd select request ***")
				return true
			}
			return false
		}),
		Result: nil,
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureWebSocketClose,
				Probability: 1.0,
				CloseCode:   1001,
				CloseReason: "Testing reconnection",
			},
		},
	})

	// Normal select response
	server.AddStubResponse(fakesdb.SimpleStubResponse("select", map[string]any{
		"id":   cbor.Tag{Number: 8, Content: []any{"test", "1"}},
		"name": "Test Record",
	}))

	// Other necessary stubs
	server.AddStubResponse(fakesdb.SimpleStubResponse("use", nil))
	server.AddStubResponse(fakesdb.SimpleStubResponse("signin", map[string]any{
		"token": testTokenSignIn,
	}))
	server.AddStubResponse(fakesdb.SimpleStubResponse("authenticate", nil))

	// Start server
	err := server.Start()
	require.NoError(t, err)
	defer func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Fatalf("Failed to stop server: %v", stopErr)
		}
	}()

	wsURL := "ws://" + server.Address()
	checkInterval := 100 * time.Millisecond

	u, err := url.ParseRequestURI(wsURL)
	require.NoError(t, err)
	config := connection.NewConfig(u)

	conn := rews.New(func(ctx context.Context) (*gorillaws.Connection, error) {
		ws := gorillaws.New(config)
		return ws, nil
	}, checkInterval, config.Unmarshaler, logger.New(slog.NewTextHandler(os.Stdout, nil)))

	ctx := context.Background()

	// Establish initial connection
	err = conn.Connect(ctx)
	require.NoError(t, err, "Initial connection should succeed")

	// The session would contain the following data, which will be restored on reconnection
	// - Used namespace and database names
	// - The token returned by the successful SignIn
	// - The variables set via Let

	err = conn.Use(ctx, "test", "test")
	require.NoError(t, err)

	token, signInErr := conn.SignIn(ctx, map[string]any{
		"username": "root",
		"password": "root",
	})
	require.NoError(t, signInErr)
	require.Equal(t, testTokenSignIn, token)

	err = conn.Authenticate(ctx, token)
	require.NoError(t, err)

	err = conn.Let(ctx, "x", 1)
	require.NoError(t, err)

	// Send live queries before disconnection
	_, _ = conn.Send(ctx, "live", "users", false)
	initialLiveCount := atomic.LoadInt32(&liveCallCount)
	t.Logf("Initial live query count: %d", initialLiveCount)

	_, _ = conn.Send(ctx, "query", "LIVE SELECT * FROM products", nil)
	initialQueryCount := atomic.LoadInt32(&queryCallCount)
	t.Logf("Initial query count: %d", initialQueryCount)

	// Make selects to trigger disconnection
	for i := 0; i < 2; i++ {
		_, _ = conn.Send(ctx, "select", "test", "1")
	}

	// Third select will trigger disconnection
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err = conn.Send(shortCtx, "select", "test", "1")
	require.Error(t, err, "3rd select should fail due to connection drop")
	cancel()

	// Wait for reconnection
	time.Sleep(1 * time.Second)

	// Verify connection is working after reconnection
	_, err = conn.Send(ctx, "select", "test", "1")
	if err != nil {
		// Try again after a bit more wait
		time.Sleep(500 * time.Millisecond)
		_, err = conn.Send(ctx, "select", "test", "1")
	}
	assert.NoError(t, err, "Connection should work after reconnection")

	// After reconnection, check if more live/query calls were made
	// This would indicate the live queries were restored
	finalLiveCount := atomic.LoadInt32(&liveCallCount)
	finalQueryCount := atomic.LoadInt32(&queryCallCount)

	t.Logf("Final live query count: %d", finalLiveCount)
	t.Logf("Final query count: %d", finalQueryCount)

	// The counts should increase if restoration is working
	// We can't guarantee exact numbers due to timing, but there should be attempts
	if finalLiveCount > initialLiveCount || finalQueryCount > initialQueryCount {
		t.Log("Live query restoration attempted (counts increased)")
	}
	require.Greater(t, finalLiveCount, initialLiveCount, "Live query should have been re-sent after reconnection")
	require.Greater(t, finalQueryCount, initialQueryCount, "LIVE SELECT query should have been re-sent after reconnection")

	t.Log("Test passed: Reconnection mechanism tested")
}

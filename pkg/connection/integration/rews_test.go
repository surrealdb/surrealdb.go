package integration

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/internal/fakesdb"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rews"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestRewsGorillaWsDoReconnect(t *testing.T) {
	testDoReconnect(t, func(wsURL string) func(context.Context) (*gorillaws.Connection, error) {
		return func(ctx context.Context) (*gorillaws.Connection, error) {
			p, err := surrealdb.Configure(wsURL)
			if err != nil {
				return nil, err
			}
			ws := gorillaws.New(p)

			if err := ws.Connect(ctx); err != nil {
				return nil, err
			}
			return ws, nil
		}
	})
}

func TestRewsGwsDoReconnect(t *testing.T) {
	testDoReconnect(t, func(wsURL string) func(context.Context) (*gws.Connection, error) {
		return func(ctx context.Context) (*gws.Connection, error) {
			p, err := surrealdb.Configure(wsURL)
			if err != nil {
				return nil, err
			}
			ws := gws.New(p)
			ws.Logger = logger.New(slog.NewTextHandler(os.Stdout, nil))

			if err := ws.Connect(ctx); err != nil {
				return nil, err
			}
			return ws, nil
		}
	})
}

// testDoReconnect tests the auto-reconnection feature
// against the WebSocket connection implementation provided by connectFunc.
//
// It simulates a connection drop and verifies that the rews package
// automatically reconnects and resumes operations,
// with any underlying connection implementation that supports the WebSocketConnection interface.
func testDoReconnect[C connection.WebSocketConnection](t *testing.T, connectFunc func(wsURL string) func(context.Context) (C, error)) {
	t.Helper()

	server := fakesdb.NewServer("127.0.0.1:0")
	server.TokenSignIn = "test_token_signin"

	var selectCount int32
	var retryRequired bool

	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("select", func(params []interface{}) bool {
			count := atomic.AddInt32(&selectCount, 1)
			t.Logf("Select request #%d", count)

			// Drop connection on 3rd select request only once
			if count == 3 && !retryRequired {
				retryRequired = true
				t.Log("*** Dropping connection on 3rd request ***")
				return true
			}
			return false
		}),
		Response: nil,
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
	server.AddStubResponse(fakesdb.SimpleStubResponse("select", map[string]interface{}{
		"id":   cbor.Tag{Number: 8, Content: []interface{}{"test", "1"}},
		"name": "Test Record",
	}))

	// Start server
	err := server.Start()
	require.NoError(t, err)
	defer func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Fatalf("Failed to stop server: %v", stopErr)
		}
	}()

	// Configure connection with auto-reconnection
	wsURL := "ws://" + server.Address()

	// Short reconnection check interval for testing
	checkInterval := 100 * time.Millisecond

	// Create auto-reconnecting connection
	conn := rews.New(connectFunc(wsURL), checkInterval, logger.New(slog.NewTextHandler(os.Stdout, nil)))

	// Initial connection
	err = conn.Connect(context.Background())
	require.NoError(t, err)

	// Create DB instance
	db := surrealdb.New(conn)
	defer db.Close(context.Background())

	// Setup
	err = db.Use(context.Background(), "test", "test")
	require.NoError(t, err)

	token, err := db.SignIn(context.Background(), &surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)
	require.Equal(t, server.TokenSignIn, token)

	err = db.Authenticate(context.Background(), token)
	require.NoError(t, err)

	type TestRecord struct {
		ID   models.RecordID `json:"id"`
		Name string          `json:"name"`
	}

	// Test select with retries
	// First two selects should work
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		result, err := surrealdb.Select[TestRecord](
			ctx,
			db,
			models.NewRecordID("test", "1"),
		)
		cancel()
		require.NoError(t, err, "Select %d should succeed", i+1)
		assert.NotNil(t, result)
		assert.Equal(t, "test", result.ID.Table)
		assert.Equal(t, "1", result.ID.ID)
	}

	// Third select will trigger connection drop
	// Track retry attempts to ensure reconnection actually happens
	var retryCount int
	var lastErr error
	var result *TestRecord

	// This should fail and trigger reconnection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result, lastErr = surrealdb.Select[TestRecord](
		ctx,
		db,
		models.NewRecordID("test", "1"),
	)

	// We expect this to fail due to connection close
	if lastErr == nil {
		t.Fatal("Expected 3rd select to fail with connection close, but it succeeded")
	}
	t.Logf("3rd select failed as expected: %v", lastErr)

	// Now retry until success - this tests the auto-reconnection
	maxRetries := 20
	for retry := 0; retry < maxRetries; retry++ {
		retryCount++
		t.Logf("Retry attempt %d after connection drop", retry+1)

		// Wait to allow reconnection check interval to trigger
		time.Sleep(600 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		result, lastErr = surrealdb.Select[TestRecord](
			ctx,
			db,
			models.NewRecordID("test", "1"),
		)
		cancel()

		if lastErr == nil {
			t.Logf("Select succeeded on retry %d", retry+1)
			break
		}

		t.Logf("Select still failing: %v", lastErr)
	}

	// Verify retry actually happened
	require.Greater(t, retryCount, 0, "At least one retry should have occurred")
	require.NoError(t, lastErr, "Select should eventually succeed after reconnection")
	assert.NotNil(t, result)
	assert.Equal(t, "test", result.ID.Table)
	assert.Equal(t, "1", result.ID.ID)

	// Verify total select count shows reconnection worked
	finalCount := atomic.LoadInt32(&selectCount)
	t.Logf("Total select requests: %d (should be > 3)", finalCount)
	require.Greater(t, int(finalCount), 3, "Should have made more than 3 select requests due to retry")

	// Verify connection is healthy after reconnection
	for i := 0; i < 3; i++ {
		result, err := surrealdb.Select[TestRecord](
			context.Background(),
			db,
			models.NewRecordID("test", "1"),
		)
		require.NoError(t, err, "Post-reconnection select %d should succeed", i+1)
		assert.NotNil(t, result)
	}

	t.Logf("Test passed: Connection dropped and successfully reconnected with %d retries", retryCount)
}

// TestDefaultWebSocketDoNotReconnect tests that the default WebSocket connection does not automatically reconnect
// when the connection is closed by the server. This ensures that our test setup actually drops connections
// and does not automatically reconnect, which is important for testing connection handling behavior,
// while verifying that the default WebSocket client does not reconnect.
func TestDefaultWebSocketDoNotReconnect(t *testing.T) {
	// This test validates that our test setup actually drops connections
	// by using a non-reconnecting client

	server := fakesdb.NewServer("127.0.0.1:0")
	server.TokenSignIn = "test_token_signin"

	// Drop connection on 2nd request
	var selectCount int32
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("select", func(params []interface{}) bool {
			count := atomic.AddInt32(&selectCount, 1)
			return count == 2
		}),
		Response: nil,
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureWebSocketClose,
				Probability: 1.0,
				CloseCode:   1001,
			},
		},
	})

	// Normal response
	server.AddStubResponse(fakesdb.SimpleStubResponse("select", map[string]interface{}{
		"id": cbor.Tag{Number: 8, Content: []interface{}{"test", "1"}},
	}))

	err := server.Start()
	require.NoError(t, err)
	defer func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Fatalf("Failed to stop server: %v", stopErr)
		}
	}()

	// Use regular connection WITHOUT auto-reconnect
	p, err := surrealdb.Configure("ws://" + server.Address())
	require.NoError(t, err)

	// Create connection with timeout to prevent hanging
	ws := gorillaws.New(p).SetTimeOut(2 * time.Second)
	err = ws.Connect(context.Background())
	require.NoError(t, err)

	db := surrealdb.New(ws)
	defer db.Close(context.Background())

	err = db.Use(context.Background(), "test", "test")
	require.NoError(t, err)

	token, err := db.SignIn(context.Background(), &surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)
	require.Equal(t, server.TokenSignIn, token)

	type TestRecord struct {
		ID models.RecordID `json:"id"`
	}

	// First select should work
	_, err = surrealdb.Select[TestRecord](
		context.Background(),
		db,
		models.NewRecordID("test", "1"),
	)
	require.NoError(t, err)

	// Second select should fail due to connection drop
	_, err = surrealdb.Select[TestRecord](
		context.Background(),
		db,
		models.NewRecordID("test", "1"),
	)
	require.Error(t, err, "Connection should be closed after WebSocket close")

	// Third select should also fail (no auto-reconnect)
	_, err = surrealdb.Select[TestRecord](
		context.Background(),
		db,
		models.NewRecordID("test", "1"),
	)
	require.Error(t, err, "Connection should remain closed without auto-reconnect")

	t.Log("Validation passed: Connection drops are working as expected")
}

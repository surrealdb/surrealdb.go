package rews

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// mockConnection implements a basic WebSocketConnection for testing
type mockConnection struct {
	connection.WebSocketConnection
	connectCalls atomic.Int32
	connectErr   error
	isClosed     atomic.Bool
	connectFunc  func(ctx context.Context) error // Allow overriding Connect behavior
}

func (m *mockConnection) Connect(ctx context.Context) error {
	m.connectCalls.Add(1)
	if m.connectFunc != nil {
		return m.connectFunc(ctx)
	}
	return m.connectErr
}

func (m *mockConnection) IsClosed() bool {
	return m.isClosed.Load()
}

func (m *mockConnection) Close(ctx context.Context) error {
	m.isClosed.Store(true)
	return nil
}

func (m *mockConnection) Use(ctx context.Context, namespace, database string) error {
	return nil
}

func (m *mockConnection) Let(ctx context.Context, key string, value any) error {
	return nil
}

func (m *mockConnection) Unset(ctx context.Context, key string) error {
	return nil
}

func (m *mockConnection) Authenticate(ctx context.Context, token string) error {
	return nil
}

func (m *mockConnection) SignUp(ctx context.Context, authData any) (string, error) {
	return testToken, nil
}

func (m *mockConnection) SignIn(ctx context.Context, authData any) (string, error) {
	return testToken, nil
}

// mockUnmarshaler implements a basic Unmarshaler for testing
type mockUnmarshaler struct{}

func (m mockUnmarshaler) Unmarshal(data []byte, v any) error {
	return cbor.Unmarshal(data, v)
}

// Ensure mockUnmarshaler implements codec.Unmarshaler
var _ codec.Unmarshaler = mockUnmarshaler{}

func TestConnectionRetry(t *testing.T) {
	t.Run("initial connect with exponential backoff", func(t *testing.T) {
		var attemptCount atomic.Int32
		failCount := 3

		mockConn := &mockConnection{}

		newFunc := func(ctx context.Context) (connection.WebSocketConnection, error) {
			attempt := int(attemptCount.Add(1))
			if attempt <= failCount {
				return nil, errors.New("connection failed")
			}
			return mockConn, nil
		}

		// Create connection with exponential backoff retryer
		conn := New(
			newFunc,
			5*time.Second,
			mockUnmarshaler{}, // unmarshaler required for reliableLQ
			nil,               // logger not needed for this test
		)

		// Configure exponential backoff with short delays for testing
		retryer := &ExponentialBackoffRetryer{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			MaxRetries:   5,
			Jitter:       false,
		}
		conn.Retryer = retryer

		// Connect should retry and eventually succeed
		ctx := context.Background()
		err := conn.Connect(ctx)
		require.NoError(t, err)

		// Should have attempted 4 times (3 failures + 1 success)
		assert.Equal(t, int32(4), attemptCount.Load())

		// Clean up
		err = conn.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("initial connect with max retries exceeded", func(t *testing.T) {
		var attemptCount atomic.Int32

		newFunc := func(ctx context.Context) (connection.WebSocketConnection, error) {
			attemptCount.Add(1)
			return nil, errors.New("connection failed")
		}

		conn := New(
			newFunc,
			5*time.Second,
			mockUnmarshaler{}, // unmarshaler required for reliableLQ
			nil,               // logger not needed for this test
		)

		// Configure retryer with limited retries
		retryer := &ExponentialBackoffRetryer{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
			Multiplier:   2.0,
			MaxRetries:   3,
			Jitter:       false,
		}
		conn.Retryer = retryer

		// Connect should fail after max retries
		ctx := context.Background()
		err := conn.Connect(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "after 4 attempts")

		// Should have attempted exactly 4 times (initial + 3 retries)
		assert.Equal(t, int32(4), attemptCount.Load())
	})

	t.Run("no retry retryer falls back to single attempt", func(t *testing.T) {
		var attemptCount atomic.Int32

		newFunc := func(ctx context.Context) (connection.WebSocketConnection, error) {
			attemptCount.Add(1)
			return nil, errors.New("connection failed")
		}

		conn := New(
			newFunc,
			5*time.Second,
			mockUnmarshaler{}, // unmarshaler required for reliableLQ
			nil,               // logger not needed for this test
		)

		// No retry retryer configured

		ctx := context.Background()
		err := conn.Connect(ctx)
		require.Error(t, err)

		// Should have attempted only once
		assert.Equal(t, int32(1), attemptCount.Load())
	})

	t.Run("context cancellation during retry", func(t *testing.T) {
		var attemptCount atomic.Int32

		newFunc := func(ctx context.Context) (connection.WebSocketConnection, error) {
			attemptCount.Add(1)
			return nil, errors.New("connection failed")
		}

		conn := New(
			newFunc,
			5*time.Second,
			mockUnmarshaler{}, // unmarshaler required for reliableLQ
			nil,               // logger not needed for this test
		)

		// Configure retryer with longer delays
		retryer := &ExponentialBackoffRetryer{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   2.0,
			MaxRetries:   10,
			Jitter:       false,
		}
		conn.Retryer = retryer

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after a short delay
		go func() {
			time.Sleep(150 * time.Millisecond)
			cancel()
		}()

		err := conn.Connect(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection canceled")

		// Should have attempted at least once but not too many times
		attempts := attemptCount.Load()
		assert.GreaterOrEqual(t, attempts, int32(1))
		assert.LessOrEqual(t, attempts, int32(3))
	})

	t.Run("reconnect with retry retryer", func(t *testing.T) {
		var createCount atomic.Int32
		var connectCount atomic.Int32
		failOnConnect := atomic.Bool{}

		mockConn := &mockConnection{}

		newFunc := func(ctx context.Context) (connection.WebSocketConnection, error) {
			createCount.Add(1)
			if failOnConnect.Load() {
				return nil, errors.New("create failed")
			}
			return mockConn, nil
		}

		// Override Connect to track calls
		mockConn.connectErr = nil
		mockConn.connectFunc = func(ctx context.Context) error {
			connectCount.Add(1)
			if failOnConnect.Load() && connectCount.Load() <= 2 {
				return errors.New("connect failed")
			}
			return nil
		}

		conn := New(
			newFunc,
			100*time.Millisecond, // Short check interval for testing
			mockUnmarshaler{},    // unmarshaler required for reliableLQ
			nil,                  // logger not needed for this test
		)

		// Configure reconnect retry retryer
		reconnectRetryer := &ExponentialBackoffRetryer{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
			Multiplier:   2.0,
			MaxRetries:   5,
			Jitter:       false,
		}
		conn.Retryer = reconnectRetryer

		// Initial connect should succeed
		ctx := context.Background()
		err := conn.Connect(ctx)
		require.NoError(t, err)

		// Simulate connection loss
		mockConn.isClosed.Store(true)
		failOnConnect.Store(true)

		// Wait for reconnection attempt
		time.Sleep(200 * time.Millisecond)

		// Allow reconnection to succeed on third attempt
		failOnConnect.Store(false)

		// Wait for successful reconnection
		time.Sleep(100 * time.Millisecond)

		// Verify reconnection occurred with retries
		assert.Greater(t, connectCount.Load(), int32(1))

		// Clean up
		err = conn.Close(ctx)
		assert.NoError(t, err)
	})
}

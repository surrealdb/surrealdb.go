package rews

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestConnection tests all public methods of the Connection struct
func TestConnection(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))

	// Helper function to create a new Connection with a mock
	createConnection := func() (*Connection[*mockWebSocketConnection], *mockWebSocketConnection) {
		mock := &mockWebSocketConnection{
			notifications: make(map[string]chan connection.Notification),
		}

		conn := &Connection[*mockWebSocketConnection]{
			WebSocketConnection: mock,
			reliableLQ:          newReliableLQ(log, &models.CborUnmarshaler{}),
			logger:              log,
			sessionVars:         make(map[string]any),
			state:               StateConnected, // Start in connected state for most tests
			CheckInterval:       100 * time.Millisecond,
		}

		return conn, mock
	}

	ctx := context.Background()

	t.Run("IsClosed", func(t *testing.T) {
		conn, _ := createConnection()

		// Initially not closed
		assert.False(t, conn.IsClosed())

		// After transitioning to closed state
		conn.state = StateClosed
		assert.True(t, conn.IsClosed())

		// Test other states
		states := []State{
			StateDisconnected,
			StateConnecting,
			StateConnected,
			StateClosing,
		}

		for _, state := range states {
			conn.state = state
			assert.False(t, conn.IsClosed(), "Should not be closed in state: %v", state)
		}
	})

	t.Run("Use", func(t *testing.T) {
		conn, mock := createConnection()

		// Test setting namespace and database
		err := conn.Use(ctx, "test_ns", "test_db")
		require.NoError(t, err)

		// Verify session state was updated
		assert.Equal(t, "test_ns", conn.sessionNS)
		assert.Equal(t, "test_db", conn.sessionDB)

		// Test with empty values
		err = conn.Use(ctx, "", "")
		require.NoError(t, err)
		assert.Equal(t, "", conn.sessionNS)
		assert.Equal(t, "", conn.sessionDB)

		// Test with special characters
		err = conn.Use(ctx, "ns-with-dash", "db_with_underscore")
		require.NoError(t, err)
		assert.Equal(t, "ns-with-dash", conn.sessionNS)
		assert.Equal(t, "db_with_underscore", conn.sessionDB)

		// Test when connection is closed
		conn.state = StateClosed
		mock.isClosed = true
		err = conn.Use(ctx, "should", "fail")
		assert.Error(t, err, "Should fail when connection is closed")
	})

	t.Run("Let", func(t *testing.T) {
		conn, mock := createConnection()

		// Test setting various types of variables
		testCases := []struct {
			key   string
			value any
		}{
			{"string_var", "test_value"},
			{"int_var", 42},
			{"float_var", 3.14},
			{"bool_var", true},
			{"nil_var", nil},
			{"map_var", map[string]any{"nested": "value"}},
			{"slice_var", []int{1, 2, 3}},
		}

		for _, tc := range testCases {
			err := conn.Let(ctx, tc.key, tc.value)
			require.NoError(t, err, "Failed to set variable %s", tc.key)
			assert.Equal(t, tc.value, conn.sessionVars[tc.key], "Variable %s not stored correctly", tc.key)
		}

		// Test overwriting existing variable
		err := conn.Let(ctx, "string_var", "new_value")
		require.NoError(t, err)
		assert.Equal(t, "new_value", conn.sessionVars["string_var"])

		// Test when connection is closed
		conn.state = StateClosed
		mock.isClosed = true
		err = conn.Let(ctx, "should_fail", "value")
		assert.Error(t, err, "Should fail when connection is closed")
	})

	t.Run("Unset", func(t *testing.T) {
		conn, mock := createConnection()

		// Set some variables first
		conn.sessionVars["var1"] = "value1"
		conn.sessionVars["var2"] = "value2"
		conn.sessionVars["var3"] = "value3"

		// Test unsetting a variable
		err := conn.Unset(ctx, "var1")
		require.NoError(t, err)
		_, exists := conn.sessionVars["var1"]
		assert.False(t, exists, "Variable should be removed")

		// Verify other variables are untouched
		assert.Equal(t, "value2", conn.sessionVars["var2"])
		assert.Equal(t, "value3", conn.sessionVars["var3"])

		// Test unsetting non-existent variable (should not error)
		err = conn.Unset(ctx, "non_existent")
		require.NoError(t, err)

		// Test unsetting all remaining variables
		err = conn.Unset(ctx, "var2")
		require.NoError(t, err)
		err = conn.Unset(ctx, "var3")
		require.NoError(t, err)
		assert.Len(t, conn.sessionVars, 0, "All variables should be removed")

		// Test when connection is closed
		conn.state = StateClosed
		mock.isClosed = true
		err = conn.Unset(ctx, "should_fail")
		assert.Error(t, err, "Should fail when connection is closed")
	})

	t.Run("SignIn", func(t *testing.T) {
		conn, mock := createConnection()

		// Test successful sign in
		authData := map[string]any{
			"user": "testuser",
			"pass": "testpass",
		}

		token, err := conn.SignIn(ctx, authData)
		require.NoError(t, err)
		assert.Equal(t, "test-token", token)
		assert.Equal(t, "test-token", conn.sessionToken, "Token should be stored in session")

		// Test with different auth structures
		authDataWithNS := map[string]any{
			"ns":   "test",
			"db":   "test",
			"user": "admin",
			"pass": "admin",
		}

		token, err = conn.SignIn(ctx, authDataWithNS)
		require.NoError(t, err)
		assert.Equal(t, "test-token", token)

		// Test with struct auth data
		type AuthStruct struct {
			Username string `json:"user"`
			Password string `json:"pass"`
		}

		authStruct := AuthStruct{
			Username: "structuser",
			Password: "structpass",
		}

		token, err = conn.SignIn(ctx, authStruct)
		require.NoError(t, err)
		assert.Equal(t, "test-token", token)

		// Test when connection is closed
		conn.state = StateClosed
		mock.isClosed = true
		_, err = conn.SignIn(ctx, authData)
		assert.Error(t, err, "Should fail when connection is closed")
	})

	t.Run("SignUp", func(t *testing.T) {
		conn, mock := createConnection()

		// Test successful sign up
		authData := map[string]any{
			"user":  "newuser",
			"pass":  "newpass",
			"email": "test@example.com",
		}

		token, err := conn.SignUp(ctx, authData)
		require.NoError(t, err)
		assert.Equal(t, "test-token", token)
		assert.Equal(t, "test-token", conn.sessionToken, "Token should be stored in session")

		// Test with minimal auth data
		minimalAuth := map[string]any{
			"user": "minimal",
			"pass": "minimal",
		}

		token, err = conn.SignUp(ctx, minimalAuth)
		require.NoError(t, err)
		assert.Equal(t, "test-token", token)

		// Test with additional fields
		extendedAuth := map[string]any{
			"user":      "extended",
			"pass":      "extended",
			"email":     "extended@example.com",
			"firstName": "Test",
			"lastName":  "User",
			"age":       25,
		}

		token, err = conn.SignUp(ctx, extendedAuth)
		require.NoError(t, err)
		assert.Equal(t, "test-token", token)

		// Test when connection is closed
		conn.state = StateClosed
		mock.isClosed = true
		_, err = conn.SignUp(ctx, authData)
		assert.Error(t, err, "Should fail when connection is closed")
	})

	t.Run("Authenticate", func(t *testing.T) {
		conn, mock := createConnection()

		// Test successful authentication
		testToken := "myexampletoken.test.signature"
		err := conn.Authenticate(ctx, testToken)
		require.NoError(t, err)
		assert.Equal(t, testToken, conn.sessionToken, "Token should be stored in session")

		// Test with different token format
		anotherToken := "Bearer token123456"
		err = conn.Authenticate(ctx, anotherToken)
		require.NoError(t, err)
		assert.Equal(t, anotherToken, conn.sessionToken, "New token should replace old one")

		// Test with empty token (should clear authentication)
		err = conn.Authenticate(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, "", conn.sessionToken, "Empty token should clear session")

		// Test when connection is closed
		conn.state = StateClosed
		mock.isClosed = true
		err = conn.Authenticate(ctx, "should_fail")
		assert.Error(t, err, "Should fail when connection is closed")
	})

	t.Run("Close", func(t *testing.T) {
		conn, mock := createConnection()

		// Initialize channels as they would be in Connect
		conn.connCloseCh = make(chan int, 1)
		conn.reconnLoopCloseCh = make(chan int, 1)

		// Start a minimal reconnection loop to avoid blocking
		go func() {
			<-conn.connCloseCh
			close(conn.reconnLoopCloseCh)
		}()

		// Setup some session state
		conn.sessionNS = "test_ns"
		conn.sessionDB = "test_db"
		conn.sessionToken = "test_token"
		conn.sessionVars["var1"] = "value1"

		// Close the connection
		err := conn.Close(ctx)
		require.NoError(t, err)
		assert.True(t, mock.isClosed, "Mock should be marked as closed")
		assert.Equal(t, StateClosed, conn.state, "Connection state should be closed")

		// Verify session state is preserved (for potential reconnection logic)
		assert.Equal(t, "test_ns", conn.sessionNS, "Session NS should be preserved")
		assert.Equal(t, "test_db", conn.sessionDB, "Session DB should be preserved")
		assert.Equal(t, "test_token", conn.sessionToken, "Session token should be preserved")
		assert.Equal(t, "value1", conn.sessionVars["var1"], "Session vars should be preserved")

		// Test closing already closed connection
		err = conn.Close(ctx)
		assert.Error(t, err, "Should error when closing already closed connection")
	})
}

// TestConnectionStateTransitions tests the state machine transitions
func TestConnectionStateTransitions(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("Valid transitions", func(t *testing.T) {
		conn := &Connection[*mockWebSocketConnection]{
			logger: log,
			state:  StateDisconnected,
		}

		// Disconnected -> Connecting
		err := conn.transitionTo(StateConnecting)
		assert.NoError(t, err)
		assert.Equal(t, StateConnecting, conn.state)

		// Connecting -> Connected
		err = conn.transitionTo(StateConnected)
		assert.NoError(t, err)
		assert.Equal(t, StateConnected, conn.state)

		// Connected -> Disconnected (connection lost)
		err = conn.transitionTo(StateDisconnected)
		assert.NoError(t, err)
		assert.Equal(t, StateDisconnected, conn.state)

		// Disconnected -> Connecting (reconnect)
		err = conn.transitionTo(StateConnecting)
		assert.NoError(t, err)
		assert.Equal(t, StateConnecting, conn.state)

		// Connecting -> Connected
		err = conn.transitionTo(StateConnected)
		assert.NoError(t, err)
		assert.Equal(t, StateConnected, conn.state)

		// Connected -> Closing
		err = conn.transitionTo(StateClosing)
		assert.NoError(t, err)
		assert.Equal(t, StateClosing, conn.state)

		// Closing -> Closed
		err = conn.transitionTo(StateClosed)
		assert.NoError(t, err)
		assert.Equal(t, StateClosed, conn.state)
	})

	t.Run("Invalid transitions", func(t *testing.T) {
		testCases := []struct {
			from State
			to   State
			desc string
		}{
			{StateDisconnected, StateConnected, "Cannot transition from Disconnected to Connected"},
			{StateDisconnected, StateClosed, "Cannot transition from Disconnected to Closed"},
			{StateConnecting, StateClosing, "Cannot transition from Connecting to Closing"},
			{StateConnected, StateClosed, "Cannot transition from Connected to Closed"},
			{StateClosing, StateConnecting, "Cannot transition from Closing to Connecting"},
			{StateClosing, StateConnected, "Cannot transition from Closing to Connected"},
			{StateClosing, StateDisconnected, "Cannot transition from Closing to Disconnected"},
			{StateClosed, StateConnecting, "Cannot reconnect from Closed state"},
			{StateClosed, StateConnected, "Cannot reconnect from Closed state"},
			{StateClosed, StateDisconnected, "Cannot disconnect from Closed state"},
		}

		for _, tc := range testCases {
			conn := &Connection[*mockWebSocketConnection]{
				logger: log,
				state:  tc.from,
			}

			err := conn.transitionTo(tc.to)
			assert.Error(t, err, tc.desc)
			assert.Equal(t, tc.from, conn.state, "State should not change on invalid transition")
		}
	})

	t.Run("Self transitions", func(t *testing.T) {
		// Some states allow self-transitions
		conn := &Connection[*mockWebSocketConnection]{
			logger: log,
			state:  StateDisconnected,
		}

		// Disconnected -> Disconnected (allowed)
		err := conn.transitionTo(StateDisconnected)
		assert.NoError(t, err)
		assert.Equal(t, StateDisconnected, conn.state)
	})
}

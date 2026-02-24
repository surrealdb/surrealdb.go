package testenv

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	surrealhttp "github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// These tests explore the actual SurrealDB v3 RPC behavior for sessions and transactions.
// They use low-level connection.Send to understand the exact parameter formats.

// RPCRequestInTransaction is a request that includes a transaction ID.
// SurrealDB v3 expects the txn field at the top level of the CBOR-encoded request.
//
// Transactions are CONNECTION-scoped, not session-scoped. Any session on the same
// connection can use, commit, or cancel any transaction started on that connection.
// The txn field specifies which transaction context to use for the operation.
type RPCRequestInTransaction struct {
	ID     any    `cbor:"id"`
	Method string `cbor:"method,omitempty"`
	Params []any  `cbor:"params,omitempty"`
	Txn    any    `cbor:"txn,omitempty"`
}

// RPCRequestInSession is a request that includes a session ID.
// SurrealDB v3 expects the session field at the top level of the CBOR-encoded request.
//
// Sessions scope authentication state, namespace/database selection, and live notifications.
// New sessions start unauthenticated and must call signin and use before executing queries.
// Sessions do NOT scope transactions - transactions are connection-scoped.
type RPCRequestInSession struct {
	ID      any    `cbor:"id"`
	Method  string `cbor:"method,omitempty"`
	Params  []any  `cbor:"params,omitempty"`
	Session any    `cbor:"session,omitempty"`
}

// rpcRequestWithSessionTxn is used internally by sendCustomRPC for exploration tests.
// It supports both session and txn fields, but in practice these are rarely needed together
// since transactions are connection-scoped (not session-scoped). This struct is primarily
// useful for testing edge cases like cross-session transaction usage.
type rpcRequestWithSessionTxn struct {
	ID      any    `cbor:"id"`
	Method  string `cbor:"method,omitempty"`
	Params  []any  `cbor:"params,omitempty"`
	Session any    `cbor:"session,omitempty"`
	Txn     any    `cbor:"txn,omitempty"`
}

// sendCustomRPC sends an RPC request with optional session and txn fields at the top level.
// This bypasses connection.Send to allow us to include these fields in the CBOR encoding.
func sendCustomRPC[T any](
	conn *gorillaws.Connection,
	ctx context.Context,
	method string,
	sessionID any,
	txnID any,
	params ...any,
) (*connection.RPCResponse[T], error) {
	// Create a unique request ID
	id := fmt.Sprintf("test-%d", time.Now().UnixNano())

	request := &rpcRequestWithSessionTxn{
		ID:      id,
		Method:  method,
		Params:  params,
		Session: sessionID,
		Txn:     txnID,
	}

	// Create response channel before sending
	responseChan, err := conn.CreateResponseChannel(id)
	if err != nil {
		return nil, err
	}
	defer conn.RemoveResponseChannel(id)

	// Marshal and write the request
	data, err := conn.Marshaler.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use the underlying gorilla connection to write (2 is BinaryMessage)
	if err := conn.Conn.WriteMessage(2, data); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	// Wait for response with timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res, open := <-responseChan:
		if !open {
			return nil, fmt.Errorf("response channel closed")
		}

		if res.Error != nil {
			return nil, res.Error
		}

		// Unmarshal the result
		var result T
		if res.Result != nil {
			resultData, err := res.Result.MarshalCBOR()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result for re-unmarshal: %w", err)
			}
			if err := conn.Unmarshaler.Unmarshal(resultData, &result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}

		return &connection.RPCResponse[T]{
			ID:     res.ID,
			Error:  res.Error,
			Result: &result,
		}, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// sendInTransaction sends an RPC request with the txn field at the top level.
// Convenience wrapper around sendCustomRPC.
func sendInTransaction[T any](
	conn *gorillaws.Connection,
	ctx context.Context,
	method string,
	txnID any,
	params ...any,
) (*connection.RPCResponse[T], error) {
	return sendCustomRPC[T](conn, ctx, method, nil, txnID, params...)
}

// sendInSession sends an RPC request with the session field at the top level.
// Convenience wrapper around sendCustomRPC.
func sendInSession[T any](
	conn *gorillaws.Connection,
	ctx context.Context,
	method string,
	sessionID any,
	params ...any,
) (*connection.RPCResponse[T], error) {
	return sendCustomRPC[T](conn, ctx, method, sessionID, nil, params...)
}

// setupWSConnection creates a WebSocket connection for testing.
// Sessions and interactive transactions only work over WebSocket.
// Returns *gorillaws.Connection to allow access to low-level connection for custom RPC requests.
func setupWSConnection(t *testing.T, namespace, database string) *gorillaws.Connection {
	t.Helper()

	c := surrealcbor.New()
	wsURL := GetSurrealDBWSURL()
	// gorillaws.Connect appends "/rpc" to BaseURL, so we need to strip it if present
	baseURL := strings.TrimSuffix(wsURL, "/rpc")
	conn := gorillaws.New(&connection.Config{
		BaseURL:     baseURL,
		Marshaler:   c,
		Unmarshaler: c,
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})

	ctx := context.Background()

	err := conn.Connect(ctx)
	require.NoError(t, err)

	err = conn.Use(ctx, namespace, database)
	require.NoError(t, err)

	// Sign in as root
	var token connection.RPCResponse[string]
	err = connection.Send(conn, ctx, &token, "signin", map[string]any{
		"user": "root",
		"pass": "root",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close(ctx)
	})

	return conn
}

// setupHTTPConnection creates an HTTP connection for testing.
// HTTP connections do NOT support sessions or interactive transactions.
// This is used to verify that session/transaction RPCs fail appropriately over HTTP.
func setupHTTPConnection(t *testing.T, namespace, database string) *surrealhttp.Connection {
	t.Helper()

	c := surrealcbor.New()
	// Derive HTTP URL from WebSocket URL
	wsURL := GetSurrealDBWSURL()
	httpURL := strings.ReplaceAll(wsURL, "ws://", "http://")
	httpURL = strings.ReplaceAll(httpURL, "wss://", "https://")
	// Remove /rpc suffix that's needed for WebSocket but not for HTTP
	httpURL = strings.TrimSuffix(httpURL, "/rpc")

	conn := surrealhttp.New(&connection.Config{
		BaseURL:     httpURL,
		Marshaler:   c,
		Unmarshaler: c,
	})

	ctx := context.Background()

	err := conn.Connect(ctx)
	require.NoError(t, err)

	err = conn.Use(ctx, namespace, database)
	require.NoError(t, err)

	// Sign in as root and store the token for HTTP requests
	var tokenRes connection.RPCResponse[string]
	err = connection.Send(conn, ctx, &tokenRes, "signin", map[string]any{
		"user": "root",
		"pass": "root",
	})
	require.NoError(t, err)
	require.NotNil(t, tokenRes.Result)

	// For HTTP connections, we must store the token via Authenticate
	// so it gets included in the Authorization header
	err = conn.Authenticate(ctx, *tokenRes.Result)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close(ctx)
	})

	return conn
}

// httpConnectionInfo holds the information needed to make custom HTTP RPC requests
// with session and txn fields at the CBOR top level.
type httpConnectionInfo struct {
	baseURL   string
	namespace string
	database  string
	token     string
	marshaler *surrealcbor.Codec
}

// setupHTTPConnectionInfo creates an HTTP connection and returns the info needed
// for custom RPC requests. This is used to test session/transaction support over HTTP.
func setupHTTPConnectionInfo(t *testing.T, namespace, database string) *httpConnectionInfo {
	t.Helper()

	c := surrealcbor.New()
	// Derive HTTP URL from WebSocket URL
	wsURL := GetSurrealDBWSURL()
	httpURL := strings.ReplaceAll(wsURL, "ws://", "http://")
	httpURL = strings.ReplaceAll(httpURL, "wss://", "https://")
	// Remove /rpc suffix that's needed for WebSocket but not for HTTP
	httpURL = strings.TrimSuffix(httpURL, "/rpc")

	conn := surrealhttp.New(&connection.Config{
		BaseURL:     httpURL,
		Marshaler:   c,
		Unmarshaler: c,
	})

	ctx := context.Background()

	err := conn.Connect(ctx)
	require.NoError(t, err)

	err = conn.Use(ctx, namespace, database)
	require.NoError(t, err)

	// Sign in as root and get the token
	var tokenRes connection.RPCResponse[string]
	err = connection.Send(conn, ctx, &tokenRes, "signin", map[string]any{
		"user": "root",
		"pass": "root",
	})
	require.NoError(t, err)
	require.NotNil(t, tokenRes.Result)

	t.Cleanup(func() {
		_ = conn.Close(ctx)
	})

	return &httpConnectionInfo{
		baseURL:   httpURL,
		namespace: namespace,
		database:  database,
		token:     *tokenRes.Result,
		marshaler: c,
	}
}

// sendCustomHTTPRPC sends an RPC request over HTTP with optional session and txn fields
// at the top level of the CBOR-encoded request body.
// This bypasses the standard connection.Send to allow us to include these custom fields.
func sendCustomHTTPRPC[T any](
	info *httpConnectionInfo,
	ctx context.Context,
	method string,
	sessionID any,
	txnID any,
	params ...any,
) (*connection.RPCResponse[T], error) {
	// Create request with session/txn at top level
	request := &rpcRequestWithSessionTxn{
		ID:      rand.NewRequestID(constants.RequestIDLength),
		Method:  method,
		Params:  params,
		Session: sessionID,
		Txn:     txnID,
	}

	reqBody, err := info.marshaler.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, info.baseURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("Surreal-NS", info.namespace)
	req.Header.Set("Surreal-DB", info.database)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", info.token))

	client := &http.Client{}
	resp, err := client.Do(req) //nolint:gosec // G704: URL from trusted test config
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Unmarshal response
	var rawRes connection.RPCResponse[T]
	if err := info.marshaler.Unmarshal(respData, &rawRes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rawRes.Error != nil {
		return nil, rawRes.Error
	}

	return &rawRes, nil
}

// sendHTTPInSession sends an RPC request over HTTP with the session field at the top level.
func sendHTTPInSession[T any](
	info *httpConnectionInfo,
	ctx context.Context,
	method string,
	sessionID any,
	params ...any,
) (*connection.RPCResponse[T], error) {
	return sendCustomHTTPRPC[T](info, ctx, method, sessionID, nil, params...)
}

// sendHTTPInTransaction sends an RPC request over HTTP with the txn field at the top level.
func sendHTTPInTransaction[T any](
	info *httpConnectionInfo,
	ctx context.Context,
	method string,
	txnID any,
	params ...any,
) (*connection.RPCResponse[T], error) {
	return sendCustomHTTPRPC[T](info, ctx, method, nil, txnID, params...)
}

// getVersion gets and checks the SurrealDB version via a separate DB connection
func getVersion(t *testing.T) *SurrealDBVersion {
	t.Helper()
	db := MustNew("version_check", "version_check", "dummy")
	v, err := GetVersion(context.Background(), db)
	require.NoError(t, err)
	_ = db.Close(context.Background())
	return v
}

// TestExplore_HTTPConnection_Sessions tests whether session RPCs work over HTTP
// when using custom RPC requests with the session field at the CBOR top level.
// This test verifies the actual SurrealDB behavior for sessions over HTTP.
//
// DISCOVERY: Sessions DO work over HTTP when using custom CBOR requests with
// the session field at the top level!
func TestExplore_HTTPConnection_Sessions(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = MustNew("explore_http", "session_test", "test_table")

	info := setupHTTPConnectionInfo(t, "explore_http", "session_test")
	ctx := context.Background()

	type queryResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// Test: Full session workflow over HTTP
	t.Run("full session workflow over HTTP", func(t *testing.T) {
		sessionUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
		t.Logf("Testing session UUID: %s", sessionUUID.String())

		// Step 1: attach - create session
		res, err := sendHTTPInSession[any](info, ctx, "attach", &sessionUUID)
		require.NoError(t, err, "attach should succeed over HTTP")
		t.Logf("attach succeeded: %+v", res.Result)

		// Step 2: signin on the session
		signinRes, err := sendHTTPInSession[string](info, ctx, "signin", &sessionUUID,
			map[string]any{"user": "root", "pass": "root"})
		require.NoError(t, err, "signin on session should succeed over HTTP")
		t.Logf("signin on session succeeded, token length: %d", len(*signinRes.Result))

		// Step 3: use namespace/database on the session
		useRes, err := sendHTTPInSession[any](info, ctx, "use", &sessionUUID,
			"explore_http", "session_test")
		require.NoError(t, err, "use on session should succeed over HTTP")
		t.Logf("use on session succeeded: %+v", useRes.Result)

		// Step 4: query using the session
		queryRes, err := sendHTTPInSession[[]queryResult](info, ctx, "query", &sessionUUID,
			"CREATE test_table:http_session SET value = 'created via HTTP session'",
			map[string]any{})
		require.NoError(t, err, "query on session should succeed over HTTP")
		require.NotNil(t, queryRes.Result, "query result should not be nil")
		require.Len(t, *queryRes.Result, 1, "should have one query result")
		assert.Equal(t, "OK", (*queryRes.Result)[0].Status, "query should succeed")
		t.Logf("query on session succeeded: %+v", queryRes.Result)

		// Step 5: detach - delete session
		detachRes, err := sendHTTPInSession[any](info, ctx, "detach", &sessionUUID)
		require.NoError(t, err, "detach should succeed over HTTP")
		t.Logf("detach succeeded: %+v", detachRes.Result)

		// Step 6: Verify session no longer exists
		_, err = sendHTTPInSession[[]queryResult](info, ctx, "query", &sessionUUID,
			"SELECT * FROM test_table", map[string]any{})
		require.Error(t, err, "query with deleted session should fail")
		assert.Contains(t, err.Error(), "Session not found", "error should indicate session not found")
		t.Logf("query with deleted session failed as expected: %v", err)
	})

	// Test: attach without session field (standard Send) - should fail
	t.Run("attach without session field over HTTP fails", func(t *testing.T) {
		conn := setupHTTPConnection(t, "explore_http", "session_test")
		var resAny connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &resAny, "attach")
		require.Error(t, err, "attach without session field should fail")
		t.Logf("attach without session field error: %v", err)
	})
}

// TestExplore_HTTPConnection_Transactions tests whether transaction RPCs work over HTTP.
// This test verifies the actual SurrealDB behavior for interactive transactions over HTTP.
func TestExplore_HTTPConnection_Transactions(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_http", "txn_test", "test_table")

	info := setupHTTPConnectionInfo(t, "explore_http", "txn_test")
	ctx := context.Background()

	type queryResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// Test: begin RPC over HTTP (doesn't need session/txn field)
	t.Run("begin over HTTP", func(t *testing.T) {
		// begin doesn't need session or txn field, so we can use standard request
		res, err := sendCustomHTTPRPC[models.UUID](info, ctx, "begin", nil, nil)
		if err != nil {
			t.Logf("begin over HTTP failed: %v", err)
			t.Logf("This indicates HTTP does not support interactive transactions")
		} else {
			t.Logf("begin over HTTP succeeded! txnID: %+v", res.Result)

			// If begin worked, try to use the transaction for a query
			txnID := res.Result
			t.Run("query with txn field over HTTP", func(t *testing.T) {
				queryRes, err := sendHTTPInTransaction[[]queryResult](info, ctx, "query", txnID,
					"CREATE test_table:http_txn SET value = 'in transaction'",
					map[string]any{})
				if err != nil {
					t.Logf("query with txn field over HTTP failed: %v", err)
				} else {
					t.Logf("query with txn field over HTTP succeeded: %+v", queryRes.Result)
				}
			})

			// Try to commit
			t.Run("commit over HTTP", func(t *testing.T) {
				commitRes, err := sendCustomHTTPRPC[any](info, ctx, "commit", nil, nil, txnID)
				if err != nil {
					t.Logf("commit over HTTP failed: %v", err)
				} else {
					t.Logf("commit over HTTP succeeded: %+v", commitRes.Result)
				}
			})
		}
	})

	// Test: begin with session field (test session+transaction combo)
	// First attach a session, then try to begin a transaction in that session
	t.Run("begin with session field over HTTP", func(t *testing.T) {
		sessionUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}

		// Step 1: Attach the session first
		_, err := sendHTTPInSession[any](info, ctx, "attach", &sessionUUID)
		require.NoError(t, err, "attach should succeed over HTTP")

		// Step 2: Sign in on the session
		_, err = sendHTTPInSession[string](info, ctx, "signin", &sessionUUID,
			map[string]any{"user": "root", "pass": "root"})
		require.NoError(t, err, "signin on session should succeed over HTTP")

		// Step 3: Use namespace/database on the session
		_, err = sendHTTPInSession[any](info, ctx, "use", &sessionUUID,
			"explore_http", "txn_test")
		require.NoError(t, err, "use on session should succeed over HTTP")

		// Step 4: Try to begin a transaction in this session
		res, err := sendHTTPInSession[models.UUID](info, ctx, "begin", &sessionUUID)
		if err != nil {
			t.Logf("begin with session over HTTP failed: %v", err)
			t.Logf("This confirms interactive transactions are not available over HTTP, even within a valid session")
		} else {
			t.Logf("begin with session over HTTP succeeded! txnID: %+v", res.Result)
			// Clean up transaction
			_, _ = sendCustomHTTPRPC[any](info, ctx, "cancel", nil, nil, res.Result)
		}

		// Clean up session
		_, _ = sendHTTPInSession[any](info, ctx, "detach", &sessionUUID)
	})
}

// TestExplore_HTTPConnection_QueryWorks tests that regular queries work over HTTP.
// This confirms that the HTTP connection is properly set up and only session/transaction
// RPCs are unsupported.
func TestExplore_HTTPConnection_QueryWorks(t *testing.T) {
	_ = MustNew("explore_http", "query_test", "test_table")

	conn := setupHTTPConnection(t, "explore_http", "query_test")
	ctx := context.Background()

	// queryResultWithSlice expects Result to be an array (for non-empty results)
	type queryResultWithSlice struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// queryResultWithAnyResult handles both empty and non-empty results
	type queryResultWithAnyResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result any    `cbor:"result"`
	}

	// Test: Regular query should work over HTTP (may return empty result)
	t.Run("query works over HTTP", func(t *testing.T) {
		var queryRes connection.RPCResponse[[]queryResultWithAnyResult]
		err := connection.Send(conn, ctx, &queryRes, "query",
			"SELECT * FROM test_table", map[string]any{})
		require.NoError(t, err, "query should succeed over HTTP")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		t.Logf("query over HTTP succeeded: %+v", queryRes.Result)
	})

	// Test: CREATE should work over HTTP (auto-commits)
	t.Run("create works over HTTP", func(t *testing.T) {
		var queryRes connection.RPCResponse[[]queryResultWithSlice]
		err := connection.Send(conn, ctx, &queryRes, "query",
			"CREATE test_table:http_test SET value = 'created via HTTP'",
			map[string]any{})
		require.NoError(t, err, "CREATE should succeed over HTTP")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Greater(t, len(*queryRes.Result), 0, "should have query result")
		assert.Equal(t, "OK", (*queryRes.Result)[0].Status, "query should succeed")
		t.Logf("CREATE over HTTP succeeded: %+v", queryRes.Result)
	})
}

// TestExplore_AttachRPC tests the attach RPC method to understand:
// - What parameters it accepts
// - What type is returned (UUID? string?)
// - The Go type mapping
func TestExplore_AttachRPC(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	// Ensure namespace/database exist
	_ = MustNew("explore_sessions", "attach_test", "test_table")

	conn := setupWSConnection(t, "explore_sessions", "attach_test")
	ctx := context.Background()

	// Test 1: Call attach with no parameters - this should fail with "Expected a session ID"
	t.Run("attach with no params requires session ID", func(t *testing.T) {
		var resAny connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &resAny, "attach")
		require.Error(t, err, "attach without session at top level should fail")
		assert.Contains(t, err.Error(), "session", "error should mention session")
	})

	// Test 2: attach with session ID at top-level CBOR field should succeed
	t.Run("attach with session at top level succeeds", func(t *testing.T) {
		testUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
		res, err := sendInSession[any](conn, ctx, "attach", &testUUID)
		require.NoError(t, err, "attach with session at top level should succeed")
		assert.NotNil(t, res, "response should not be nil")
		// Clean up
		_, _ = sendInSession[any](conn, ctx, "detach", &testUUID)
	})

	// Test 3: attach with session ID as string at top-level CBOR field should also succeed
	t.Run("attach with session string at top level succeeds", func(t *testing.T) {
		testUUID := uuid.Must(uuid.NewV4()).String()
		res, err := sendInSession[any](conn, ctx, "attach", testUUID)
		require.NoError(t, err, "attach with session string at top level should succeed")
		assert.NotNil(t, res, "response should not be nil")
		// Clean up
		_, _ = sendInSession[any](conn, ctx, "detach", testUUID)
	})

	// Test 4: Query with non-existent session ID should fail with "Session not found"
	t.Run("query with non-existent session fails", func(t *testing.T) {
		testUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
		type queryResult struct {
			Status string `cbor:"status"`
			Time   string `cbor:"time"`
			Result []any  `cbor:"result"`
		}
		_, err := sendInSession[[]queryResult](conn, ctx, "query", &testUUID,
			"SELECT * FROM test_table", map[string]any{})
		require.Error(t, err, "query with non-existent session should fail")
		assert.Contains(t, err.Error(), "Session not found", "error should indicate session not found")
	})
}

// TestExplore_SessionWorkflow tests the complete session workflow:
// 1. attach with session ID at top level → creates session
// 2. query with session ID at top level → uses session
// 3. detach with session ID at top level → deletes session
func TestExplore_SessionWorkflow(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = MustNew("explore_sessions", "workflow_test", "test_table")

	conn := setupWSConnection(t, "explore_sessions", "workflow_test")
	ctx := context.Background()

	type queryResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// Step 1: Create a session using attach with session at top level
	sessionUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	t.Logf("Creating session with UUID: %s", sessionUUID.String())

	t.Run("1. attach creates session", func(t *testing.T) {
		res, err := sendInSession[any](conn, ctx, "attach", &sessionUUID)
		if err != nil {
			t.Fatalf("attach failed: %v", err)
		}
		t.Logf("attach succeeded: %+v", res.Result)
	})

	// Step 1.5: Authenticate the new session (sessions start unauthenticated)
	t.Run("1.5. signin on session", func(t *testing.T) {
		res, err := sendInSession[string](conn, ctx, "signin", &sessionUUID,
			map[string]any{"user": "root", "pass": "root"})
		if err != nil {
			t.Fatalf("signin on session failed: %v", err)
		}
		t.Logf("signin on session succeeded, token: %s", *res.Result)
	})

	// Step 1.6: Use namespace/database on the session
	t.Run("1.6. use on session", func(t *testing.T) {
		res, err := sendInSession[any](conn, ctx, "use", &sessionUUID,
			"explore_sessions", "workflow_test")
		if err != nil {
			t.Fatalf("use on session failed: %v", err)
		}
		t.Logf("use on session succeeded: %+v", res.Result)
	})

	// Step 2: Use the session for a query
	t.Run("2. query uses session", func(t *testing.T) {
		res, err := sendInSession[any](conn, ctx, "query", &sessionUUID,
			"SELECT * FROM test_table", map[string]any{})
		require.NoError(t, err, "query with authenticated session should succeed")
		assert.NotNil(t, res, "response should not be nil")
	})

	// Step 3: Start a transaction within the session
	t.Run("3. begin transaction in session", func(t *testing.T) {
		res, err := sendInSession[models.UUID](conn, ctx, "begin", &sessionUUID)
		require.NoError(t, err, "begin in session should succeed")
		require.NotNil(t, res.Result, "begin should return txn UUID")
		t.Logf("begin in session returned txnID: %+v", res.Result)
		// Cancel it to clean up
		_, err = sendInSession[any](conn, ctx, "cancel", &sessionUUID, res.Result)
		require.NoError(t, err, "cancel should succeed")
	})

	// Step 4: Detach (delete) the session
	t.Run("4. detach deletes session", func(t *testing.T) {
		res, err := sendInSession[any](conn, ctx, "detach", &sessionUUID)
		if err != nil {
			t.Fatalf("detach failed: %v", err)
		}
		t.Logf("detach succeeded: %+v", res.Result)
	})

	// Step 5: Verify session no longer exists
	t.Run("5. query with deleted session fails", func(t *testing.T) {
		_, err := sendInSession[[]queryResult](conn, ctx, "query", &sessionUUID,
			"SELECT * FROM test_table", map[string]any{})
		require.Error(t, err, "query with deleted session should fail")
		assert.Contains(t, err.Error(), "Session not found", "error should indicate session not found")
	})
}

// TestExplore_DetachRPC tests the detach RPC method to understand:
// - What parameters it expects (session UUID format)
// - Error behavior for invalid session
func TestExplore_DetachRPC(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = MustNew("explore_sessions", "detach_test", "test_table")

	conn := setupWSConnection(t, "explore_sessions", "detach_test")
	ctx := context.Background()

	// Create a session first using session at top level
	sessionUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	res, err := sendInSession[any](conn, ctx, "attach", &sessionUUID)
	require.NoError(t, err)
	t.Logf("Created session: %s, result: %+v", sessionUUID.String(), res.Result)

	// Test detach with session at top level succeeds
	t.Run("detach with session at top level succeeds", func(t *testing.T) {
		detachRes, detachErr := sendInSession[any](conn, ctx, "detach", &sessionUUID)
		require.NoError(t, detachErr, "detach with session at top level should succeed")
		assert.NotNil(t, detachRes, "response should not be nil")
	})

	// Create another session for testing double detach
	sessionUUID2 := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	_, err = sendInSession[any](conn, ctx, "attach", &sessionUUID2)
	require.NoError(t, err)

	// Detach it
	_, err = sendInSession[any](conn, ctx, "detach", &sessionUUID2)
	require.NoError(t, err)

	// Test double detach - SurrealDB allows double detach (idempotent behavior)
	t.Run("double detach is idempotent", func(t *testing.T) {
		_, err := sendInSession[any](conn, ctx, "detach", &sessionUUID2)
		// SurrealDB allows double detach - this is idempotent behavior
		assert.NoError(t, err, "double detach should be allowed (idempotent)")
	})
}

// TestExplore_BeginRPC tests the begin RPC method to understand:
// - What parameters it accepts
// - What type is returned (transaction UUID)
// - How session parameter works
func TestExplore_BeginRPC(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn", "begin_test", "test_table")

	conn := setupWSConnection(t, "explore_txn", "begin_test")
	ctx := context.Background()

	// Test 1: Begin transaction on default session returns models.UUID
	t.Run("begin on default session returns UUID", func(t *testing.T) {
		var resUUID connection.RPCResponse[models.UUID]
		err := connection.Send(conn, ctx, &resUUID, "begin")
		require.NoError(t, err, "begin should succeed")
		require.NotNil(t, resUUID.Result, "begin should return a UUID")
		t.Logf("begin returned models.UUID: %+v", resUUID.Result)
		// Cancel it to clean up
		var cancelRes connection.RPCResponse[any]
		err = connection.Send(conn, ctx, &cancelRes, "cancel", resUUID.Result)
		require.NoError(t, err, "cancel should succeed")
	})

	// Test 2: Verify the raw type returned by begin is models.UUID
	t.Run("begin returns models.UUID type", func(t *testing.T) {
		var resAny connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &resAny, "begin")
		require.NoError(t, err, "begin should succeed")
		require.NotNil(t, resAny.Result, "begin should return a value")
		_, ok := (*resAny.Result).(models.UUID)
		assert.True(t, ok, "begin should return models.UUID type, got %T", *resAny.Result)
		// Cancel to clean up
		var cancelRes connection.RPCResponse[any]
		_ = connection.Send(conn, ctx, &cancelRes, "cancel", *resAny.Result)
	})

	// Test 3: Begin with session parameter succeeds
	t.Run("begin with session at top level succeeds", func(t *testing.T) {
		// Create a session using session at top-level CBOR field
		sessionUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
		_, err := sendInSession[any](conn, ctx, "attach", &sessionUUID)
		require.NoError(t, err, "attach should succeed")

		// Begin with session at top level
		beginRes, err := sendInSession[models.UUID](conn, ctx, "begin", &sessionUUID)
		require.NoError(t, err, "begin with session at top level should succeed")
		require.NotNil(t, beginRes.Result, "begin should return txn UUID")
		t.Logf("begin with session returned: %+v", beginRes.Result)

		// Cancel the transaction
		var cancelRes connection.RPCResponse[any]
		err = connection.Send(conn, ctx, &cancelRes, "cancel", beginRes.Result)
		require.NoError(t, err, "cancel should succeed")

		// Clean up session
		_, _ = sendInSession[any](conn, ctx, "detach", &sessionUUID)
	})
}

// TestExplore_CommitRPC tests the commit RPC method to understand:
// - Parameter format (txn key in map, or direct param)
func TestExplore_CommitRPC(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn", "commit_test", "test_table")

	conn := setupWSConnection(t, "explore_txn", "commit_test")
	ctx := context.Background()

	// Start a transaction to get txn ID
	var beginRes connection.RPCResponse[any]
	err := connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID := *beginRes.Result
	t.Logf("Transaction ID: %+v (type: %T)", txnID, txnID)

	// Test commit with txn in map fails (wrong format)
	t.Run("commit with txn in map fails", func(t *testing.T) {
		var commitRes connection.RPCResponse[any]
		sendErr := connection.Send(conn, ctx, &commitRes, "commit", map[string]any{"txn": txnID})
		require.Error(t, sendErr, "commit with txn in map should fail")
		assert.Contains(t, sendErr.Error(), "transaction", "error should mention transaction")
	})

	// Start another transaction for testing direct param
	err = connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID = *beginRes.Result

	// Test commit with direct param succeeds (correct format)
	t.Run("commit with direct param succeeds", func(t *testing.T) {
		var commitRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &commitRes, "commit", txnID)
		require.NoError(t, err, "commit with direct param should succeed")
	})
}

// TestExplore_CancelRPC tests the cancel RPC method to understand:
// - Parameter format (params array vs map)
func TestExplore_CancelRPC(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn", "cancel_test", "test_table")

	conn := setupWSConnection(t, "explore_txn", "cancel_test")
	ctx := context.Background()

	// Start a transaction to cancel
	var beginRes connection.RPCResponse[any]
	err := connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID := *beginRes.Result
	t.Logf("Transaction ID: %+v (type: %T)", txnID, txnID)

	// Test cancel with direct param succeeds (correct format)
	t.Run("cancel with direct param succeeds", func(t *testing.T) {
		var cancelRes connection.RPCResponse[any]
		sendErr := connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
		require.NoError(t, sendErr, "cancel with direct param should succeed")
	})

	// Start another transaction for testing map format
	err = connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID = *beginRes.Result

	// Test cancel with txn in map fails (wrong format)
	t.Run("cancel with txn in map fails", func(t *testing.T) {
		var cancelRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &cancelRes, "cancel", map[string]any{"txn": txnID})
		require.Error(t, err, "cancel with txn in map should fail")
		assert.Contains(t, err.Error(), "transaction", "error should mention transaction")
	})
}

// TestExplore_TransactionIsolation tests if transaction isolation works as expected
func TestExplore_TransactionIsolation(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn", "isolation_test", "users")

	conn := setupWSConnection(t, "explore_txn", "isolation_test")
	ctx := context.Background()

	// queryResultWithAnyResult handles both empty and non-empty results
	// SurrealDB may return Result as an empty string when no records exist
	type queryResultWithAnyResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result any    `cbor:"result"`
	}

	// Start a transaction
	var beginRes connection.RPCResponse[models.UUID]
	err := connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID := beginRes.Result
	t.Logf("Transaction ID: %+v", txnID)

	// Test: Query with txn as positional param fails (wrong format - documented behavior)
	t.Run("query with txn as positional param fails", func(t *testing.T) {
		var queryRes connection.RPCResponse[[]queryResultWithAnyResult]
		err := connection.Send(conn, ctx, &queryRes, "query",
			"CREATE users:alice SET name = 'Alice'",
			map[string]any{},
			txnID)
		// This fails because txn should be at top-level CBOR field, not positional param
		require.Error(t, err, "query with txn as positional param should fail")
		assert.Contains(t, err.Error(), "query", "error should be about query format")
	})

	// Test: Query with txn at top level succeeds (correct format)
	t.Run("query with txn at top level succeeds", func(t *testing.T) {
		res, err := sendInTransaction[[]queryResultWithAnyResult](conn, ctx, "query", txnID,
			"CREATE users:alice SET name = 'Alice'",
			map[string]any{})
		require.NoError(t, err, "query with txn at top level should succeed")
		require.NotNil(t, res.Result, "result should not be nil")
	})

	// Test: Check if data is visible before commit FROM A DIFFERENT CONNECTION
	t.Run("uncommitted data not visible from other connection", func(t *testing.T) {
		conn2 := setupWSConnection(t, "explore_txn", "isolation_test")
		var queryRes connection.RPCResponse[[]queryResultWithAnyResult]
		err := connection.Send(conn2, ctx, &queryRes, "query",
			"SELECT * FROM users", map[string]any{})
		require.NoError(t, err, "SELECT query should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Greater(t, len(*queryRes.Result), 0, "should have query result")
		// Result can be empty slice, empty string, or populated slice
		result := (*queryRes.Result)[0].Result
		count := 0
		if arr, ok := result.([]any); ok {
			count = len(arr)
		}
		assert.Equal(t, 0, count, "uncommitted data should not be visible from other connection")
	})

	// Cancel the transaction and verify rollback
	t.Run("cancel rolls back data", func(t *testing.T) {
		var cancelRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
		require.NoError(t, err, "cancel should succeed")

		// Verify data is rolled back by checking from another connection
		conn3 := setupWSConnection(t, "explore_txn", "isolation_test")
		var queryRes connection.RPCResponse[[]queryResultWithAnyResult]
		err = connection.Send(conn3, ctx, &queryRes, "query",
			"SELECT * FROM users", map[string]any{})
		require.NoError(t, err, "SELECT query should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Greater(t, len(*queryRes.Result), 0, "should have query result")
		// Result can be empty slice, empty string, or populated slice
		result := (*queryRes.Result)[0].Result
		count := 0
		if arr, ok := result.([]any); ok {
			count = len(arr)
		}
		assert.Equal(t, 0, count, "data should be rolled back after cancel")
	})
}

// TestExplore_TransactionAutoCommit tests if transactions without txn param auto-commit
func TestExplore_TransactionAutoCommit(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn", "autocommit_test", "items")

	conn := setupWSConnection(t, "explore_txn", "autocommit_test")
	ctx := context.Background()

	// queryResultWithSlice expects Result to be an array (for non-empty results)
	type queryResultWithSlice struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// queryResultWithAnyResult handles both empty and non-empty results
	type queryResultWithAnyResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result any    `cbor:"result"`
	}

	t.Run("query without txn param auto-commits", func(t *testing.T) {
		// Create without txn param - should auto-commit
		var queryRes connection.RPCResponse[[]queryResultWithSlice]
		err := connection.Send(conn, ctx, &queryRes, "query",
			"CREATE items:test1 SET value = 100",
			map[string]any{})
		require.NoError(t, err, "CREATE without txn should succeed and auto-commit")

		// Check from another connection - should be visible
		conn2 := setupWSConnection(t, "explore_txn", "autocommit_test")
		err = connection.Send(conn2, ctx, &queryRes, "query",
			"SELECT * FROM items", map[string]any{})
		require.NoError(t, err, "SELECT from another connection should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Greater(t, len(*queryRes.Result), 0, "should have query result")
		assert.Greater(t, len((*queryRes.Result)[0].Result), 0, "auto-committed data should be visible from other connection")
	})

	t.Run("begin with txn at top level then cancel rolls back", func(t *testing.T) {
		// Clean up first
		var cleanRes connection.RPCResponse[any]
		_ = connection.Send(conn, ctx, &cleanRes, "query", "DELETE items", map[string]any{})

		// Start transaction
		var beginRes connection.RPCResponse[models.UUID]
		err := connection.Send(conn, ctx, &beginRes, "begin")
		require.NoError(t, err, "begin should succeed")
		txnID := beginRes.Result
		require.NotNil(t, txnID, "begin should return txn UUID")

		// Create using txn at top level (correct format)
		res, err := sendInTransaction[[]queryResultWithSlice](conn, ctx, "query", txnID,
			"CREATE items:test2 SET value = 200",
			map[string]any{})
		require.NoError(t, err, "CREATE with txn at top level should succeed")
		require.NotNil(t, res.Result, "result should not be nil")

		// Cancel
		var cancelRes connection.RPCResponse[any]
		err = connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
		require.NoError(t, err, "cancel should succeed")

		// Check after cancel - item should not exist (rolled back)
		// Use queryResultWithAnyResult since Result can be empty string when no records
		var queryRes connection.RPCResponse[[]queryResultWithAnyResult]
		err = connection.Send(conn, ctx, &queryRes, "query",
			"SELECT * FROM items WHERE id = items:test2",
			map[string]any{})
		require.NoError(t, err, "SELECT query should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Greater(t, len(*queryRes.Result), 0, "should have query result")
		// Result can be empty slice, empty string, or populated slice
		result := (*queryRes.Result)[0].Result
		count := 0
		if arr, ok := result.([]any); ok {
			count = len(arr)
		}
		assert.Equal(t, 0, count, "canceled transaction data should be rolled back")
	})
}

// TestExplore_CRUDInTransaction tests CRUD operations within a transaction
func TestExplore_CRUDInTransaction(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_crud_txn", "crud_test", "products")

	conn := setupWSConnection(t, "explore_crud_txn", "crud_test")
	ctx := context.Background()

	// Start transaction
	var beginRes connection.RPCResponse[any]
	err := connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID := *beginRes.Result
	t.Logf("Transaction ID: %+v (type: %T)", txnID, txnID)

	// Test: create RPC within transaction using txn map (expected to fail - txn must be top-level)
	t.Run("create with txn map param fails", func(t *testing.T) {
		// Try create with txn as last param (map) - this should fail because txn must be top-level CBOR field
		var createRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &createRes, "create",
			"products:widget",
			map[string]any{"name": "Widget", "price": 9.99},
			map[string]any{"txn": txnID})
		// The create itself may succeed but won't be in the transaction context
		// Log the result for information
		t.Logf("create with txn map: err=%v, result=%+v", err, createRes.Result)
	})

	// Test: select RPC within transaction using txn map (expected to not use transaction context)
	t.Run("select with txn map param", func(t *testing.T) {
		var selectRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &selectRes, "select",
			"products",
			map[string]any{"txn": txnID})
		// The select may succeed but won't be in the transaction context
		t.Logf("select with txn map: err=%v, result=%+v", err, selectRes.Result)
	})

	// Test: update RPC within transaction using txn map (expected to not use transaction context)
	t.Run("update with txn map param", func(t *testing.T) {
		var updateRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &updateRes, "update",
			"products:widget",
			map[string]any{"name": "Super Widget", "price": 19.99},
			map[string]any{"txn": txnID})
		// The update may succeed but won't be in the transaction context
		t.Logf("update with txn map: err=%v, result=%+v", err, updateRes.Result)
	})

	// Cancel to clean up - this should succeed
	t.Run("cancel transaction", func(t *testing.T) {
		var cancelRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
		require.NoError(t, err, "cancel with direct txnID param should succeed")
	})
}

// TestExplore_QueryWithTxnTopLevel tests query with txn as a top-level CBOR field
// This is the key test to verify that SurrealDB v3 expects txn at the request level, not in params
func TestExplore_QueryWithTxnTopLevel(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn_toplevel", "txn_toplevel_test", "records")

	conn := setupWSConnection(t, "explore_txn_toplevel", "txn_toplevel_test")
	ctx := context.Background()

	type queryResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// Start a transaction
	var beginRes connection.RPCResponse[models.UUID]
	err := connection.Send(conn, ctx, &beginRes, "begin")
	require.NoError(t, err)
	txnID := beginRes.Result
	t.Logf("Transaction ID: %+v (type: %T)", txnID, txnID)

	// Test: Create record using txn as top-level CBOR field
	t.Run("create with txn at top level", func(t *testing.T) {
		res, err := sendInTransaction[[]queryResult](
			conn, ctx, "query", txnID,
			"CREATE records:test1 SET value = 'created in txn'",
			map[string]any{},
		)
		require.NoError(t, err, "CREATE with txn top-level should succeed")
		require.NotNil(t, res.Result, "result should not be nil")
		require.Len(t, *res.Result, 1, "should have one query result")
		assert.Equal(t, "OK", (*res.Result)[0].Status, "query should succeed")
		t.Logf("CREATE with txn top-level succeeded: %+v", res.Result)
	})

	// Test: Select within transaction using txn at top level
	t.Run("select with txn at top level", func(t *testing.T) {
		res, err := sendInTransaction[[]queryResult](
			conn, ctx, "query", txnID,
			"SELECT * FROM records",
			map[string]any{},
		)
		require.NoError(t, err, "SELECT with txn top-level should succeed")
		require.NotNil(t, res.Result, "result should not be nil")
		require.Len(t, *res.Result, 1, "should have one query result")
		assert.Equal(t, "OK", (*res.Result)[0].Status, "query should succeed")
		assert.Len(t, (*res.Result)[0].Result, 1, "should see 1 record within transaction")
		t.Logf("SELECT with txn top-level result: %+v", res.Result)
	})

	// Test: Check isolation - select from DIFFERENT connection should NOT see uncommitted data
	t.Run("select from different connection should not see uncommitted", func(t *testing.T) {
		// Use a different connection to check isolation
		conn2 := setupWSConnection(t, "explore_txn_toplevel", "txn_toplevel_test")

		var queryRes connection.RPCResponse[any]
		err := connection.Send(conn2, ctx, &queryRes, "query",
			"SELECT * FROM records",
			map[string]any{})
		require.NoError(t, err, "SELECT from conn2 should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")

		// Parse the result to get count
		results, ok := (*queryRes.Result).([]any)
		require.True(t, ok, "result should be an array")
		require.Len(t, results, 1, "should have one query result")

		firstResult, ok := results[0].(map[string]any)
		require.True(t, ok, "first result should be a map")

		resultArr, ok := firstResult["result"].([]any)
		count := 0
		if ok {
			count = len(resultArr)
		}
		assert.Equal(t, 0, count, "uncommitted data should not be visible from different connection (isolation)")
		t.Logf("SELECT from different connection count: %d", count)
	})

	// Test: Cancel and verify rollback
	t.Run("cancel and verify rollback", func(t *testing.T) {
		var cancelRes connection.RPCResponse[any]
		err := connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
		require.NoError(t, err, "cancel should succeed")
		t.Logf("Transaction canceled")

		// Check data is rolled back - use a different connection to avoid any session state issues
		conn3 := setupWSConnection(t, "explore_txn_toplevel", "txn_toplevel_test")
		var queryRes connection.RPCResponse[any]
		err = connection.Send(conn3, ctx, &queryRes, "query",
			"SELECT * FROM records",
			map[string]any{})
		require.NoError(t, err, "SELECT after cancel should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")

		// Parse the result to get count
		results, ok := (*queryRes.Result).([]any)
		require.True(t, ok, "result should be an array")
		require.Len(t, results, 1, "should have one query result")

		firstResult, ok := results[0].(map[string]any)
		require.True(t, ok, "first result should be a map")

		resultArr, ok := firstResult["result"].([]any)
		count := 0
		if ok {
			count = len(resultArr)
		}
		assert.Equal(t, 0, count, "canceled transaction data should be rolled back")
		t.Logf("SELECT after cancel count: %d", count)
	})

	// Test: New transaction with commit
	t.Run("create and commit with txn at top level", func(t *testing.T) {
		// Start new transaction
		var beginRes2 connection.RPCResponse[models.UUID]
		err := connection.Send(conn, ctx, &beginRes2, "begin")
		require.NoError(t, err, "begin should succeed")
		txnID2 := beginRes2.Result
		require.NotNil(t, txnID2, "transaction ID should not be nil")
		t.Logf("New Transaction ID: %+v", txnID2)

		// Create using txn at top level
		res, err := sendInTransaction[[]queryResult](
			conn, ctx, "query", txnID2,
			"CREATE records:committed SET value = 'will be committed'",
			map[string]any{},
		)
		require.NoError(t, err, "CREATE in transaction should succeed")
		require.NotNil(t, res.Result, "result should not be nil")
		t.Logf("CREATE succeeded: %+v", res.Result)

		// Commit
		var commitRes connection.RPCResponse[any]
		err = connection.Send(conn, ctx, &commitRes, "commit", txnID2)
		require.NoError(t, err, "commit should succeed")
		t.Logf("Transaction committed")

		// Verify data persisted
		var queryRes connection.RPCResponse[[]queryResult]
		err = connection.Send(conn, ctx, &queryRes, "query",
			"SELECT * FROM records",
			map[string]any{})
		require.NoError(t, err, "SELECT after commit should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Len(t, *queryRes.Result, 1, "should have one query result")

		count := len((*queryRes.Result)[0].Result)
		assert.Equal(t, 1, count, "committed data should be visible")
		t.Logf("SELECT after commit count: %d", count)
	})
}

// TestExplore_MultipleTransactions tests multiple sequential transactions
// Note: Concurrent transactions on the same connection may conflict even when writing to different records.
// This is due to SurrealDB's optimistic concurrency control.
// Uses the correct parameter formats discovered in Phase 1:
// - txn at top-level CBOR field for queries
// - txn as direct param for commit/cancel
func TestExplore_MultipleTransactions(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = MustNew("explore_multi_txn", "multi_txn_test", "accounts")

	conn := setupWSConnection(t, "explore_multi_txn", "multi_txn_test")
	ctx := context.Background()

	type queryResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// Test sequential transactions - first transaction
	t.Run("first transaction creates and commits", func(t *testing.T) {
		var begin1Res connection.RPCResponse[models.UUID]
		err := connection.Send(conn, ctx, &begin1Res, "begin")
		require.NoError(t, err, "begin txn1 should succeed")
		txn1 := begin1Res.Result
		require.NotNil(t, txn1, "txn1 ID should not be nil")
		t.Logf("Transaction 1: %+v", txn1)

		// Write in txn1 using txn at top level
		res1, err := sendInTransaction[[]queryResult](conn, ctx, "query", txn1,
			"CREATE accounts:a SET balance = 100",
			map[string]any{})
		require.NoError(t, err, "CREATE in txn1 should succeed")
		require.NotNil(t, res1.Result, "txn1 result should not be nil")
		t.Logf("Create in txn1: result=%+v", res1)

		// Commit txn1
		var commit1Res connection.RPCResponse[any]
		err = connection.Send(conn, ctx, &commit1Res, "commit", txn1)
		require.NoError(t, err, "commit txn1 should succeed")
		t.Logf("Commit txn1: success")
	})

	// Test sequential transactions - second transaction
	t.Run("second transaction creates and commits", func(t *testing.T) {
		var begin2Res connection.RPCResponse[models.UUID]
		err := connection.Send(conn, ctx, &begin2Res, "begin")
		require.NoError(t, err, "begin txn2 should succeed")
		txn2 := begin2Res.Result
		require.NotNil(t, txn2, "txn2 ID should not be nil")
		t.Logf("Transaction 2: %+v", txn2)

		// Write in txn2 using txn at top level
		res2, err := sendInTransaction[[]queryResult](conn, ctx, "query", txn2,
			"CREATE accounts:b SET balance = 200",
			map[string]any{})
		require.NoError(t, err, "CREATE in txn2 should succeed")
		require.NotNil(t, res2.Result, "txn2 result should not be nil")
		t.Logf("Create in txn2: result=%+v", res2)

		// Commit txn2
		var commit2Res connection.RPCResponse[any]
		err = connection.Send(conn, ctx, &commit2Res, "commit", txn2)
		require.NoError(t, err, "commit txn2 should succeed")
		t.Logf("Commit txn2: success")
	})

	// Verify both records exist
	t.Run("verify both records exist", func(t *testing.T) {
		var queryRes connection.RPCResponse[[]queryResult]
		err := connection.Send(conn, ctx, &queryRes, "query",
			"SELECT * FROM accounts", map[string]any{})
		require.NoError(t, err, "final SELECT should succeed")
		require.NotNil(t, queryRes.Result, "result should not be nil")
		require.Len(t, *queryRes.Result, 1, "should have one query result")

		count := len((*queryRes.Result)[0].Result)
		assert.Equal(t, 2, count, "both transactions' data should be visible after commit")
		t.Logf("Final SELECT count: %d", count)
	})
}

// TestExplore_TransactionOwnership tests what happens when trying to use a transaction
// started in one session from another session (transaction takeover attempt).
func TestExplore_TransactionOwnership(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions/transactions (requires v3+)", v)
	}

	_ = MustNew("explore_txn_ownership", "ownership_test", "items")

	conn := setupWSConnection(t, "explore_txn_ownership", "ownership_test")
	ctx := context.Background()

	type queryResult struct {
		Status string `cbor:"status"`
		Time   string `cbor:"time"`
		Result []any  `cbor:"result"`
	}

	// Create two sessions
	session1 := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	session2 := models.UUID{UUID: uuid.Must(uuid.NewV4())}

	// Setup session 1
	_, err := sendInSession[any](conn, ctx, "attach", &session1)
	require.NoError(t, err, "attach session1 should succeed")
	_, err = sendInSession[string](conn, ctx, "signin", &session1,
		map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err, "signin session1 should succeed")
	_, err = sendInSession[any](conn, ctx, "use", &session1,
		"explore_txn_ownership", "ownership_test")
	require.NoError(t, err, "use session1 should succeed")

	// Setup session 2
	_, err = sendInSession[any](conn, ctx, "attach", &session2)
	require.NoError(t, err, "attach session2 should succeed")
	_, err = sendInSession[string](conn, ctx, "signin", &session2,
		map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err, "signin session2 should succeed")
	_, err = sendInSession[any](conn, ctx, "use", &session2,
		"explore_txn_ownership", "ownership_test")
	require.NoError(t, err, "use session2 should succeed")

	t.Logf("Session 1: %s", session1.String())
	t.Logf("Session 2: %s", session2.String())

	// Start a transaction in session 1
	t.Run("start transaction in session 1", func(t *testing.T) {
		beginRes, err := sendInSession[models.UUID](conn, ctx, "begin", &session1)
		require.NoError(t, err, "begin in session1 should succeed")
		require.NotNil(t, beginRes.Result, "begin should return txn UUID")
		txnID := beginRes.Result
		t.Logf("Transaction started in session1: %+v", txnID)

		// Create a record in the transaction using session 1
		res, err := sendCustomRPC[[]queryResult](conn, ctx, "query", &session1, txnID,
			"CREATE items:test1 SET value = 'from session1'",
			map[string]any{})
		require.NoError(t, err, "CREATE in session1's txn should succeed")
		require.NotNil(t, res.Result, "result should not be nil")
		t.Logf("Created in session1's txn: %+v", res.Result)

		// Try to use the same transaction from session 2 (takeover attempt)
		// DISCOVERY: SurrealDB allows this! Transactions are connection-scoped, not session-scoped.
		t.Run("session 2 can use session 1's transaction", func(t *testing.T) {
			res2, err := sendCustomRPC[[]queryResult](conn, ctx, "query", &session2, txnID,
				"CREATE items:test2 SET value = 'from session2 takeover'",
				map[string]any{})
			// SurrealDB allows cross-session transaction usage
			require.NoError(t, err, "transaction takeover should be allowed (transactions are connection-scoped)")
			require.NotNil(t, res2.Result, "result should not be nil")
			t.Logf("Session2 using session1's txn succeeded: %+v", res2.Result)
		})

		// Try to commit the transaction from session 2
		// DISCOVERY: SurrealDB allows this! Any session can commit/cancel any transaction on the connection.
		t.Run("session 2 can commit session 1's transaction", func(t *testing.T) {
			// Note: commit/cancel use direct param, not session context
			res, err := sendInSession[any](conn, ctx, "commit", &session2, txnID)
			require.NoError(t, err, "cross-session commit should be allowed")
			t.Logf("Session2 committed session1's txn: %+v", res.Result)
		})

		// Try to cancel from session 1 (original owner) after session 2 committed
		// DISCOVERY: Transaction is already committed, so cancel fails with "Transaction not found"
		t.Run("session 1 cancel fails after commit", func(t *testing.T) {
			_, err := sendInSession[any](conn, ctx, "cancel", &session1, txnID)
			require.Error(t, err, "cancel should fail - transaction already committed")
			assert.Contains(t, err.Error(), "Transaction not found", "error should indicate txn not found")
			t.Logf("Session1 cancel failed as expected: %v", err)

			// Also try direct cancel (without session context) - same result
			var cancelRes connection.RPCResponse[any]
			err = connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
			require.Error(t, err, "direct cancel should also fail")
			t.Logf("Direct cancel also failed: %v", err)
		})
	})

	// Test: Transaction on default session, try to use from explicit session
	// DISCOVERY: This is also allowed - transactions are truly connection-scoped
	t.Run("default session transaction can be used from explicit session", func(t *testing.T) {
		// Start transaction on default session (no session field)
		var beginRes connection.RPCResponse[models.UUID]
		err := connection.Send(conn, ctx, &beginRes, "begin")
		require.NoError(t, err, "begin on default session should succeed")
		txnID := beginRes.Result
		t.Logf("Transaction on default session: %+v", txnID)

		// Try to use this transaction from session 2
		res, err := sendCustomRPC[[]queryResult](conn, ctx, "query", &session2, txnID,
			"CREATE items:default_txn_test SET value = 'takeover from default'",
			map[string]any{})
		require.NoError(t, err, "using default session's txn from explicit session should be allowed")
		require.NotNil(t, res.Result, "result should not be nil")
		t.Logf("Session2 using default session's txn succeeded: %+v", res.Result)

		// Clean up
		var cancelRes connection.RPCResponse[any]
		_ = connection.Send(conn, ctx, &cancelRes, "cancel", txnID)
	})

	// Cleanup sessions
	_, _ = sendInSession[any](conn, ctx, "detach", &session1)
	_, _ = sendInSession[any](conn, ctx, "detach", &session2)
}

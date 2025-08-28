package rews

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// testUnmarshaler is a test implementation of codec.Unmarshaler

// TestReliableLQ_requiresUnmarshaler tests that newReliableLQ panics with nil unmarshaler
func TestReliableLQ_requiresUnmarshaler(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))

	assert.Panics(t, func() {
		newReliableLQ(log, nil)
	}, "newReliableLQ should panic when unmarshaler is nil")
}

// TestReliableLQ_restoreLiveQueries tests that live queries are restored after reconnection
func TestReliableLQ_restoreLiveQueries(t *testing.T) {
	t.Run("restores live RPC query", func(t *testing.T) {
		mock := &mockRPCSender{}
		log := logger.New(slog.NewTextHandler(os.Stdout, nil))
		rlq := newReliableLQ(log, &models.CborUnmarshaler{})

		// Manually add a live query to simulate one that needs restoration
		rlq.liveQueries["test-id"] = &LiveQueryInfo{
			InternalID: "test-id",
			ExternalID: "test-id",
			Method:     "live",
			Params:     []any{"users", false},
		}

		// Restore live queries
		ctx := context.Background()
		// Create a mock provider (we won't actually use the channel for this test)
		mockProvider := newMockNotificationProvider()
		mockProvider.errors["test-id"] = fmt.Errorf("expected error - live query not accessible during test")
		err := rlq.restoreLiveQueries(ctx, mock, mockProvider, log)
		assert.Error(t, err) // We expect an error because mock doesn't return a valid UUID

		// Verify the live query was resent
		assert.Equal(t, 1, mock.sendCalled)
		assert.Equal(t, "live", mock.lastMethod)
		assert.Equal(t, []any{"users", false}, mock.lastParams)
	})

	t.Run("restores query RPC with LIVE SELECT", func(t *testing.T) {
		mock := &mockRPCSender{}
		log := logger.New(slog.NewTextHandler(os.Stdout, nil))
		rlq := newReliableLQ(log, &models.CborUnmarshaler{})

		// Manually add a LIVE SELECT query to simulate one that needs restoration
		liveSelectQuery := "LIVE SELECT * FROM products WHERE active = true"
		rlq.liveQueries["live-select-id"] = &LiveQueryInfo{
			InternalID: "live-select-id",
			ExternalID: "live-select-id",
			Method:     "query",
			Params:     []any{liveSelectQuery, nil},
		}

		// Restore live queries
		ctx := context.Background()
		mockProvider := newMockNotificationProvider()
		mockProvider.errors["live-select-id"] = fmt.Errorf("expected error - live query not accessible during test")
		err := rlq.restoreLiveQueries(ctx, mock, mockProvider, log)
		assert.Error(t, err) // We expect an error because mock doesn't return a valid UUID

		// Verify the LIVE SELECT query was resent
		assert.Equal(t, 1, mock.sendCalled)
		assert.Equal(t, "query", mock.lastMethod)
		assert.Equal(t, []any{liveSelectQuery, nil}, mock.lastParams)
	})

	t.Run("restores multiple queries of different types", func(t *testing.T) {
		mock := &mockRPCSender{}
		log := logger.New(slog.NewTextHandler(os.Stdout, nil))
		rlq := newReliableLQ(log, &models.CborUnmarshaler{})

		// Add both types of live queries
		rlq.liveQueries["live-1"] = &LiveQueryInfo{
			InternalID: "live-1",
			ExternalID: "live-1",
			Method:     "live",
			Params:     []any{"users", true},
		}
		rlq.liveQueries["live-select-1"] = &LiveQueryInfo{
			InternalID: "live-select-1",
			ExternalID: "live-select-1",
			Method:     "query",
			Params:     []any{"LIVE SELECT * FROM orders", map[string]any{"limit": 10}},
		}
		rlq.liveQueries["live-2"] = &LiveQueryInfo{
			InternalID: "live-2",
			ExternalID: "live-2",
			Method:     "live",
			Params:     []any{"products"},
		}

		// Restore live queries
		ctx := context.Background()
		mockProvider := newMockNotificationProvider()
		err := rlq.restoreLiveQueries(ctx, mock, mockProvider, log)
		assert.Error(t, err) // We expect an error because mock doesn't return a valid UUID

		// Since restoreLiveQueries now returns an error immediately when UUID extraction fails,
		// it will only make one Send call before returning the error
		assert.Equal(t, 1, mock.sendCalled)
	})
	t.Run("updates existing live query mappings", func(t *testing.T) {
		log := logger.New(slog.NewTextHandler(os.Stdout, nil))
		rlq := newReliableLQ(log, &models.CborUnmarshaler{})
		ctx := context.Background()

		// Setup initial state with existing mappings
		oldExternalID := "old-external-uuid"
		internalID := "stable-internal-uuid"

		rlq.liveQueries[internalID] = &LiveQueryInfo{
			InternalID: internalID,
			ExternalID: oldExternalID,
			Method:     "live",
			Params:     []any{"users", false},
		}

		// Create mock that returns a new UUID
		newExternalUUID := uuid.Must(uuid.NewV4())
		newExternalID := newExternalUUID.String()
		mock := &mockRPCSender{
			mockResult: func(method string) cbor.RawMessage {
				uuid := models.UUID{UUID: newExternalUUID}
				data, _ := cbor.Marshal(cbor.Tag{
					Number:  models.TagSpecBinaryUUID,
					Content: uuid,
				})
				return data
			},
		}

		mockProvider := newMockNotificationProvider()

		// Restore live queries
		err := rlq.restoreLiveQueries(ctx, mock, mockProvider, log)
		assert.NoError(t, err)

		// Verify the LiveQueryInfo was updated with new external ID
		assert.Equal(t, newExternalID, rlq.liveQueries[internalID].ExternalID)
		assert.Equal(t, internalID, rlq.liveQueries[internalID].InternalID)
	})

	t.Run("restores multiple queries with existing mappings", func(t *testing.T) {
		log := logger.New(slog.NewTextHandler(os.Stdout, nil))
		rlq := newReliableLQ(log, &models.CborUnmarshaler{})
		ctx := context.Background()

		// Setup initial state with multiple existing mappings
		queries := map[string]*LiveQueryInfo{
			"internal-1": {
				InternalID: "internal-1",
				ExternalID: "old-external-1",
				Method:     "live",
				Params:     []any{"users"},
			},
			"internal-2": {
				InternalID: "internal-2",
				ExternalID: "old-external-2",
				Method:     "query",
				Params:     []any{"LIVE SELECT * FROM products"},
			},
		}

		for id, info := range queries {
			rlq.liveQueries[id] = info
		}

		// Track which methods were called
		var callCount int
		methodCalls := make(map[string]int)

		queries["internal-1"].ExternalID = uuid.Must(uuid.NewV4()).String()
		queries["internal-2"].ExternalID = uuid.Must(uuid.NewV4()).String()

		// Create mock that returns different UUIDs for each call
		mock := &mockRPCSender{
			mockResult: func(method string) cbor.RawMessage {
				callCount++
				methodCalls[method]++

				var info *LiveQueryInfo
				for _, v := range rlq.liveQueries {
					if v.Method == method {
						info = v
						break
					}
				}

				uuid := models.UUID{UUID: uuid.Must(uuid.FromString(info.ExternalID))}

				// Return proper format based on method
				if method == "query" {
					type QueryResult struct {
						Status string          `json:"status"`
						Time   string          `json:"time"`
						Result cbor.RawMessage `json:"result"`
					}

					uuidData, _ := cbor.Marshal(cbor.Tag{
						Number:  models.TagSpecBinaryUUID,
						Content: uuid,
					})
					queryResults := []QueryResult{
						{
							Status: "OK",
							Time:   "1ms",
							Result: uuidData,
						},
					}

					data, _ := cbor.Marshal(queryResults)
					return data
				} else {
					data, _ := cbor.Marshal(cbor.Tag{
						Number:  models.TagSpecBinaryUUID,
						Content: uuid,
					})
					return data
				}
			},
		}

		mockProvider := newMockNotificationProvider()

		// Restore live queries
		err := rlq.restoreLiveQueries(ctx, mock, mockProvider, log)
		assert.NoError(t, err)

		// Verify all queries were restored
		assert.Equal(t, 2, callCount)
		assert.Equal(t, 1, methodCalls["live"])
		assert.Equal(t, 1, methodCalls["query"])

		assert.Equal(t, queries, rlq.liveQueries)
	})
}

// TestReliableLQ_handleSend tests the handleSend method's behavior
func TestReliableLQ_handleSend(t *testing.T) {
	// QueryResult represents the response from a query method
	type QueryResult struct {
		Status string          `json:"status"`
		Time   string          `json:"time"`
		Result cbor.RawMessage `json:"result"`
	}

	// Create response for live method (direct UUID)
	liveUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	liveData, _ := cbor.Marshal(cbor.Tag{
		Number:  models.TagSpecBinaryUUID,
		Content: liveUUID,
	})
	liveResp := &connection.RPCResponse[cbor.RawMessage]{
		Result: (*cbor.RawMessage)(&liveData),
	}

	// Create response for query method (array of QueryResult with UUID)
	queryUUID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	queryUUIDData, _ := cbor.Marshal(cbor.Tag{
		Number:  models.TagSpecBinaryUUID,
		Content: queryUUID,
	})
	queryResults := []QueryResult{
		{
			Status: "OK",
			Time:   "1ms",
			Result: queryUUIDData,
		},
	}
	queryData, _ := cbor.Marshal(queryResults)
	queryResp := &connection.RPCResponse[cbor.RawMessage]{
		Result: (*cbor.RawMessage)(&queryData),
	}

	tests := []struct {
		name        string
		method      string
		params      []any
		resp        *connection.RPCResponse[cbor.RawMessage]
		handled     bool
		sendCalled  int
		lastMethod  string
		lastParams  []any
		description string
	}{
		{
			name:        "handles live RPC",
			method:      "live",
			params:      []any{"users"},
			handled:     true,
			sendCalled:  1,
			lastMethod:  "live",
			lastParams:  []any{"users"},
			resp:        liveResp,
			description: "Should handle 'live' method calls",
		},
		{
			name:        "handles query RPC with LIVE SELECT",
			method:      "query",
			params:      []any{"LIVE SELECT * FROM products WHERE active = true"},
			handled:     true,
			sendCalled:  1,
			lastMethod:  "query",
			lastParams:  []any{"LIVE SELECT * FROM products WHERE active = true"},
			resp:        queryResp,
			description: "Should handle 'query' method with LIVE SELECT statement",
		},
		{
			name:        "does not handle query RPC with regular SELECT",
			method:      "query",
			params:      []any{"SELECT * FROM products WHERE active = true"},
			handled:     false,
			sendCalled:  0,
			description: "Should not handle 'query' method with regular SELECT statement",
		},
		{
			name:        "does not handle select RPC",
			method:      "select",
			params:      []any{"users", "123"},
			handled:     false,
			sendCalled:  0,
			description: "Should not handle 'select' method calls",
		},
		{
			name:        "handles case-insensitive LIVE SELECT",
			method:      "query",
			params:      []any{"live select * from users"},
			handled:     true,
			sendCalled:  1,
			lastMethod:  "query",
			lastParams:  []any{"live select * from users"},
			resp:        queryResp,
			description: "Should handle LIVE SELECT in lowercase",
		},
		{
			name:        "handles LIVE SELECT with whitespace",
			method:      "query",
			params:      []any{"  LIVE SELECT  * FROM users  "},
			handled:     true,
			sendCalled:  1,
			lastMethod:  "query",
			lastParams:  []any{"  LIVE SELECT  * FROM users  "},
			resp:        queryResp,
			description: "Should handle LIVE SELECT with extra whitespace",
		},
		{
			name:        "does not handle empty query params",
			method:      "query",
			params:      []any{},
			handled:     false,
			sendCalled:  0,
			description: "Should not handle query with empty params",
		},
		{
			name:        "does not handle create RPC",
			method:      "create",
			params:      []any{"users", map[string]any{"name": "test"}},
			handled:     false,
			sendCalled:  0,
			description: "Should not handle 'create' method calls",
		},
		{
			name:        "does not handle update RPC",
			method:      "update",
			params:      []any{"users:123", map[string]any{"name": "test"}},
			handled:     false,
			sendCalled:  0,
			description: "Should not handle 'update' method calls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRPCSender{
				mockResult: func(method string) cbor.RawMessage {
					if tt.resp != nil && tt.resp.Result != nil {
						return *tt.resp.Result
					}
					return cbor.RawMessage{}
				},
			}
			log := logger.New(slog.NewTextHandler(os.Stdout, nil))
			rlq := newReliableLQ(log, &models.CborUnmarshaler{})
			ctx := context.Background()

			handled, resp, err := rlq.handleSend(ctx, tt.method, tt.params, mock, log)

			assert.NoError(t, err, tt.description)
			assert.Equal(t, tt.handled, handled, tt.description+" - handled flag")
			assert.Equal(t, tt.sendCalled, mock.sendCalled, tt.description+" - sendCalled")
			assert.Equal(t, tt.lastMethod, mock.lastMethod, tt.description+" - method")
			assert.Equal(t, tt.lastParams, mock.lastParams, tt.description+" - params")
			assert.Equal(t, tt.resp, resp, tt.description+" - response")
		})
	}
}

// TestReliableLQ_handleSend_tracking tests that handleSend properly tracks live queries
func TestReliableLQ_handleSend_tracking(t *testing.T) {
	// Setup
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	rlq := newReliableLQ(log, &models.CborUnmarshaler{})
	ctx := context.Background()

	// Create a mock that returns a UUID in the response
	testUUID := uuid.Must(uuid.NewV4())
	uuidStr := testUUID.String()
	mock := &mockRPCSender{
		mockResult: func(method string) cbor.RawMessage {
			// Return a mock UUID response as models.UUID
			uuid := models.UUID{UUID: testUUID}
			data, _ := cbor.Marshal(cbor.Tag{Number: models.TagSpecBinaryUUID, Content: uuid})
			return data
		},
	}

	// Test live method tracking
	t.Run("tracks live query", func(t *testing.T) {
		params := []any{"users", false}
		handled, resp, err := rlq.handleSend(ctx, "live", params, mock, log)

		assert.True(t, handled)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify the live query was tracked
		assert.Len(t, rlq.liveQueries, 1)
		assert.Contains(t, rlq.liveQueries, uuidStr)
		info := rlq.liveQueries[uuidStr]
		assert.NotNil(t, info)
		assert.Equal(t, "live", info.Method)
		assert.Equal(t, params, info.Params)
	})

	// Clear tracked queries
	rlq.liveQueries = make(map[string]*LiveQueryInfo)

	// Test LIVE SELECT tracking
	t.Run("tracks LIVE SELECT query", func(t *testing.T) {
		query := "LIVE SELECT * FROM products"
		params := []any{query, nil}
		liveSelectTestUUID := uuid.Must(uuid.NewV4())
		liveSelectUUIDStr := liveSelectTestUUID.String()

		// For LIVE SELECT, the response is an array of QueryResult objects
		mock.mockResult = func(method string) cbor.RawMessage {
			// Create a QueryResult array with the UUID in the Result field
			type QueryResult struct {
				Status string          `json:"status"`
				Time   string          `json:"time"`
				Result cbor.RawMessage `json:"result"`
			}

			uuidData, _ := cbor.Marshal(cbor.Tag{Number: models.TagSpecBinaryUUID, Content: liveSelectTestUUID})
			queryResults := []QueryResult{
				{
					Status: "OK",
					Time:   "1ms",
					Result: uuidData,
				},
			}

			data, _ := cbor.Marshal(queryResults)
			return data
		}

		handled, resp, err := rlq.handleSend(ctx, "query", params, mock, log)

		assert.True(t, handled)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify the LIVE SELECT was tracked
		assert.Len(t, rlq.liveQueries, 1)
		assert.Contains(t, rlq.liveQueries, liveSelectUUIDStr)
		info := rlq.liveQueries[liveSelectUUIDStr]
		assert.NotNil(t, info)
		assert.Equal(t, "query", info.Method)
		assert.Equal(t, params, info.Params)
	})
}

// TestReliableLQ_recordLiveQueryFromResponse_errors tests error handling
func TestReliableLQ_recordLiveQueryFromResponse_errors(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	rlq := newReliableLQ(log, &models.CborUnmarshaler{})

	tests := []struct {
		name        string
		method      string
		params      []any
		result      cbor.RawMessage
		expectError string
	}{
		{
			name:        "returns error for invalid UUID in live method",
			method:      "live",
			params:      []any{"users"},
			result:      cbor.RawMessage([]byte{0x01, 0x02}), // Invalid CBOR
			expectError: "failed to decode live query UUID",
		},
		{
			name:        "returns error for invalid CBOR in query method",
			method:      "query",
			params:      []any{"LIVE SELECT * FROM users"},
			result:      cbor.RawMessage([]byte{0x01, 0x02}), // Invalid CBOR
			expectError: "failed to decode query results array",
		},
		{
			name:        "returns error for empty result in query method",
			method:      "query",
			params:      []any{"LIVE SELECT * FROM users"},
			result:      cbor.RawMessage{},
			expectError: "failed to decode query results array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rlq.recordLiveQueryFromResponse(&tt.result, tt.method, tt.params, log)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// mockRPCSender is used for testing send operations
type mockRPCSender struct {
	sendCalled int
	lastMethod string
	lastParams []any
	mockResult func(method string) cbor.RawMessage
}

func (m *mockRPCSender) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	m.sendCalled++
	m.lastMethod = method
	m.lastParams = params

	// Return a mock response
	var result cbor.RawMessage
	if m.mockResult != nil {
		result = m.mockResult(method)
	} else {
		// Default empty response
		result = cbor.RawMessage{}
	}

	return &connection.RPCResponse[cbor.RawMessage]{
		Result: &result,
	}, nil
}

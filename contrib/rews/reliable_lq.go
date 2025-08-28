package rews

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const (
	// methodLive is the RPC method name for live queries
	methodLive = "live"
	// methodQuery is the RPC method name for query operations
	methodQuery = "query"
)

// NotificationCloser is an interface for closing live notifications
type NotificationCloser interface {
	CloseLiveNotifications(id string) error
}

// RPCSender is an interface for sending RPC requests
type RPCSender interface {
	Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error)
}

// reliableLQ encapsulates all the state and functionality for reliable live query management
// across reconnections. It maintains mappings between stable internal UUIDs and changing
// external UUIDs, and handles the restoration of live queries after reconnection.
type reliableLQ struct {
	// liveQueries stores information about active live queries
	// Maps internal (stable) UUID -> LiveQueryInfo
	liveQueries map[string]*LiveQueryInfo

	// liveQueriesMu protects access to liveQueries map
	liveQueriesMu sync.RWMutex

	// router handles notification routing between external and internal UUIDs
	router *NotificationRouter

	// unmarshaler is used to unmarshal CBOR data
	unmarshaler codec.Unmarshaler
}

// newReliableLQ creates and initializes a new reliableLQ instance
func newReliableLQ(log logger.Logger, unmarshaler codec.Unmarshaler) *reliableLQ {
	if unmarshaler == nil {
		panic("reliableLQ requires a valid unmarshaler")
	}

	return &reliableLQ{
		liveQueries: make(map[string]*LiveQueryInfo),
		router:      NewNotificationRouter(log),
		unmarshaler: unmarshaler,
	}
}

// LiveQueryInfo stores information about a live query for restoration after reconnection
type LiveQueryInfo struct {
	// InternalID is the stable UUID that the consumer uses
	InternalID string
	// ExternalID is the current UUID from the server (changes on reconnect)
	ExternalID string
	// Method is the RPC method used to create the live query ("live" or "query")
	Method string
	// Params are the parameters used to create the live query
	Params []any
}

// isLiveSelectQuery checks if a query string is a LIVE SELECT statement
func (rlq *reliableLQ) isLiveSelectQuery(query string) bool {
	// Simple check for LIVE SELECT queries
	// This is a basic implementation - could be made more robust
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(trimmed, "LIVE SELECT")
}

// recordLiveQueryFromResponse extracts the UUID from a response and records the live query.
func (rlq *reliableLQ) recordLiveQueryFromResponse(result *cbor.RawMessage, method string, params []any, log logger.Logger) error {
	// Try to extract the UUID from the result
	var liveID string

	// Special handling for query method with LIVE SELECT
	// The response structure might be different
	if method == methodQuery {
		// For LIVE SELECT through query method, the result is an array of QueryResult
		// We need to extract the UUID from the first QueryResult's Result field
		type QueryResult struct {
			Status string          `json:"status"`
			Time   string          `json:"time"`
			Result cbor.RawMessage `json:"result"`
		}

		var queryResults []QueryResult
		if err := rlq.unmarshaler.Unmarshal(*result, &queryResults); err != nil {
			return fmt.Errorf("failed to decode query results array: %w", err)
		}

		if len(queryResults) == 0 {
			return fmt.Errorf("query returned no results")
		}

		// Extract UUID from the first result
		var uuid models.UUID
		if err := rlq.unmarshaler.Unmarshal(queryResults[0].Result, &uuid); err != nil {
			return fmt.Errorf("failed to decode live query UUID from first query result: %w", err)
		}
		liveID = uuid.String()
		log.Debug("Extracted UUID from query result", "liveID", liveID, "method", method)
	} else {
		// For regular live method, unmarshal directly as UUID
		var uuid models.UUID
		if err := rlq.unmarshaler.Unmarshal(*result, &uuid); err != nil {
			return fmt.Errorf("failed to decode live query UUID: %w", err)
		}
		liveID = uuid.String()
		log.Debug("Extracted UUID from live result", "liveID", liveID, "method", method)
	}

	if liveID == "" {
		// This shouldn't happen - we expect a valid UUID to be extracted
		return fmt.Errorf("extracted UUID is empty for %s method", method)
	}

	rlq.recordLiveQuery(liveID, method, params, log)
	return nil
}

// recordLiveQuery records a live query for restoration after reconnection
func (rlq *reliableLQ) recordLiveQuery(liveID, method string, params []any, log logger.Logger) {
	rlq.liveQueriesMu.Lock()
	defer rlq.liveQueriesMu.Unlock()

	info := &LiveQueryInfo{
		InternalID: liveID, // Initially, internal and external are the same
		ExternalID: liveID,
		Method:     method,
		Params:     params,
	}

	rlq.liveQueries[liveID] = info

	log.Debug("Recorded live query", "id", liveID, "method", method)
}

// send performs the actual send operation and tracks live queries
func (rlq *reliableLQ) send(
	ctx context.Context,
	method string,
	params []any,
	sender RPCSender,
	log logger.Logger,
) (*connection.RPCResponse[cbor.RawMessage], error) {
	// Send the request through the sender
	resp, err := sender.Send(ctx, method, params...)
	if err != nil {
		return nil, err
	}

	// Only process successful responses with results
	if resp != nil && resp.Error == nil && resp.Result != nil {
		// Record the live query for restoration
		if err := rlq.recordLiveQueryFromResponse(resp.Result, method, params, log); err != nil {
			// Log the error but don't fail the send operation
			// The query was successful, we just couldn't track it for restoration
			log.Error("Failed to record live query for restoration", "method", method, "error", err)
		}
	}

	return resp, nil
}

// handleSend intercepts and handles live query sends, tracking them for restoration
// Returns true if the send was handled, false if it should be passed through
func (rlq *reliableLQ) handleSend(
	ctx context.Context,
	method string,
	params []any,
	sender RPCSender,
	log logger.Logger,
) (bool, *connection.RPCResponse[cbor.RawMessage], error) {
	// Check if this is a live query that needs to be tracked
	shouldHandle := false
	if method == methodLive {
		shouldHandle = true
		log.Debug("Handling live method", "params", params)
	} else if method == methodQuery && len(params) > 0 {
		// Check if this is a LIVE SELECT query
		if query, ok := params[0].(string); ok {
			log.Debug("Checking query for LIVE SELECT")
			if rlq.isLiveSelectQuery(query) {
				shouldHandle = true
				log.Debug("Handling LIVE SELECT query")
			}
		}
	}

	// If it's not a live query, don't handle it
	if !shouldHandle {
		log.Debug("Not handling method", "method", method, "params", params)
		return false, nil, nil
	}

	// Use our send method that handles live query tracking
	log.Debug("Sending and tracking live query", "method", method)
	resp, err := rlq.send(ctx, method, params, sender, log)
	return true, resp, err
}

// liveNotifications handles getting live notifications with UUID mapping
func (rlq *reliableLQ) liveNotifications(
	id string,
	provider NotificationProvider,
) (chan connection.Notification, error) {
	// Check if this is an internal ID that needs mapping
	rlq.liveQueriesMu.RLock()
	info, exists := rlq.liveQueries[id]
	if !exists {
		rlq.liveQueriesMu.RUnlock()
		// This should not happen in normal usage - LiveNotifications should only be called
		// after a successful Live() or Query() with LIVE SELECT, which records the query
		return nil, fmt.Errorf("live query with ID %s not found - was Live() or LIVE SELECT query called first?", id)
	}

	externalID := info.ExternalID
	rlq.liveQueriesMu.RUnlock()

	// Use setupNotificationRouting which will handle the routing setup and return the channel
	return rlq.router.SetupRouting(id, externalID, provider)
}

// closeLiveNotifications handles closing live notifications with UUID mapping
func (rlq *reliableLQ) closeLiveNotifications(closer NotificationCloser, id string) (string, error) {
	// Check if this is an internal ID that needs mapping
	rlq.liveQueriesMu.RLock()
	info, exists := rlq.liveQueries[id]
	if !exists {
		rlq.liveQueriesMu.RUnlock()
		// This should not happen in normal usage - CloseLiveNotifications should only be called
		// for live queries that were previously created and tracked
		return "", fmt.Errorf("live query with ID %s not found - was this live query previously created?", id)
	}

	externalID := info.ExternalID
	rlq.liveQueriesMu.RUnlock()

	// Close notifications on the underlying connection
	err := closer.CloseLiveNotifications(externalID)

	// Remove route from router
	rlq.router.RemoveRoute(id)

	// Remove from live queries tracking
	rlq.liveQueriesMu.Lock()
	delete(rlq.liveQueries, id)
	rlq.liveQueriesMu.Unlock()

	return externalID, err
}

// restoreLiveQueries re-establishes live queries after reconnection.
//
// This function does not fail-fast when a live query cannot be restored due to
// server errors (e.g., permission issues, query errors). However, it WILL return
// an error if we cannot extract UUIDs from successful responses, as this indicates
// a bug in either the SDK or SurrealDB.
func (rlq *reliableLQ) restoreLiveQueries(ctx context.Context, sender RPCSender, provider NotificationProvider, log logger.Logger) error {
	rlq.liveQueriesMu.RLock()
	queries := make([]*LiveQueryInfo, 0, len(rlq.liveQueries))
	for _, info := range rlq.liveQueries {
		queries = append(queries, info)
	}
	rlq.liveQueriesMu.RUnlock()

	for _, info := range queries {
		log.Debug("Restoring live query", "internal_id", info.InternalID, "method", info.Method)

		// Send the query to get a new external ID
		resp, err := sender.Send(ctx, info.Method, info.Params...)
		if err != nil {
			log.Error("Failed to restore live query", "internal_id", info.InternalID, "error", err)
			continue
		}

		if resp.Error != nil {
			log.Error("Live query restoration returned error", "internal_id", info.InternalID, "error", resp.Error)
			continue
		}

		// Extract the new external ID from the response
		var newExternalID string
		if resp.Result != nil {
			switch info.Method {
			case methodLive:
				var uuid models.UUID
				if err := rlq.unmarshaler.Unmarshal(*resp.Result, &uuid); err == nil {
					newExternalID = uuid.String()
				}
			case methodQuery:
				type QueryResult struct {
					Status string          `json:"status"`
					Time   string          `json:"time"`
					Result cbor.RawMessage `json:"result"`
				}

				var queryResults []QueryResult
				if err := rlq.unmarshaler.Unmarshal(*resp.Result, &queryResults); err == nil {
					if len(queryResults) > 0 {
						var uuid models.UUID
						if err := rlq.unmarshaler.Unmarshal(queryResults[0].Result, &uuid); err == nil {
							newExternalID = uuid.String()
						}
					}
				}
			}
		}

		if newExternalID == "" {
			return fmt.Errorf("failed to extract UUID from restored live query response for %s (method: %s)",
				info.InternalID, info.Method)
		}

		// Update the mapping with the new external ID
		rlq.liveQueriesMu.Lock()
		if oldInfo, exists := rlq.liveQueries[info.InternalID]; exists {
			// Update with new external ID
			oldInfo.ExternalID = newExternalID

			log.Debug("Live query restored with new external ID",
				"internal_id", info.InternalID,
				"old_external", info.ExternalID,
				"new_external", newExternalID)
		}
		rlq.liveQueriesMu.Unlock()

		// Setup notification routing for the restored query
		if _, err := rlq.router.SetupRouting(info.InternalID, newExternalID, provider); err != nil {
			log.Error("Failed to setup notification routing for restored query",
				"internal_id", info.InternalID,
				"external_id", newExternalID,
				"error", err)
		}
	}

	return nil
}

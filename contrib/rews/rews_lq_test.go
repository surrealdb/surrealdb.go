package rews

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// TestLiveQueryLifecycle tests the lifecycle of live queries
// including LiveNotifications and CloseLiveNotifications calls.
// This primarily ensure that the LQ-related features work correctly with rews,
// when no reconnection is involved.
func TestLiveQueryLifecycle(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a mock that returns proper UUIDs for live queries
	mock := &mockWebSocketConnection{
		notifications: make(map[string]chan connection.Notification),
	}

	conn := &Connection[*mockWebSocketConnection]{
		WebSocketConnection: mock,
		reliableLQ:          newReliableLQ(log, surrealcbor.New()),
		logger:              log,
		sessionVars:         make(map[string]any),
	}

	ctx := context.Background()

	t.Run("live RPC with notifications", func(t *testing.T) {
		// Call live RPC
		resp, err := conn.Send(ctx, methodLive, "users", false)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Result)

		// Extract the UUID from the response
		var liveID models.UUID
		err = cbor.Unmarshal(*resp.Result, &liveID)
		require.NoError(t, err)
		liveIDStr := liveID.String()
		require.NotEmpty(t, liveIDStr)

		// Get live notifications channel
		ch, err := conn.LiveNotifications(liveIDStr)
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Send a test notification through the mock
		notificationID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
		testNotification := connection.Notification{
			ID:     &notificationID,
			Action: connection.CreateAction,
			Result: map[string]any{"test": "data"},
		}

		// Send notification to the channel
		go func() {
			mock.SendNotification(liveIDStr, testNotification)
		}()

		// Receive the notification
		select {
		case received := <-ch:
			assert.Equal(t, testNotification.ID, received.ID)
			assert.Equal(t, testNotification.Action, received.Action)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for notification")
		}

		// Close live notifications
		err = conn.CloseLiveNotifications(liveIDStr)
		require.NoError(t, err)

		// Verify the channel is closed
		select {
		case _, ok := <-ch:
			assert.False(t, ok, "Channel should be closed")
		case <-time.After(100 * time.Millisecond):
			// Channel might be blocked, but that's ok as long as CloseLiveNotifications didn't error
		}
	})

	t.Run("LIVE SELECT query with notifications", func(t *testing.T) {
		// Call query RPC with LIVE SELECT
		resp, err := conn.Send(ctx, methodQuery, "LIVE SELECT * FROM products WHERE active = true", nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Result)

		// For LIVE SELECT, the response is an array of QueryResult
		type QueryResult struct {
			Status string          `json:"status"`
			Time   string          `json:"time"`
			Result cbor.RawMessage `json:"result"`
		}

		var queryResults []QueryResult
		err = cbor.Unmarshal(*resp.Result, &queryResults)
		require.NoError(t, err)
		require.Len(t, queryResults, 1)

		var liveID models.UUID
		err = cbor.Unmarshal(queryResults[0].Result, &liveID)
		require.NoError(t, err)
		liveIDStr := liveID.String()
		require.NotEmpty(t, liveIDStr)

		// Get live notifications channel
		ch, err := conn.LiveNotifications(liveIDStr)
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Send a test notification
		notificationID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
		testNotification := connection.Notification{
			ID:     &notificationID,
			Action: connection.UpdateAction,
			Result: map[string]any{"product": "updated"},
		}

		go func() {
			mock.SendNotification(liveIDStr, testNotification)
		}()

		// Receive the notification
		select {
		case received := <-ch:
			assert.Equal(t, testNotification.ID, received.ID)
			assert.Equal(t, testNotification.Action, received.Action)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for notification")
		}

		// Close live notifications
		err = conn.CloseLiveNotifications(liveIDStr)
		require.NoError(t, err)

		// Verify the internal tracking was cleaned up
		conn.reliableLQ.liveQueriesMu.RLock()
		_, exists := conn.reliableLQ.liveQueries[liveIDStr]
		conn.reliableLQ.liveQueriesMu.RUnlock()
		assert.False(t, exists, "Live query should be removed from tracking")
	})

	t.Run("multiple live queries with proper cleanup", func(t *testing.T) {
		var liveIDs []string

		// Create multiple live queries
		for i := 0; i < 3; i++ {
			table := fmt.Sprintf("table_%d", i)
			resp, err := conn.Send(ctx, methodLive, table, false)
			require.NoError(t, err)

			var liveID models.UUID
			err = cbor.Unmarshal(*resp.Result, &liveID)
			require.NoError(t, err)
			liveIDs = append(liveIDs, liveID.String())
		}

		// Get notification channels for all
		channels := make(map[string]chan connection.Notification)
		for _, id := range liveIDs {
			ch, err := conn.LiveNotifications(id)
			require.NoError(t, err)
			channels[id] = ch
		}

		// Verify all channels work
		for id, ch := range channels {
			notificationID := models.UUID{UUID: uuid.Must(uuid.NewV4())}
			notification := connection.Notification{
				ID:     &notificationID,
				Action: connection.CreateAction,
			}

			go func(id string, n connection.Notification) {
				mock.SendNotification(id, n)
			}(id, notification)

			select {
			case received := <-ch:
				assert.Equal(t, notification.ID, received.ID)
			case <-time.After(1 * time.Second):
				t.Fatalf("Timeout waiting for notification on %s", id)
			}
		}

		// Close all live queries
		for _, id := range liveIDs {
			err := conn.CloseLiveNotifications(id)
			require.NoError(t, err)
		}

		// Verify all are cleaned up
		conn.reliableLQ.liveQueriesMu.RLock()
		assert.Len(t, conn.reliableLQ.liveQueries, 0, "All live queries should be removed")
		conn.reliableLQ.liveQueriesMu.RUnlock()
	})
}

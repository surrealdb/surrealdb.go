package rews

import (
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// createTestUUID creates a models.UUID for testing
func createTestUUID(name string) *models.UUID {
	// Create a deterministic UUID from string for testing
	id := uuid.NewV5(uuid.NamespaceURL, name)
	return &models.UUID{UUID: id}
}

// TestNewRouteValidation tests that newRoute panics with empty internalID
func TestNewRouteValidation(t *testing.T) {
	// Test that it panics with empty internalID
	assert.Panics(t, func() {
		newRoute("")
	}, "Should panic with empty internalID")

	// Test that it doesn't panic with valid internalID
	assert.NotPanics(t, func() {
		r := newRoute("valid-id")
		assert.NotNil(t, r)
		assert.Equal(t, "valid-id", r.internalID)
		assert.NotNil(t, r.internalCh)
	}, "Should not panic with valid internalID")
}

// TestSetExternalValidation tests that setExternal validates its parameters correctly
func TestSetExternalValidation(t *testing.T) {
	r := newRoute("test-route")

	// Test that it panics with empty externalID
	assert.Panics(t, func() {
		ch := make(chan connection.Notification)
		r.setExternal("", ch, nil)
	}, "Should panic when externalID is empty")

	// Test that it panics with nil externalCh
	assert.Panics(t, func() {
		r.setExternal("external-id", nil, nil)
	}, "Should panic when externalCh is nil")

	// Test that it doesn't panic when both are provided
	assert.NotPanics(t, func() {
		ch := make(chan connection.Notification)
		r.setExternal("external-id", ch, nil)
		// Clean up
		r.stopRoutingGoroutine()
	}, "Should not panic when both are provided")
}

// TestRouteStopRoutingGoroutine tests the stopRoutingGoroutine method
func TestRouteStopRoutingGoroutine(t *testing.T) {
	r := newRoute("test-route")

	// Initially, no goroutine is running
	assert.Panics(t, func() {
		r.stopRoutingGoroutine()
	}, "Should panic when stopping with no goroutine running")

	// Set up an external channel
	externalCh := make(chan connection.Notification, 1)
	r.setExternal("ext-1", externalCh, nil)

	// Now stopping should return true
	assert.NotPanics(t, func() {
		r.stopRoutingGoroutine()
	}, "Should not panic when stopping with goroutine running")
}

// TestRouteSetExternalMultipleTimes tests setting external multiple times
func TestRouteSetExternalMultipleTimes(t *testing.T) {
	r := newRoute("multi-test")

	// Create channels for notifications
	sentNotifications := []connection.Notification{
		{ID: createTestUUID("n1"), Action: connection.CreateAction},
		{ID: createTestUUID("n2"), Action: connection.UpdateAction},
		{ID: createTestUUID("n3"), Action: connection.DeleteAction},
	}

	// First external setup
	ext1 := make(chan connection.Notification, 10)
	r.setExternal("ext-1", ext1, nil)

	// Send a notification through first external
	ext1 <- sentNotifications[0]

	// Wait a bit for routing
	time.Sleep(50 * time.Millisecond)

	// Switch to second external (should stop first goroutine)
	ext2 := make(chan connection.Notification, 10)
	r.setExternal("ext-2", ext2, nil)

	// Send through second external
	ext2 <- sentNotifications[1]

	// Wait a bit for routing
	time.Sleep(50 * time.Millisecond)

	// Switch to third external
	ext3 := make(chan connection.Notification, 10)
	r.setExternal("ext-3", ext3, nil)

	// Send through third external
	ext3 <- sentNotifications[2]

	// Collect notifications from internal channel
	var received []connection.Notification
	timeout := time.After(200 * time.Millisecond)

	for {
		select {
		case notif := <-r.internalCh:
			received = append(received, notif)
		case <-timeout:
			// Done collecting
			goto done
		}
	}

done:
	// We should have received all notifications
	assert.Len(t, received, 3, "Should have received all 3 notifications")

	// Clean up
	r.stop()
}

// TestRouteSequentialReconnection tests sequential reconnection scenarios
// This simulates the real usage where setExternal is called sequentially during reconnections
func TestRouteSequentialReconnection(t *testing.T) {
	r := newRoute("reconnection-test")

	const numReconnections = 5
	const numMessages = 10

	totalReceived := 0
	done := make(chan struct{})

	// Start a reader goroutine to consume from internal channel
	go func() {
		for {
			select {
			case notif, ok := <-r.internalCh:
				if !ok {
					close(done)
					return
				}
				totalReceived++
				assert.NotNil(t, notif)
			case <-time.After(2 * time.Second):
				close(done)
				return
			}
		}
	}()

	// Simulate sequential reconnections
	for reconnect := 0; reconnect < numReconnections; reconnect++ {
		// Create a new external channel for this "connection"
		extCh := make(chan connection.Notification, numMessages)
		extID := fmt.Sprintf("connection-%d", reconnect)

		// Set this as the new external channel (simulating reconnection with new UUID)
		r.setExternal(extID, extCh, nil)

		// Send messages through this connection
		go func(connID int, ch chan connection.Notification) {
			for m := 0; m < numMessages; m++ {
				notification := connection.Notification{
					ID:     createTestUUID(fmt.Sprintf("c%d-m%d", connID, m)),
					Action: connection.CreateAction,
				}

				select {
				case ch <- notification:
					// Sent successfully
				case <-time.After(10 * time.Millisecond):
					// Timeout - connection might have changed
					return
				}

				// Small delay between messages
				time.Sleep(5 * time.Millisecond)
			}
		}(reconnect, extCh)

		// Simulate connection duration before next reconnection
		time.Sleep(100 * time.Millisecond)
	}

	// Give final messages time to be routed
	time.Sleep(200 * time.Millisecond)

	// Stop the route
	r.stop()

	// Wait for reader to finish
	<-done

	t.Logf("Received %d notifications from %d sequential reconnections", totalReceived, numReconnections)
	assert.Greater(t, totalReceived, 0, "Should have received at least some notifications")

	// We expect to receive most messages, though some from earlier connections might be lost
	// when reconnection happens (this is realistic behavior)
	expectedMin := numMessages * (numReconnections - 1) // At least messages from completed connections
	assert.GreaterOrEqual(t, totalReceived, expectedMin/2, "Should receive a reasonable number of messages")
}

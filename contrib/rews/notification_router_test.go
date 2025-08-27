package rews

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const (
	testInternalID = "internal-123"
	testExternalID = "external-456"
)

// testUUID creates a models.UUID for testing
func testUUID(name string) *models.UUID {
	// Create a deterministic UUID from string for testing
	id := uuid.NewV5(uuid.NamespaceURL, name)
	return &models.UUID{UUID: id}
}

// mockNotificationProvider is a mock implementation of NotificationProvider
type mockNotificationProvider struct {
	channels map[string]chan connection.Notification
	mu       sync.Mutex
	errors   map[string]error
}

func newMockNotificationProvider() *mockNotificationProvider {
	return &mockNotificationProvider{
		channels: make(map[string]chan connection.Notification),
		errors:   make(map[string]error),
	}
}

func (m *mockNotificationProvider) LiveNotifications(id string) (chan connection.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, exists := m.errors[id]; exists {
		return nil, err
	}

	if ch, exists := m.channels[id]; exists {
		return ch, nil
	}

	// Create a new channel for this ID
	ch := make(chan connection.Notification, 10)
	m.channels[id] = ch
	return ch, nil
}

func (m *mockNotificationProvider) SendNotification(id string, notification connection.Notification) error {
	m.mu.Lock()
	ch, exists := m.channels[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("no channel for id: %s", id)
	}

	select {
	case ch <- notification:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("timeout sending notification")
	}
}

func (m *mockNotificationProvider) CloseChannel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, exists := m.channels[id]; exists {
		close(ch)
		delete(m.channels, id)
	}
}

func (m *mockNotificationProvider) SetError(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[id] = err
}

// TestNotificationRouter_BasicRouting tests basic routing functionality
func TestNotificationRouter_BasicRouting(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	internalID := testInternalID
	externalID := testExternalID

	// Setup routing - this will create the channel and return it
	ch, err := router.SetupRouting(internalID, externalID, provider)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Send a notification through the external channel
	notification := connection.Notification{
		ID:     testUUID("test-notification"),
		Action: connection.CreateAction,
		Result: map[string]interface{}{"test": "data"},
	}

	err = provider.SendNotification(externalID, notification)
	require.NoError(t, err)

	// Receive the notification on the internal channel
	select {
	case received := <-ch:
		assert.Equal(t, notification.ID, received.ID)
		assert.Equal(t, notification.Action, received.Action)
		assert.Equal(t, notification.Result, received.Result)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for notification")
	}

	// Clean up
	router.RemoveRoute(internalID)
}

// TestNotificationRouter_UpdateRouting tests updating an existing route
func TestNotificationRouter_UpdateRouting(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	internalID := "internal-123"
	externalID1 := "external-456"
	externalID2 := "external-789"

	// Setup initial routing
	ch, err := router.SetupRouting(internalID, externalID1, provider)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Send notification through first external channel
	notification1 := connection.Notification{
		ID:     testUUID("notification-1"),
		Action: connection.CreateAction,
	}
	err = provider.SendNotification(externalID1, notification1)
	require.NoError(t, err)

	// Receive first notification
	select {
	case received := <-ch:
		assert.Equal(t, notification1.ID, received.ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for first notification")
	}

	// Update routing to new external ID - should return the same channel
	ch2, err := router.SetupRouting(internalID, externalID2, provider)
	require.NoError(t, err)
	require.Equal(t, ch, ch2, "Should return the same internal channel")

	// Allow time for old routing to stop
	time.Sleep(100 * time.Millisecond)

	// Send notification through second external channel
	notification2 := connection.Notification{
		ID:     testUUID("notification-2"),
		Action: connection.UpdateAction,
	}
	err = provider.SendNotification(externalID2, notification2)
	require.NoError(t, err)

	// Should receive notification from new external ID on same internal channel
	select {
	case received := <-ch:
		assert.Equal(t, notification2.ID, received.ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for second notification")
	}

	// Notifications to old external ID should not be routed
	notification3 := connection.Notification{
		ID:     testUUID("notification-3"),
		Action: connection.DeleteAction,
	}
	err = provider.SendNotification(externalID1, notification3)
	require.NoError(t, err)

	// Should not receive notification from old external ID
	select {
	case <-ch:
		t.Fatal("Should not receive notification from old external ID")
	case <-time.After(200 * time.Millisecond):
		// Expected timeout
	}

	// Clean up
	router.RemoveRoute(internalID)
}

// TestNotificationRouter_ChannelReuse tests that SetupRouting reuses channels
func TestNotificationRouter_ChannelReuse(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	internalID := "internal-123"
	externalID := "external-456"

	// First call should create channel
	ch1, err := router.SetupRouting(internalID, externalID, provider)
	require.NoError(t, err)
	require.NotNil(t, ch1)

	// Second call with same internal ID should return same channel
	ch2, err := router.SetupRouting(internalID, externalID, provider)
	require.NoError(t, err)
	require.NotNil(t, ch2)
	assert.Equal(t, ch1, ch2, "Should return the same channel")

	// Call with different external ID should still return same internal channel
	ch3, err := router.SetupRouting(internalID, "external-789", provider)
	require.NoError(t, err)
	assert.Equal(t, ch1, ch3, "Should return the same internal channel")

	// Clean up
	router.RemoveRoute(internalID)
}

// TestNotificationRouter_MultipleRoutes tests managing multiple routes
func TestNotificationRouter_MultipleRoutes(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	routes := []struct {
		internalID string
		externalID string
	}{
		{"internal-1", "external-1"},
		{"internal-2", "external-2"},
		{"internal-3", "external-3"},
	}

	// Setup multiple routes
	channels := make(map[string]chan connection.Notification)
	for _, route := range routes {
		ch, err := router.SetupRouting(route.internalID, route.externalID, provider)
		require.NoError(t, err)
		require.NotNil(t, ch)
		channels[route.internalID] = ch
	}

	// Send notifications to each external ID
	for i, route := range routes {
		notification := connection.Notification{
			ID:     testUUID(fmt.Sprintf("notification-%d", i)),
			Action: connection.CreateAction,
		}
		err := provider.SendNotification(route.externalID, notification)
		require.NoError(t, err)
	}

	// Verify each internal channel receives its notification
	for i, route := range routes {
		ch := channels[route.internalID]
		select {
		case received := <-ch:
			assert.Equal(t, testUUID(fmt.Sprintf("notification-%d", i)), received.ID)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("Timeout waiting for notification on %s", route.internalID)
		}
	}

	// Clean up all routes
	for _, route := range routes {
		router.RemoveRoute(route.internalID)
	}
}

// TestNotificationRouter_ProviderError tests handling of provider errors
func TestNotificationRouter_ProviderError(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	internalID := testInternalID
	externalID := testExternalID

	// Set an error for the external ID
	provider.SetError(externalID, fmt.Errorf("connection failed"))

	// Setup routing should fail but still return the channel
	ch, err := router.SetupRouting(internalID, externalID, provider)
	assert.NotNil(t, ch) // Channel is created even if provider fails
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

// TestNotificationRouter_ExternalChannelClose tests handling when external channel closes
func TestNotificationRouter_ExternalChannelClose(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	internalID := testInternalID
	externalID := testExternalID

	// Setup routing
	ch, err := router.SetupRouting(internalID, externalID, provider)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Verify we get the same channel when setting up routing again with same IDs
	ch2, err := router.SetupRouting(internalID, externalID, provider)
	require.NoError(t, err)
	require.Equal(t, ch, ch2)

	// Send a notification to verify it's working
	notification := connection.Notification{
		ID:     testUUID("test-notification"),
		Action: connection.CreateAction,
	}
	err = provider.SendNotification(externalID, notification)
	require.NoError(t, err)

	// Receive it
	select {
	case <-ch:
		// Good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for notification")
	}

	// Close the external channel
	provider.CloseChannel(externalID)

	// Wait a bit for the routing goroutine to notice
	time.Sleep(100 * time.Millisecond)

	// Internal channel should still exist but not receive new notifications
	// We can verify this by trying to setup routing again - should return same channel
	ch3, err := router.SetupRouting(internalID, externalID, provider)
	assert.NoError(t, err)
	assert.Equal(t, ch, ch3)

	// Clean up
	router.RemoveRoute(internalID)
}

// TestNotificationRouter_Close tests closing the router
func TestNotificationRouter_Close(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	// Setup multiple routes and keep track of channels
	channels := make(map[string]chan connection.Notification)
	for i := 0; i < 3; i++ {
		internalID := fmt.Sprintf("internal-%d", i)
		externalID := fmt.Sprintf("external-%d", i)
		ch, err := router.SetupRouting(internalID, externalID, provider)
		require.NoError(t, err)
		channels[internalID] = ch
	}

	// Close the router
	router.Close()

	// All channels should be closed
	for internalID, ch := range channels {
		select {
		case _, ok := <-ch:
			assert.False(t, ok, "Channel for %s should be closed", internalID)
		default:
			// Channel might be blocked, try with timeout
			select {
			case _, ok := <-ch:
				assert.False(t, ok, "Channel for %s should be closed", internalID)
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Channel for %s appears to be still open", internalID)
			}
		}
	}
}

// TestNotificationRouter_ConcurrentAccess tests concurrent operations
func TestNotificationRouter_ConcurrentAccess(t *testing.T) {
	log := logger.New(slog.NewTextHandler(os.Stdout, nil))
	router := NewNotificationRouter(log)
	provider := newMockNotificationProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// Start multiple goroutines doing various operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				internalID := fmt.Sprintf("internal-%d", id)
				externalID := fmt.Sprintf("external-%d-%d", id, j)

				// Setup routing
				ch, err := router.SetupRouting(internalID, externalID, provider)
				assert.NoError(t, err)
				assert.NotNil(t, ch)

				// Verify we get the same channel back when setting up again
				ch2, err := router.SetupRouting(internalID, externalID, provider)
				assert.NoError(t, err)
				assert.Equal(t, ch, ch2)

				time.Sleep(10 * time.Millisecond)
			}

			// Clean up
			router.RemoveRoute(fmt.Sprintf("internal-%d", id))
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Close router
	router.Close()
}

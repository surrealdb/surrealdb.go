package rews

import (
	"sync"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

// NotificationProvider is an interface for getting live notification channels
type NotificationProvider interface {
	LiveNotifications(id string) (chan connection.Notification, error)
}

// NotificationRouter manages routing of notifications from external (changing) UUIDs
// to internal (stable) UUIDs after reconnection
type NotificationRouter struct {
	// routes maps internal UUID -> routing info
	routes map[string]*route
	// routesMu protects access to routes map
	routesMu sync.RWMutex

	// logger for debugging
	logger logger.Logger
}

// NewNotificationRouter creates a new NotificationRouter
func NewNotificationRouter(log logger.Logger) *NotificationRouter {
	return &NotificationRouter{
		routes: make(map[string]*route),
		logger: log,
	}
}

// SetupRouting sets up routing from an external UUID to an internal UUID
// It creates the internal channel if it doesn't exist and routes notifications
// from the external channel to the internal channel.
// This handles both cases:
// 1. When internalID == externalID (initial setup)
// 2. When internalID != externalID (after reconnection)
func (nr *NotificationRouter) SetupRouting(
	internalID, externalID string,
	provider NotificationProvider,
) (chan connection.Notification, error) {
	nr.routesMu.Lock()
	defer nr.routesMu.Unlock()

	// Get or create route
	r, exists := nr.routes[internalID]
	if !exists {
		// Create new route with internal channel
		r = newRoute(internalID)
		nr.routes[internalID] = r
		nr.logger.Debug("Created new route", "internal_id", internalID)
	}

	// Get external channel from provider
	externalCh, err := provider.LiveNotifications(externalID)
	if err != nil {
		nr.logger.Error("Failed to get notifications for external ID",
			"internal_id", internalID,
			"external_id", externalID,
			"error", err)
		return r.internalCh, err
	}

	// Update the route's external connection and manage goroutine lifecycle
	r.setExternal(externalID, externalCh, nr.logger)

	nr.logger.Debug("Notification routing setup",
		"internal_id", internalID,
		"external_id", externalID)

	return r.internalCh, nil
}

// RemoveRoute removes a routing entry and stops its goroutine
func (nr *NotificationRouter) RemoveRoute(internalID string) {
	nr.routesMu.Lock()
	defer nr.routesMu.Unlock()

	if r, exists := nr.routes[internalID]; exists {
		r.stop() // Stop the routing goroutine and close internal channel
		delete(nr.routes, internalID)

		nr.logger.Debug("Route removed", "internal_id", internalID)
	}
}

// Close stops all routing goroutines and closes all channels
func (nr *NotificationRouter) Close() {
	nr.routesMu.Lock()
	defer nr.routesMu.Unlock()

	for internalID, r := range nr.routes {
		r.stop() // Stop the routing goroutine and close internal channel
		delete(nr.routes, internalID)
		nr.logger.Debug("Route closed", "internal_id", internalID)
	}
}

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

// route represents a single notification routing path
type route struct {
	internalID string
	externalID string
	internalCh chan connection.Notification
	externalCh chan connection.Notification
	stopCh     chan struct{}
	wg         sync.WaitGroup
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
		r = &route{
			internalID: internalID,
			internalCh: make(chan connection.Notification, 100), // buffered to avoid blocking
			stopCh:     make(chan struct{}),
		}
		nr.routes[internalID] = r
		nr.logger.Debug("Created new route", "internal_id", internalID)
	}

	// Stop old routing if it exists and external ID is changing
	if r.externalCh != nil && r.externalID != externalID {
		nr.logger.Debug("Stopping old routing",
			"internal_id", internalID,
			"old_external_id", r.externalID,
			"new_external_id", externalID)
		close(r.stopCh)
		r.wg.Wait()
		r.stopCh = make(chan struct{})
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

	// Update route
	r.externalID = externalID
	r.externalCh = externalCh

	// Start routing goroutine only if needed
	if r.externalID != "" {
		r.wg.Add(1)
		go nr.routeNotifications(r)

		nr.logger.Debug("Notification routing setup",
			"internal_id", internalID,
			"external_id", externalID)
	}

	return r.internalCh, nil
}

// routeNotifications routes notifications from external to internal channel
func (nr *NotificationRouter) routeNotifications(r *route) {
	defer r.wg.Done()

	nr.logger.Debug("Starting notification routing",
		"internal_id", r.internalID,
		"external_id", r.externalID)

	for {
		select {
		case notification, ok := <-r.externalCh:
			if !ok {
				nr.logger.Debug("External channel closed",
					"internal_id", r.internalID,
					"external_id", r.externalID)
				return
			}

			select {
			case r.internalCh <- notification:
				// Successfully routed notification
			default:
				// Internal channel might be full or closed
				nr.logger.Warn("Failed to route notification, channel might be full",
					"internal_id", r.internalID)
			}

		case <-r.stopCh:
			nr.logger.Debug("Notification routing stopped",
				"internal_id", r.internalID,
				"external_id", r.externalID)
			return
		}
	}
}

// RemoveRoute removes a routing entry and stops its goroutine
func (nr *NotificationRouter) RemoveRoute(internalID string) {
	nr.routesMu.Lock()
	defer nr.routesMu.Unlock()

	if r, exists := nr.routes[internalID]; exists {
		if r.externalCh != nil {
			close(r.stopCh)
			r.wg.Wait()
		}
		close(r.internalCh)
		delete(nr.routes, internalID)

		nr.logger.Debug("Route removed", "internal_id", internalID)
	}
}

// Close stops all routing goroutines and closes all channels
func (nr *NotificationRouter) Close() {
	nr.routesMu.Lock()
	defer nr.routesMu.Unlock()

	for internalID, r := range nr.routes {
		if r.externalCh != nil {
			close(r.stopCh)
			r.wg.Wait()
		}
		close(r.internalCh)
		delete(nr.routes, internalID)
		nr.logger.Debug("Route closed", "internal_id", internalID)
	}
}

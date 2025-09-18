package rews

import (
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

// panicIfEmpty panics with the given message if the value is empty
func panicIfEmpty(value, message string) {
	if value == "" {
		panic(message)
	}
}

// panicIfNilNotificationCh panics with the given message if the value is nil
func panicIfNilNotificationCh(value chan connection.Notification, message string) {
	if value == nil {
		panic(message)
	}
}

// route represents a single notification routing path
type route struct {
	// Immutable fields - set once during creation, never changed
	internalID string                       // Stable ID that client uses
	internalCh chan connection.Notification // Stable channel that client reads from

	// Mutable fields - updated during reconnection when external UUID changes
	// Note: These fields should only be modified by a single goroutine (no concurrent access)
	// The routing goroutine receives copies of these values to avoid races
	externalID string                       // Changes on reconnection (new UUID from server)
	externalCh chan connection.Notification // Changes on reconnection (new channel from server)
	stopCh     chan struct{}                // Signal to stop routing goroutine
	stoppedCh  chan struct{}                // Closed by goroutine when it finishes
}

// newRoute creates a new route with the given internal ID.
// The internal channel is buffered to avoid blocking when routing notifications.
func newRoute(internalID string) *route {
	panicIfEmpty(internalID, "BUG: internalID is mandatory")

	return &route{
		internalID: internalID,
		internalCh: make(chan connection.Notification, 100), // buffered to avoid blocking
	}
}

// stop completely stops the route by stopping any running routing goroutine and closing the internal channel.
// This should be called when the route is being removed.
func (r *route) stop() {
	// First stop any running goroutine to ensure nothing is trying to send to internalCh
	r.stopRoutingGoroutine()

	// Now it's safe to close the internal channel since no goroutine is running
	close(r.internalCh)
}

// setExternal updates the external ID and channel, managing the routing goroutine lifecycle.
// If an existing goroutine is running with a different external ID, it stops the old one first.
// Then it starts a new routing goroutine if the external ID is not empty.
// This method should NOT be called concurrently - it's designed for sequential reconnection scenarios.
func (r *route) setExternal(externalID string, externalCh chan connection.Notification, l logger.Logger) {
	// both externalID and externalCh must be provided together
	panicIfEmpty(externalID, "BUG: externalID is mandatory")
	panicIfNilNotificationCh(externalCh, "BUG: externalCh is mandatory")

	// Stop old routing if it exists and external ID is changing
	if r.stopCh != nil && r.externalID != externalID {
		if l != nil {
			l.Debug("Stopping old routing",
				"internal_id", r.internalID,
				"old_external_id", r.externalID,
				"new_external_id", externalID)
		}
		r.stopRoutingGoroutine()
	}

	// Update route
	r.externalID = externalID
	r.externalCh = externalCh

	// Start the routing goroutine with the logger
	r.startRoutingGoroutine(l)
}

// stopRoutingGoroutine stops the routing goroutine if it's running and waits for it to finish.
// It returns true if a goroutine was stopped, false if there was no goroutine running.
func (r *route) stopRoutingGoroutine() {
	// If you see this panic, it means stopRoutingGoroutine was called
	// multiple times concurrently, which is not allowed and a programming error.
	close(r.stopCh)
	<-r.stoppedCh // Wait for goroutine to finish

	// Clear the channels after stopping
	r.stopCh = nil
	r.stoppedCh = nil
	r.externalCh = nil
}

// startRoutingGoroutine starts a goroutine that routes notifications from external to internal channel.
// It takes copies of the mutable fields to avoid race conditions during reconnection.
// The goroutine will signal completion by closing stoppedCh.
func (r *route) startRoutingGoroutine(l logger.Logger) {
	// Create fresh channels for the new goroutine
	r.stopCh = make(chan struct{})
	r.stoppedCh = make(chan struct{})

	// Create copies of all the fields we need to avoid races
	internalID := r.internalID
	externalID := r.externalID
	internalCh := r.internalCh
	externalCh := r.externalCh
	stopCh := r.stopCh
	stoppedCh := r.stoppedCh

	go func() {
		defer close(stoppedCh) // Signal completion when goroutine exits

		if l != nil {
			l.Debug("Starting notification routing",
				"internal_id", internalID,
				"external_id", externalID)
		}

		for {
			select {
			case notification, ok := <-externalCh:
				if !ok {
					if l != nil {
						l.Debug("External channel closed",
							"internal_id", internalID,
							"external_id", externalID)
					}
					return
				}

				select {
				case internalCh <- notification:
					// Successfully routed notification
				default:
					// Internal channel might be full
					if l != nil {
						l.Warn("Failed to route notification, channel might be full",
							"internal_id", internalID)
					}
				}

			case <-stopCh:
				if l != nil {
					l.Debug("Notification routing stopped",
						"internal_id", internalID,
						"external_id", externalID)
				}
				return
			}
		}
	}()
}

package rews

import (
	"sync"
	"sync/atomic"

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

	// closeInternal closes internalCh exactly once: either when the live query
	// ends with a KILLED notification, or when the route is stopped.
	closeInternal sync.Once

	// ended is set once the live query has ended with a KILLED notification
	// and internalCh has been closed. An ended route routes nothing more.
	ended atomic.Bool

	// onEnd, if set, is called once the live query has ended with a KILLED
	// notification, after the routing goroutine has finished.
	onEnd func(*route)

	// Mutable fields - updated during reconnection when external UUID changes
	// Note: These fields should only be modified by a single goroutine (no concurrent access)
	// The routing goroutine receives copies of these values to avoid races
	externalID string                       // Changes on reconnection (new UUID from server)
	externalCh chan connection.Notification // Changes on reconnection (new channel from server)
	stopCh     chan struct{}                // Signal to stop routing goroutine
	stoppedCh  chan struct{}                // Closed by goroutine when it finishes

	// backlog holds what a stopped routing goroutine had not yet routed, for
	// the next one to route first. It is written by the goroutine before it
	// closes stoppedCh, and otherwise only touched with no goroutine running.
	backlog backlog
}

// backlog is what is still to be routed from earlier sources when the route
// switches to a new one: notifications already taken from them, then whatever
// remains on the sources themselves, oldest first.
type backlog struct {
	pending []connection.Notification
	sources []chan connection.Notification
}

// newRoute creates a new route with the given internal ID.
// The internal channel is buffered so that a consumer briefly busy elsewhere
// does not hold up routing.
func newRoute(internalID string) *route {
	panicIfEmpty(internalID, "BUG: internalID is mandatory")

	return &route{
		internalID: internalID,
		internalCh: make(chan connection.Notification, 100),
	}
}

// stop completely stops the route by stopping any running routing goroutine and closing the internal channel.
// This should be called when the route is being removed.
func (r *route) stop() {
	// First stop any running goroutine to ensure nothing is trying to send to internalCh
	if r.stopCh != nil {
		r.stopRoutingGoroutine()
	}

	r.backlog = backlog{}

	// Now it's safe to close the internal channel since no goroutine is running.
	// It is already closed if the live query ended with KILLED.
	r.closeInternal.Do(func() { close(r.internalCh) })
}

// isEnded reports whether the live query ended with a KILLED notification,
// which closed the internal channel.
func (r *route) isEnded() bool {
	return r.ended.Load()
}

// setExternal updates the external ID and channel, managing the routing goroutine lifecycle.
// It stops any routing goroutine already running, then starts a new one reading from externalCh.
// This method should NOT be called concurrently - it's designed for sequential reconnection scenarios.
//
// It does nothing for a route whose live query has ended.
func (r *route) setExternal(externalID string, externalCh chan connection.Notification, l logger.Logger) {
	// both externalID and externalCh must be provided together
	panicIfEmpty(externalID, "BUG: externalID is mandatory")
	panicIfNilNotificationCh(externalCh, "BUG: externalCh is mandatory")

	// Stop old routing if it exists. Two goroutines reading the same or
	// different external channels would reorder notifications.
	previous := r.externalCh
	if r.stopCh != nil {
		if l != nil {
			l.Debug("Stopping old routing",
				"internal_id", r.internalID,
				"old_external_id", r.externalID,
				"new_external_id", externalID)
		}
		r.stopRoutingGoroutine()
	}

	// Checked once no goroutine is running, as the one just stopped may have
	// routed a KILLED notification before it stopped.
	if r.isEnded() {
		return
	}

	// What remains on the previous source was received before the switch, and
	// is routed before anything from the new one. When its connection was
	// closed or lost, it is closed with its last notifications still buffered.
	if previous != nil && previous != externalCh {
		r.backlog.sources = append(r.backlog.sources, previous)
	}

	// Update route
	r.externalID = externalID
	r.externalCh = externalCh

	// Start the routing goroutine with the logger
	r.startRoutingGoroutine(l)
}

// stopRoutingGoroutine stops the routing goroutine if it's running and waits for it to finish.
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
//
// The external channel can end in two ways, and the goroutine tells them apart
// by the last notification it routed:
//
//   - With a KILLED notification: the live query has ended for good. The
//     goroutine closes the internal channel, so that a consumer ranging over it
//     stops, and marks the route ended.
//   - Without one: the external channel was closed because its connection was
//     closed or lost. The live query will be restored on the next connection,
//     which calls setExternal with the new external channel, so the internal
//     channel stays open.
func (r *route) startRoutingGoroutine(l logger.Logger) {
	// Create fresh channels for the new goroutine
	r.stopCh = make(chan struct{})
	r.stoppedCh = make(chan struct{})

	// Create copies of all the fields we need to avoid races
	externalID := r.externalID
	externalCh := r.externalCh
	stopCh := r.stopCh
	stoppedCh := r.stoppedCh
	earlier := r.backlog
	r.backlog = backlog{}

	go func() {
		ended, left := r.route(earlier, externalID, externalCh, stopCh, l)
		// Read by setExternal once stoppedCh is closed.
		r.backlog = left
		// Signal completion before calling onEnd, which may take locks held
		// by someone waiting for this goroutine to stop.
		close(stoppedCh)
		if ended && r.onEnd != nil {
			r.onEnd(r)
		}
	}()
}

// route routes the backlog of earlier sources, then notifications from
// externalCh, to the internal channel until externalCh is closed or stopCh is
// closed. It returns true if the live query ended with a KILLED notification,
// having closed the internal channel. Otherwise it returns what it was stopped
// before routing, for the next routing goroutine.
func (r *route) route(
	earlier backlog,
	externalID string,
	externalCh chan connection.Notification,
	stopCh chan struct{},
	l logger.Logger,
) (bool, backlog) {
	if l != nil {
		l.Debug("Starting notification routing",
			"internal_id", r.internalID,
			"external_id", externalID)
	}

	g := routing{r: r, externalID: externalID, stopCh: stopCh, l: l}
	if ended, left, ok := g.routeBacklog(earlier); !ok {
		return ended, left
	}
	return g.routeSource(externalCh)
}

// routing is one routing goroutine's view of its route.
type routing struct {
	r          *route
	externalID string
	stopCh     chan struct{}
	l          logger.Logger
}

// forward routes n, waiting for the consumer rather than dropping it. It
// returns false if stopped first, leaving n unrouted.
func (g *routing) forward(n connection.Notification) bool {
	select {
	case g.r.internalCh <- n:
		return true
	case <-g.stopCh:
		return false
	}
}

// end ends the route after its KILLED notification has been routed.
func (g *routing) end() (bool, backlog) {
	if g.l != nil {
		g.l.Debug("Live query killed, closing its channel",
			"internal_id", g.r.internalID,
			"external_id", g.externalID)
	}
	g.r.ended.Store(true)
	g.r.closeInternal.Do(func() { close(g.r.internalCh) })
	return true, backlog{}
}

// routeBacklog routes what earlier sources left. It returns ok if all of it
// was routed; otherwise routing is over, with the results of route.
func (g *routing) routeBacklog(earlier backlog) (ended bool, left backlog, ok bool) {
	pending := earlier.pending
	for len(pending) > 0 {
		n := pending[0]
		if !g.forward(n) {
			return false, backlog{pending: pending, sources: earlier.sources}, false
		}
		pending = pending[1:]
		if n.Action == connection.KilledAction {
			ended, left = g.end()
			return ended, left, false
		}
	}

	// Earlier sources are drained without waiting: a closed one up to its
	// end, one still open up to what it already holds.
	sources := earlier.sources
	for len(sources) > 0 {
		select {
		case n, open := <-sources[0]:
			if !open {
				sources = sources[1:]
				continue
			}
			if !g.forward(n) {
				return false, backlog{pending: []connection.Notification{n}, sources: sources}, false
			}
			if n.Action == connection.KilledAction {
				ended, left = g.end()
				return ended, left, false
			}
		case <-g.stopCh:
			return false, backlog{sources: sources}, false
		default:
			sources = sources[1:]
		}
	}
	return false, backlog{}, true
}

// routeSource routes notifications from externalCh until it is closed, the
// live query is killed, or routing is stopped.
func (g *routing) routeSource(externalCh chan connection.Notification) (bool, backlog) {
	for {
		select {
		case n, ok := <-externalCh:
			if !ok {
				if g.l != nil {
					g.l.Debug("External channel closed, waiting for the live query to be restored",
						"internal_id", g.r.internalID,
						"external_id", g.externalID)
				}
				return false, backlog{}
			}
			if !g.forward(n) {
				return false, backlog{pending: []connection.Notification{n}}
			}
			if n.Action == connection.KilledAction {
				return g.end()
			}

		case <-g.stopCh:
			if g.l != nil {
				g.l.Debug("Notification routing stopped",
					"internal_id", g.r.internalID,
					"external_id", g.externalID)
			}
			return false, backlog{}
		}
	}
}

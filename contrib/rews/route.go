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

// segment is part of what a route has received from earlier sources but not
// yet routed: items, then, if ch is set, everything a source whose connection
// was lost goes on delivering until it closes.
type segment struct {
	items []connection.Notification
	ch    chan connection.Notification
}

// discard reads ch until it is closed, so that whoever delivers on it is not
// left blocked once nothing routes it any more. A source whose connection was
// lost closes once it has delivered what it holds.
func discard(ch chan connection.Notification) {
	if ch == nil {
		return
	}
	go func() {
		for range ch {
		}
	}()
}

// discardSegments discards the sources of segments which will not be routed.
func discardSegments(segments []segment) {
	for _, s := range segments {
		discard(s.ch)
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

	// ended is set once the live query has ended with a KILLED notification:
	// when the KILLED notification is known, whether still to be routed or
	// already routed. internalCh is closed once it has been routed. An ended
	// route takes no new source.
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

	// orphaned, if set, is called if the live query ends with a KILLED
	// notification from an earlier source while externalCh is attached:
	// nothing will route externalCh's live query, which must be ended.
	orphaned func()

	// backlog holds what was received from earlier sources but not yet
	// routed, oldest first, for the next routing goroutine to route before
	// reading its source. It is written by a routing goroutine before it
	// closes stoppedCh, and otherwise only touched with no goroutine running.
	backlog []segment
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

	discardSegments(r.backlog)
	r.backlog = nil

	// Now it's safe to close the internal channel since no goroutine is running.
	// It is already closed if the live query ended with KILLED.
	r.closeInternal.Do(func() { close(r.internalCh) })
}

// isEnded reports whether the live query ended with a KILLED notification.
func (r *route) isEnded() bool {
	return r.ended.Load()
}

// setExternal updates the external ID and channel, managing the routing goroutine lifecycle.
// It stops any routing goroutine already running, then starts a new one reading from externalCh.
// This method should NOT be called concurrently - it's designed for sequential reconnection scenarios.
//
// It returns true, and does not use externalCh, if the live query has ended.
// NotificationRouter uses detach and attach directly, to subscribe in between.
func (r *route) setExternal(externalID string, externalCh chan connection.Notification) bool {
	// both externalID and externalCh must be provided together
	panicIfEmpty(externalID, "BUG: externalID is mandatory")
	panicIfNilNotificationCh(externalCh, "BUG: externalCh is mandatory")

	if r.detach(nil, false) {
		return true
	}
	r.attach(externalID, externalCh, nil, nil)
	return false
}

// detach stops routing from the current source, keeping what was received
// from it but not yet routed in the backlog. It returns true if the live query
// has ended, in which case routing carries on until the KILLED notification
// has been routed, and the route takes no new source.
//
// What the previous source already holds is taken now, so that a KILLED
// notification among it is known before a new source is attached.
//
// If previousLost, the previous source's connection has been lost, and the
// source goes on delivering what that connection received before closing.
// What it delivers after now is routed, before anything from the next source,
// by the routing goroutine, which reads it until it closes however long that
// takes. A KILLED notification among it ends the route then, and the next
// source's orphaned is called. Otherwise only what the previous source
// already holds is routed.
func (r *route) detach(l logger.Logger, previousLost bool) bool {
	previous := r.externalCh
	if r.stopCh != nil {
		if l != nil {
			l.Debug("Stopping old routing",
				"internal_id", r.internalID,
				"old_external_id", r.externalID)
		}
		r.stopRoutingGoroutine()
	}
	r.orphaned = nil

	// The goroutine just stopped may have routed a KILLED notification, or
	// been stopped before routing one already taken into the backlog: then
	// it carries on routing that.
	if r.isEnded() {
		if previousLost {
			// Its connection was lost: nothing else will read it.
			discard(previous)
		}
		if len(r.backlog) > 0 {
			r.attach("", nil, nil, l)
		}
		return true
	}

	// What remains on the previous source was received before the switch.
	if previous != nil {
		r.backlog = append(r.backlog, takeRemaining(previous, previousLost))
	}

	for i := range r.backlog {
		for j, n := range r.backlog[i].items {
			if n.Action != connection.KilledAction {
				continue
			}
			// Nothing after KILLED is routed. Route what comes before it,
			// then end.
			discard(r.backlog[i].ch)
			discardSegments(r.backlog[i+1:])
			r.backlog[i] = segment{items: r.backlog[i].items[:j+1]}
			r.backlog = r.backlog[:i+1]
			r.ended.Store(true)
			r.attach("", nil, nil, l)
			return true
		}
	}
	return false
}

// takeRemaining takes what a previous source holds now. With lost, the
// source, unless already closed, is kept to be read until it closes.
func takeRemaining(previous chan connection.Notification, lost bool) segment {
	var s segment
	for {
		select {
		case n, ok := <-previous:
			if !ok {
				return s
			}
			s.items = append(s.items, n)
		default:
			if lost {
				s.ch = previous
			}
			return s
		}
	}
}

// attach starts routing the backlog, then externalCh. With a nil externalCh,
// only the backlog is routed, then routing waits to be stopped. orphaned, if
// set, is called if the live query ends with a KILLED notification from the
// backlog while externalCh is attached.
func (r *route) attach(externalID string, externalCh chan connection.Notification, orphaned func(), l logger.Logger) {
	r.externalID = externalID
	r.externalCh = externalCh
	r.orphaned = orphaned
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
	g := routing{r: r, externalID: r.externalID, stopCh: r.stopCh, l: l}
	externalCh := r.externalCh
	stoppedCh := r.stoppedCh
	orphaned := r.orphaned
	earlier := r.backlog
	r.backlog = nil

	go func() {
		ended, early, left := g.route(earlier, externalCh)
		// Read once stoppedCh is closed.
		r.backlog = left
		// Signal completion before calling onEnd, which may take locks held
		// by someone waiting for this goroutine to stop.
		close(stoppedCh)
		if ended && r.onEnd != nil {
			r.onEnd(r)
		}
		if early && orphaned != nil {
			orphaned()
		}
	}()
}

// routing is one routing goroutine's view of its route.
type routing struct {
	r          *route
	externalID string
	stopCh     chan struct{}
	l          logger.Logger
}

// route routes the backlog, then notifications from externalCh, to the
// internal channel until externalCh is closed or stopCh is closed. It returns
// true if the live query ended with a KILLED notification, having closed the
// internal channel, and whether that came from the backlog while externalCh
// was attached. Otherwise it returns what it was stopped before routing.
func (g *routing) route(backlog []segment, externalCh chan connection.Notification) (ended, early bool, left []segment) {
	if g.l != nil {
		g.l.Debug("Starting notification routing",
			"internal_id", g.r.internalID,
			"external_id", g.externalID)
	}

	for len(backlog) > 0 {
		var stopped bool
		if ended, stopped = g.routeSegment(&backlog[0]); stopped {
			return false, false, backlog
		}
		if ended {
			discardSegments(backlog[1:])
			return true, externalCh != nil, nil
		}
		backlog = backlog[1:]
	}

	ended, left = g.routeSource(externalCh)
	return ended, false, left
}

// routeSegment routes a segment of the backlog. It returns true if the live
// query ended with a KILLED notification among it, or stopped, true if it was
// stopped first, leaving s with what it did not route.
func (g *routing) routeSegment(s *segment) (ended, stopped bool) {
	for len(s.items) > 0 {
		n := s.items[0]
		if !g.forward(n) {
			return false, true
		}
		s.items = s.items[1:]
		if n.Action == connection.KilledAction {
			discard(s.ch)
			return g.end(), false
		}
	}
	// A source whose connection was lost: route everything it delivers,
	// until it closes, before anything later.
	for s.ch != nil {
		select {
		case n, ok := <-s.ch:
			if !ok {
				s.ch = nil
				return false, false
			}
			if !g.forward(n) {
				s.items = []connection.Notification{n}
				return false, true
			}
			if n.Action == connection.KilledAction {
				discard(s.ch)
				return g.end(), false
			}
		case <-g.stopCh:
			return false, true
		}
	}
	return false, false
}

// routeSource routes notifications from externalCh until it is closed or
// stopCh is closed. It returns true if the live query ended with a KILLED
// notification. Otherwise it returns what it was stopped before routing.
func (g *routing) routeSource(externalCh chan connection.Notification) (bool, []segment) {
	for {
		select {
		case n, ok := <-externalCh:
			if !ok {
				if g.l != nil {
					g.l.Debug("External channel closed, waiting for the live query to be restored",
						"internal_id", g.r.internalID,
						"external_id", g.externalID)
				}
				return false, nil
			}
			if !g.forward(n) {
				return false, []segment{{items: []connection.Notification{n}}}
			}
			if n.Action == connection.KilledAction {
				return g.end(), nil
			}

		case <-g.stopCh:
			if g.l != nil {
				g.l.Debug("Notification routing stopped",
					"internal_id", g.r.internalID,
					"external_id", g.externalID)
			}
			return false, nil
		}
	}
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
func (g *routing) end() bool {
	if g.l != nil {
		g.l.Debug("Live query killed, closing its channel",
			"internal_id", g.r.internalID,
			"external_id", g.externalID)
	}
	g.r.ended.Store(true)
	g.r.closeInternal.Do(func() { close(g.r.internalCh) })
	return true
}

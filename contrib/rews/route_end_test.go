package rews

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
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

func quietLogger() logger.Logger {
	return logger.New(slog.NewTextHandler(io.Discard, nil))
}

func notification(action connection.Action, n int) connection.Notification {
	return connection.Notification{Action: action, Result: n}
}

func receiveNotification(t *testing.T, ch chan connection.Notification) (connection.Notification, bool) {
	t.Helper()
	select {
	case n, ok := <-ch:
		return n, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a notification")
		return connection.Notification{}, false
	}
}

func requireOpenAndEmpty(t *testing.T, ch chan connection.Notification) {
	t.Helper()
	select {
	case n, ok := <-ch:
		if !ok {
			t.Fatal("the channel was closed")
		}
		t.Fatalf("unexpected notification %+v", n)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestRoute_killedClosesInternalChannel checks that a source ending with KILLED
// closes the channel the caller reads from. It used to stay open, so a caller
// ranging over it blocked for ever.
func TestRoute_killedClosesInternalChannel(t *testing.T) {
	r := newRoute("internal")
	var ended atomic.Int32
	r.onEnd = func(*route) { ended.Add(1) }

	source := make(chan connection.Notification, 2)
	r.setExternal("external", source)
	source <- notification(connection.CreateAction, 1)
	source <- notification(connection.KilledAction, 2)
	// The source closes after KILLED, as connection.Toolkit does.
	close(source)

	var got []connection.Action
	for n := range r.internalCh {
		got = append(got, n.Action)
	}
	assert.Equal(t, []connection.Action{connection.CreateAction, connection.KilledAction}, got)
	assert.True(t, r.isEnded())
	assert.Eventually(t, func() bool { return ended.Load() == 1 }, time.Second, time.Millisecond)

	// An ended route is not restarted with a new source, and stopping it does
	// not close its channel twice.
	r.setExternal("restored", make(chan connection.Notification))
	assert.Nil(t, r.stopCh, "no routing goroutine runs for an ended route")
	assert.NotPanics(t, r.stop)
}

// TestRoute_sourceClosedWithoutKilledKeepsRouting checks that a source closed
// without KILLED, as when its connection is closed or lost, leaves the
// caller's channel open for the source of the next connection.
func TestRoute_sourceClosedWithoutKilledKeepsRouting(t *testing.T) {
	r := newRoute("internal")
	r.onEnd = func(*route) { t.Error("the live query did not end") }

	lost := make(chan connection.Notification, 1)
	r.setExternal("external-1", lost)
	lost <- notification(connection.CreateAction, 1)
	n, ok := receiveNotification(t, r.internalCh)
	require.True(t, ok)
	assert.Equal(t, 1, n.Result)

	close(lost)
	requireOpenAndEmpty(t, r.internalCh)
	assert.False(t, r.isEnded())

	restored := make(chan connection.Notification, 1)
	r.setExternal("external-2", restored)
	restored <- notification(connection.UpdateAction, 2)
	n, ok = receiveNotification(t, r.internalCh)
	require.True(t, ok)
	assert.Equal(t, 2, n.Result)

	r.stop()
	_, ok = <-r.internalCh
	assert.False(t, ok, "stop closes the channel")
}

// TestRoute_slowConsumerLosesNothing sends more notifications than the
// caller's channel holds before reading any. They used to be dropped.
func TestRoute_slowConsumerLosesNothing(t *testing.T) {
	r := newRoute("internal")
	const count = 500
	source := make(chan connection.Notification, count)
	r.setExternal("external", source)
	for i := 0; i < count; i++ {
		source <- notification(connection.CreateAction, i)
	}
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < count; i++ {
		n, ok := receiveNotification(t, r.internalCh)
		require.True(t, ok)
		require.Equal(t, i, n.Result, "notifications arrive in order")
	}
	r.stop()
}

// TestRoute_stopWhileConsumerNotReading checks that a route whose caller has
// stopped reading can still be stopped.
func TestRoute_stopWhileConsumerNotReading(t *testing.T) {
	r := newRoute("internal")
	source := make(chan connection.Notification, 200)
	r.setExternal("external", source)
	for i := 0; i < 200; i++ {
		source <- notification(connection.CreateAction, i)
	}

	done := make(chan struct{})
	go func() {
		r.stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stop blocked")
	}
}

// TestNotificationRouter_reconnectThenKill follows a live query through a lost
// connection, its restoration on a new one, and a KILL: the caller's channel
// stays open across the reconnection and is closed after KILLED.
func TestNotificationRouter_reconnectThenKill(t *testing.T) {
	router := NewNotificationRouter(quietLogger())
	var endedID atomic.Value
	router.onEnd = func(id string) { endedID.Store(id) }
	provider := newMockNotificationProvider()

	ch, err := router.SetupRouting("internal", "external-1", provider)
	require.NoError(t, err)

	// The connection is lost: its channel closes without KILLED.
	provider.CloseChannel("external-1")
	requireOpenAndEmpty(t, ch)

	// The live query is restored on the new connection.
	ch2, err := router.SetupRouting("internal", "external-2", provider)
	require.NoError(t, err)
	require.Equal(t, ch, ch2)

	require.NoError(t, provider.SendNotification("external-2", notification(connection.CreateAction, 1)))
	require.NoError(t, provider.SendNotification("external-2", notification(connection.KilledAction, 2)))

	var got []connection.Action
	for n := range ch {
		got = append(got, n.Action)
	}
	assert.Equal(t, []connection.Action{connection.CreateAction, connection.KilledAction}, got)
	assert.Eventually(t, func() bool { return endedID.Load() == "internal" }, time.Second, time.Millisecond)

	// Setting up routing for the ended live query, as a reconnection racing
	// the KILL would, returns its closed channel without subscribing again.
	ch3, err := router.SetupRouting("internal", "external-3", provider)
	require.NoError(t, err)
	require.Equal(t, ch, ch3)
	provider.mu.Lock()
	_, subscribed := provider.channels["external-3"]
	provider.mu.Unlock()
	assert.False(t, subscribed)

	assert.NotPanics(t, func() { router.RemoveRoute("internal") })
}

type recordingCloser struct{ closed []string }

func (c *recordingCloser) CloseLiveNotifications(id string) error {
	c.closed = append(c.closed, id)
	return nil
}

// TestReliableLQ_killedIsNotRestored checks that a live query ended by KILLED
// is not started again after a reconnection, and that closing its
// notifications afterwards succeeds.
func TestReliableLQ_killedIsNotRestored(t *testing.T) {
	log := quietLogger()
	rlq := newReliableLQ(log, surrealcbor.New())
	rlq.recordLiveQuery("lq", methodLive, []any{"users", false}, log)
	provider := newMockNotificationProvider()

	ch, err := rlq.liveNotifications("lq", provider)
	require.NoError(t, err)
	require.NoError(t, provider.SendNotification("lq", notification(connection.KilledAction, 1)))
	n, ok := receiveNotification(t, ch)
	require.True(t, ok)
	assert.Equal(t, connection.KilledAction, n.Action)
	_, ok = receiveNotification(t, ch)
	assert.False(t, ok, "the channel is closed after KILLED")

	require.Eventually(t, func() bool {
		rlq.liveQueriesMu.RLock()
		defer rlq.liveQueriesMu.RUnlock()
		return rlq.liveQueries["lq"].ended
	}, time.Second, time.Millisecond)

	sender := &mockRPCSender{}
	require.NoError(t, rlq.restoreLiveQueries(context.Background(), sender, provider, log))
	assert.Equal(t, 0, sender.sendCalled, "a killed live query is not restored")

	closer := &recordingCloser{}
	require.NoError(t, rlq.closeLiveNotifications(closer, "lq"))
	assert.Empty(t, closer.closed, "the ended live query's notifications are already closed")
	rlq.liveQueriesMu.RLock()
	assert.Empty(t, rlq.liveQueries)
	rlq.liveQueriesMu.RUnlock()
}

// TestRoute_reconnectWhileConsumerBehindLosesNothing switches the route to a
// new source while the caller is more than a channel's worth behind. The
// notifications still on the old, closed source, and the one being routed,
// used to be dropped.
func TestRoute_reconnectWhileConsumerBehindLosesNothing(t *testing.T) {
	r := newRoute("internal")
	const before, after = 150, 5

	lost := make(chan connection.Notification, before)
	r.setExternal("external-1", lost)
	for i := 0; i < before; i++ {
		lost <- notification(connection.CreateAction, i)
	}
	// The caller's channel fills up and the routing goroutine waits on it.
	require.Eventually(t, func() bool { return len(r.internalCh) == cap(r.internalCh) }, time.Second, time.Millisecond)
	// The connection is lost: its channel is closed with notifications left.
	close(lost)

	restored := make(chan connection.Notification, after)
	r.setExternal("external-2", restored)
	for i := before; i < before+after; i++ {
		restored <- notification(connection.CreateAction, i)
	}

	for i := 0; i < before+after; i++ {
		n, ok := receiveNotification(t, r.internalCh)
		require.True(t, ok)
		require.Equal(t, i, n.Result, "notifications arrive in order, none lost")
	}
	r.stop()
}

// TestRoute_repeatedReconnectsWhileConsumerBehind switches source twice before
// the caller reads anything, so the second switch happens while the first
// backlog is still being forwarded.
func TestRoute_repeatedReconnectsWhileConsumerBehind(t *testing.T) {
	r := newRoute("internal")
	const per = 120
	next := 0
	for c := 0; c < 3; c++ {
		src := make(chan connection.Notification, per)
		r.setExternal(fmt.Sprintf("external-%d", c), src)
		for i := 0; i < per; i++ {
			src <- notification(connection.CreateAction, next)
			next++
		}
		if c < 2 {
			close(src)
		}
	}

	for i := 0; i < next; i++ {
		n, ok := receiveNotification(t, r.internalCh)
		require.True(t, ok)
		require.Equal(t, i, n.Result)
	}
	assert.NotPanics(t, r.stop)
}

// restoreSender answers a restored live query with a new id, running during
// to simulate what happens while the request is in flight, and records every
// request.
type restoreSender struct {
	newID   string
	during  func()
	mu      sync.Mutex
	methods []string
	params  [][]any
}

func (s *restoreSender) Send(_ context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	s.mu.Lock()
	s.methods = append(s.methods, method)
	s.params = append(s.params, params)
	s.mu.Unlock()
	var result cbor.RawMessage
	if method == methodLive {
		if s.during != nil {
			s.during()
		}
		id, err := uuid.FromString(s.newID)
		if err != nil {
			return nil, err
		}
		result, err = surrealcbor.New().Marshal(models.UUID{UUID: id})
		if err != nil {
			return nil, err
		}
	}
	return &connection.RPCResponse[cbor.RawMessage]{Result: &result}, nil
}

// TestReliableLQ_endedDuringRestoreIsKilled ends a live query while it is
// being restored. The restored server query used to be left running, with no
// route and no way to kill it.
func TestReliableLQ_endedDuringRestoreIsKilled(t *testing.T) {
	cases := map[string]func(rlq *reliableLQ){
		"killed": func(rlq *reliableLQ) { rlq.liveQueryEnded("lq") },
		"notifications closed": func(rlq *reliableLQ) {
			require.NoError(t, rlq.closeLiveNotifications(&recordingCloser{}, "lq"))
		},
	}
	for name, during := range cases {
		t.Run(name, func(t *testing.T) {
			log := quietLogger()
			rlq := newReliableLQ(log, surrealcbor.New())
			rlq.recordLiveQuery("lq", methodLive, []any{"users", false}, log)
			provider := newMockNotificationProvider()

			newID := uuid.Must(uuid.NewV4()).String()
			sender := &restoreSender{newID: newID, during: func() { during(rlq) }}
			require.NoError(t, rlq.restoreLiveQueries(context.Background(), sender, provider, log))

			require.Equal(t, []string{methodLive, "kill"}, sender.methods)
			assert.Equal(t, []any{newID}, sender.params[1])
			provider.mu.Lock()
			_, subscribed := provider.channels[newID]
			provider.mu.Unlock()
			assert.False(t, subscribed, "nothing subscribes to the killed query's notifications")
			rlq.router.routesMu.RLock()
			assert.Empty(t, rlq.router.routes)
			rlq.router.routesMu.RUnlock()
		})
	}
}

// TestReliableLQ_killedInBacklogDuringRestoreIsKilled restores a live query
// whose old, closed source still holds its KILLED notification, behind a
// consumer which has fallen behind. The restore used to be told the query was
// active, leaving the new server query running with nothing routing it.
func TestReliableLQ_killedInBacklogDuringRestoreIsKilled(t *testing.T) {
	log := quietLogger()
	rlq := newReliableLQ(log, surrealcbor.New())
	rlq.recordLiveQuery("lq", methodLive, []any{"users", false}, log)
	provider := newMockNotificationProvider()
	const before = 120
	lost := make(chan connection.Notification, before+1)
	provider.channels["lq"] = lost

	ch, err := rlq.liveNotifications("lq", provider)
	require.NoError(t, err)
	for i := 0; i < before; i++ {
		lost <- notification(connection.CreateAction, i)
	}
	lost <- notification(connection.KilledAction, before)
	// The consumer has fallen behind, and the connection is lost.
	require.Eventually(t, func() bool { return len(ch) == cap(ch) }, time.Second, time.Millisecond)
	close(lost)

	newID := uuid.Must(uuid.NewV4()).String()
	sender := &restoreSender{newID: newID}
	restoredCh := make(chan error, 1)
	go func() { restoredCh <- rlq.restoreLiveQueries(context.Background(), sender, provider, log) }()
	select {
	case err := <-restoredCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("restoring blocked")
	}

	require.Equal(t, []string{methodLive, "kill"}, sender.methods, "the new server query is killed")
	assert.Equal(t, []any{newID}, sender.params[1])
	provider.mu.Lock()
	_, subscribed := provider.channels[newID]
	provider.mu.Unlock()
	assert.False(t, subscribed, "nothing subscribes to the killed query's notifications")

	// Everything up to KILLED is still delivered, then the channel is closed.
	for i := 0; i < before; i++ {
		n, ok := receiveNotification(t, ch)
		require.True(t, ok)
		require.Equal(t, i, n.Result)
	}
	n, ok := receiveNotification(t, ch)
	require.True(t, ok)
	assert.Equal(t, connection.KilledAction, n.Action)
	_, ok = receiveNotification(t, ch)
	assert.False(t, ok)
	require.Eventually(t, func() bool {
		rlq.liveQueriesMu.RLock()
		defer rlq.liveQueriesMu.RUnlock()
		return rlq.liveQueries["lq"].ended
	}, time.Second, time.Millisecond)
}

// TestRoute_endedBacklogSurvivesAnotherSwap swaps the source again while a
// KILLED notification found in the backlog is still waiting to be routed.
func TestRoute_endedBacklogSurvivesAnotherSwap(t *testing.T) {
	r := newRoute("internal")
	const before = 120
	lost := make(chan connection.Notification, before+1)
	r.setExternal("external-1", lost)
	for i := 0; i < before; i++ {
		lost <- notification(connection.CreateAction, i)
	}
	lost <- notification(connection.KilledAction, before)
	require.Eventually(t, func() bool { return len(r.internalCh) == cap(r.internalCh) }, time.Second, time.Millisecond)
	close(lost)

	assert.True(t, r.setExternal("external-2", make(chan connection.Notification)), "the swap reports the end")
	assert.True(t, r.setExternal("external-3", make(chan connection.Notification)), "and so does the next one")

	for i := 0; i < before; i++ {
		n, ok := receiveNotification(t, r.internalCh)
		require.True(t, ok)
		require.Equal(t, i, n.Result)
	}
	n, ok := receiveNotification(t, r.internalCh)
	require.True(t, ok)
	assert.Equal(t, connection.KilledAction, n.Action)
	_, ok = receiveNotification(t, r.internalCh)
	assert.False(t, ok)
	assert.NotPanics(t, r.stop)
}

func newToolkit() *connection.Toolkit {
	return &connection.Toolkit{
		ResponseChannels:     make(map[string]chan connection.RPCResponse[cbor.RawMessage]),
		NotificationChannels: make(map[string]chan connection.Notification),
	}
}

// TestReliableLQ_restoreAfterLossLosesNothing restores a live query after its
// connection was lost while the consumer was far behind. What the old
// connection had received is delivered before anything from the new one; it
// used to be dropped beyond what the channels between them held.
func TestReliableLQ_restoreAfterLossLosesNothing(t *testing.T) {
	log := quietLogger()
	rlq := newReliableLQ(log, surrealcbor.New())
	rlq.recordLiveQuery("lq", methodLive, []any{"users", false}, log)

	old := newToolkit()
	ch, err := rlq.liveNotifications("lq", old)
	require.NoError(t, err)
	const before, after = 300, 3
	for i := 0; i < before; i++ {
		require.True(t, old.SendNotification("lq", notification(connection.CreateAction, i)))
	}
	require.Eventually(t, func() bool { return len(ch) == cap(ch) }, time.Second, time.Millisecond)
	loseConnection(old)

	restored := newToolkit()
	newID := uuid.Must(uuid.NewV4()).String()
	sender := &restoreSender{newID: newID}
	require.NoError(t, rlq.restoreLiveQueries(context.Background(), sender, restored, log))
	require.Equal(t, []string{methodLive}, sender.methods)
	for i := before; i < before+after; i++ {
		require.True(t, restored.SendNotification(newID, notification(connection.CreateAction, i)))
	}

	for i := 0; i < before+after; i++ {
		n, ok := receiveNotification(t, ch)
		require.True(t, ok)
		require.Equal(t, i, n.Result, "notifications arrive in order, none lost")
	}
	require.NoError(t, rlq.closeLiveNotifications(restored, "lq"))
}

// sent returns the methods and params sent so far.
func (s *restoreSender) sent() (methods []string, params [][]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.methods...), append([][]any(nil), s.params...)
}

// slowLostSource sets up a live query, routed from a source which the test
// controls, then restores it on a new connection while the old source is
// still open: its connection was lost, but it has not yet delivered all it
// received. Restoring used to wait for it for a few seconds, then route the
// new source without it, so whatever it delivered afterwards was lost and
// whoever delivered it was left blocked.
func slowLostSource(t *testing.T) (rlq *reliableLQ, lost, ch chan connection.Notification, restored *connection.Toolkit, sender *restoreSender) {
	t.Helper()
	log := quietLogger()
	rlq = newReliableLQ(log, surrealcbor.New())
	rlq.recordLiveQuery("lq", methodLive, []any{"users", false}, log)
	provider := newMockNotificationProvider()
	lost = make(chan connection.Notification)
	provider.channels["lq"] = lost

	ch, err := rlq.liveNotifications("lq", provider)
	require.NoError(t, err)

	restored = newToolkit()
	sender = &restoreSender{newID: uuid.Must(uuid.NewV4()).String()}
	restoredCh := make(chan error, 1)
	go func() { restoredCh <- rlq.restoreLiveQueries(context.Background(), sender, restored, log) }()
	select {
	case err := <-restoredCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("restoring waited for the old source to close")
	}
	methods, _ := sender.sent()
	require.Equal(t, []string{methodLive}, methods)
	return rlq, lost, ch, restored, sender
}

// TestReliableLQ_restoreWhileLostSourceStillDeliveringLosesNothing delivers
// more on the old source, after restoring, than every channel in between can
// hold, then closes it. All of it is routed, before anything from the new
// connection.
func TestReliableLQ_restoreWhileLostSourceStillDeliveringLosesNothing(t *testing.T) {
	rlq, lost, ch, restored, sender := slowLostSource(t)
	const before, after = 300, 3

	delivered := make(chan struct{})
	go func() {
		defer close(delivered)
		for i := 0; i < before; i++ {
			lost <- notification(connection.CreateAction, i)
		}
		close(lost)
	}()
	for i := before; i < before+after; i++ {
		require.True(t, restored.SendNotification(sender.newID, notification(connection.CreateAction, i)))
	}

	for i := 0; i < before+after; i++ {
		n, ok := receiveNotification(t, ch)
		require.True(t, ok)
		require.Equal(t, i, n.Result, "notifications arrive in order, none lost")
	}
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("the old source was left blocked")
	}
	requireOpenAndEmpty(t, ch)
	require.NoError(t, rlq.closeLiveNotifications(restored, "lq"))
}

// TestReliableLQ_killedOnLostSourceAfterRestoreKillsRestored delivers the old
// connection's KILLED notification only after the live query was restored.
// The live query ends there, and the restored query, whose notifications
// nothing will route, is killed and unsubscribed from.
func TestReliableLQ_killedOnLostSourceAfterRestoreKillsRestored(t *testing.T) {
	rlq, lost, ch, restored, sender := slowLostSource(t)
	const before = 150

	require.True(t, restored.SendNotification(sender.newID, notification(connection.CreateAction, -1)))
	go func() {
		for i := 0; i < before; i++ {
			lost <- notification(connection.CreateAction, i)
		}
		lost <- notification(connection.KilledAction, before)
		close(lost)
	}()

	for i := 0; i < before; i++ {
		n, ok := receiveNotification(t, ch)
		require.True(t, ok)
		require.Equal(t, i, n.Result)
	}
	n, ok := receiveNotification(t, ch)
	require.True(t, ok)
	assert.Equal(t, connection.KilledAction, n.Action)
	_, ok = receiveNotification(t, ch)
	assert.False(t, ok, "nothing from the restored query is routed after KILLED")

	require.Eventually(t, func() bool {
		methods, _ := sender.sent()
		return len(methods) == 2
	}, 2*time.Second, time.Millisecond)
	methods, params := sender.sent()
	require.Equal(t, []string{methodLive, methodKill}, methods, "the restored query is killed")
	assert.Equal(t, []any{sender.newID}, params[1])
	_, subscribed := restored.GetNotificationChannel(sender.newID)
	assert.False(t, subscribed, "the restored query's notifications are closed")

	rlq.liveQueriesMu.RLock()
	ended := rlq.liveQueries["lq"].ended
	rlq.liveQueriesMu.RUnlock()
	assert.True(t, ended, "the live query is not restored again")
	require.NoError(t, rlq.closeLiveNotifications(restored, "lq"))
}

// TestRoute_stopWhileLostSourceStillDelivering stops a route still waiting
// for a lost source to close. Whatever delivers on it is not left blocked.
func TestRoute_stopWhileLostSourceStillDelivering(t *testing.T) {
	r := newRoute("internal")
	lost := make(chan connection.Notification)
	r.setExternal("external-1", lost)
	require.False(t, r.detach(nil, true))
	r.attach("external-2", make(chan connection.Notification), nil, nil)
	r.stop()

	delivered := make(chan struct{})
	go func() {
		defer close(delivered)
		for i := 0; i < 200; i++ {
			lost <- notification(connection.CreateAction, i)
		}
		close(lost)
	}()
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("the old source was left blocked")
	}
}

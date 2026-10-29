package connection

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func note(action Action, i int) Notification {
	return Notification{Action: action, Result: i}
}

func receive(t *testing.T, ch chan Notification) (Notification, bool) {
	t.Helper()
	select {
	case n, ok := <-ch:
		return n, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a notification")
		return Notification{}, false
	}
}

// TestLive_slowConsumerLosesNothing sends far more notifications than the
// channel holds before anything is read. They used to be dropped once 16 were buffered.
func TestLive_slowConsumerLosesNothing(t *testing.T) {
	tk := newTestToolkit()
	ch, err := tk.CreateNotificationChannel("lq")
	require.NoError(t, err)

	const count = 1000
	for i := 0; i < count; i++ {
		require.True(t, tk.SendNotification("lq", note(CreateAction, i)))
	}

	for i := 0; i < count; i++ {
		n, ok := receive(t, ch)
		require.True(t, ok)
		require.Equal(t, i, n.Result, "notifications arrive in order")
	}
	require.NoError(t, tk.CloseLiveNotifications("lq"))
}

// TestLive_killedClosesChannel checks that a KILLED notification is delivered
// and then ends the subscription.
func TestLive_killedClosesChannel(t *testing.T) {
	tk := newTestToolkit()
	ch, err := tk.CreateNotificationChannel("lq")
	require.NoError(t, err)

	require.True(t, tk.SendNotification("lq", note(CreateAction, 1)))
	require.True(t, tk.SendNotification("lq", note(KilledAction, 2)))
	// Nothing for a killed live query is delivered.
	assert.False(t, tk.SendNotification("lq", note(CreateAction, 3)))

	n, ok := receive(t, ch)
	require.True(t, ok)
	assert.Equal(t, CreateAction, n.Action)
	n, ok = receive(t, ch)
	require.True(t, ok)
	assert.Equal(t, KilledAction, n.Action)
	_, ok = receive(t, ch)
	assert.False(t, ok, "the channel is closed after KILLED")

	require.Eventually(t, func() bool {
		_, exists := tk.GetNotificationChannel("lq")
		return !exists
	}, time.Second, time.Millisecond)

	// Kill closes the notifications after the server has killed the query; that
	// must not fail because KILLED got there first.
	assert.NoError(t, tk.CloseLiveNotifications("lq"))
	// Stragglers are dropped rather than kept for a subscription that won't come.
	assert.False(t, tk.SendNotification("lq", note(UpdateAction, 4)))
}

// TestLive_notificationsBeforeSubscribing checks that notifications arriving
// between the start of a live query and the call to LiveNotifications are kept.
func TestLive_notificationsBeforeSubscribing(t *testing.T) {
	tk := newTestToolkit()

	require.True(t, tk.SendNotification("lq", note(CreateAction, 1)))
	require.True(t, tk.SendNotification("lq", note(UpdateAction, 2)))

	ch, err := tk.CreateNotificationChannel("lq")
	require.NoError(t, err)
	require.True(t, tk.SendNotification("lq", note(DeleteAction, 3)))

	for i := 1; i <= 3; i++ {
		n, ok := receive(t, ch)
		require.True(t, ok)
		assert.Equal(t, i, n.Result)
	}
	require.NoError(t, tk.CloseLiveNotifications("lq"))
}

func TestLive_killedBeforeSubscribing(t *testing.T) {
	tk := newTestToolkit()
	require.True(t, tk.SendNotification("lq", note(CreateAction, 1)))
	require.True(t, tk.SendNotification("lq", note(KilledAction, 2)))

	ch, err := tk.CreateNotificationChannel("lq")
	require.NoError(t, err)
	_, ok := receive(t, ch)
	require.True(t, ok)
	n, ok := receive(t, ch)
	require.True(t, ok)
	assert.Equal(t, KilledAction, n.Action)
	_, ok = receive(t, ch)
	assert.False(t, ok)
}

// TestLive_closeWithUnreadNotifications checks that closing a subscription
// whose consumer stopped reading returns at once.
func TestLive_closeWithUnreadNotifications(t *testing.T) {
	tk := newTestToolkit()
	ch, err := tk.CreateNotificationChannel("lq")
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		tk.SendNotification("lq", note(CreateAction, i))
	}

	done := make(chan error)
	go func() { done <- tk.CloseLiveNotifications("lq") }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("CloseLiveNotifications blocked")
	}

	// The buffered ones may still be read, then the channel is closed.
	for range ch {
	}
	assert.False(t, tk.SendNotification("lq", note(CreateAction, 0)), "a closed live query's stragglers are dropped")
	assert.Error(t, tk.CloseLiveNotifications("nope"))
}

func TestLive_pendingIsBounded(t *testing.T) {
	tk := newTestToolkit()
	for i := 0; i < maxPending; i++ {
		require.True(t, tk.SendNotification(fmt.Sprintf("lq%d", i%10), note(CreateAction, i)))
	}
	assert.False(t, tk.SendNotification("one-too-many", note(CreateAction, 0)))

	// Subscribing takes a live query's pending notifications off the total.
	_, err := tk.CreateNotificationChannel("lq0")
	require.NoError(t, err)
	assert.True(t, tk.SendNotification("room-again", note(CreateAction, 0)))
	require.NoError(t, tk.CloseLiveNotifications("lq0"))
}

func TestLive_duplicateSubscription(t *testing.T) {
	tk := newTestToolkit()
	_, err := tk.CreateNotificationChannel("lq")
	require.NoError(t, err)
	_, err = tk.CreateNotificationChannel("lq")
	require.Error(t, err)
	require.NoError(t, tk.CloseLiveNotifications("lq"))
}

// TestLive_closeAllEndsEverySubscription checks that closing or losing the
// connection ends every live query's notifications: each channel is closed and
// each subscription's goroutine has stopped, even for a consumer that stopped
// reading. They used to stay blocked for ever.
func TestLive_closeAllEndsEverySubscription(t *testing.T) {
	tk := newTestToolkit()

	unread, err := tk.CreateNotificationChannel("unread")
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		require.True(t, tk.SendNotification("unread", note(CreateAction, i)))
	}
	idle, err := tk.CreateNotificationChannel("idle")
	require.NoError(t, err)
	legacy := make(chan Notification, 1)
	tk.NotificationChannelsLock.Lock()
	tk.NotificationChannels["legacy"] = legacy
	tk.NotificationChannelsLock.Unlock()
	// Kept for a subscription that will now never come.
	require.True(t, tk.SendNotification("pending", note(CreateAction, 0)))

	tk.NotificationChannelsLock.RLock()
	subs := make([]*subscription, 0, len(tk.live.subs))
	for _, s := range tk.live.subs {
		subs = append(subs, s)
	}
	tk.NotificationChannelsLock.RUnlock()
	require.Len(t, subs, 2)

	done := make(chan struct{})
	go func() {
		tk.CloseAllLiveNotifications()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CloseAllLiveNotifications blocked")
	}

	// Each subscription's goroutine has stopped by the time it returns.
	for _, s := range subs {
		select {
		case <-s.done:
		default:
			t.Fatalf("the goroutine of %s is still running", s.id)
		}
	}

	// The buffered notifications may still be read, then the channels are closed.
	for range unread {
	}
	_, ok := <-idle
	assert.False(t, ok)
	_, ok = <-legacy
	assert.False(t, ok)

	tk.NotificationChannelsLock.RLock()
	assert.Empty(t, tk.live.subs)
	assert.Empty(t, tk.live.pending)
	assert.Empty(t, tk.NotificationChannels)
	tk.NotificationChannelsLock.RUnlock()

	// Nothing more is delivered or kept, and nobody can subscribe.
	assert.False(t, tk.SendNotification("idle", note(CreateAction, 0)))
	assert.False(t, tk.SendNotification("new", note(CreateAction, 0)))
	_, err = tk.CreateNotificationChannel("new")
	assert.ErrorIs(t, err, ErrLiveNotificationsClosed)
	_, err = tk.CreateNotificationChannel("pending")
	assert.ErrorIs(t, err, ErrLiveNotificationsClosed)

	// Kill closes the notifications of a live query; that must not fail
	// because the connection closed first.
	assert.NoError(t, tk.CloseLiveNotifications("idle"))
	assert.NoError(t, tk.CloseLiveNotifications("unread"))

	// Closing again, as both a lost connection and Close may do, is harmless.
	tk.CloseAllLiveNotifications()
}

// TestLive_drainAllDeliversQueued checks that losing the connection delivers
// what each live query has queued before closing its channel, and that
// closing afterwards stops whatever is still being delivered.
func TestLive_drainAllDeliversQueued(t *testing.T) {
	tk := newTestToolkit()
	read, err := tk.CreateNotificationChannel("read")
	require.NoError(t, err)
	abandoned, err := tk.CreateNotificationChannel("abandoned")
	require.NoError(t, err)
	const queued = 100
	for i := 0; i < queued; i++ {
		require.True(t, tk.SendNotification("read", note(CreateAction, i)))
		require.True(t, tk.SendNotification("abandoned", note(CreateAction, i)))
	}

	done := make(chan struct{})
	go func() {
		tk.DrainAllLiveNotifications()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DrainAllLiveNotifications blocked")
	}

	assert.False(t, tk.SendNotification("read", note(CreateAction, queued)))
	_, err = tk.CreateNotificationChannel("new")
	assert.ErrorIs(t, err, ErrLiveNotificationsClosed)

	for i := 0; i < queued; i++ {
		n, ok := receive(t, read)
		require.True(t, ok, "notification %d was dropped", i)
		require.Equal(t, i, n.Result)
	}
	_, ok := receive(t, read)
	assert.False(t, ok, "the channel is closed once its queue is delivered")
	require.Eventually(t, func() bool {
		_, exists := tk.GetNotificationChannel("read")
		return !exists
	}, time.Second, time.Millisecond)
	assert.NoError(t, tk.CloseLiveNotifications("read"))

	// Closing the connection stops the delivery nobody is reading.
	tk.NotificationChannelsLock.RLock()
	s := tk.live.subs["abandoned"]
	tk.NotificationChannelsLock.RUnlock()
	require.NotNil(t, s)
	tk.CloseAllLiveNotifications()
	select {
	case <-s.done:
	default:
		t.Fatal("the abandoned subscription's goroutine is still running")
	}
	for range abandoned {
	}
}

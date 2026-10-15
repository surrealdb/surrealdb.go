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

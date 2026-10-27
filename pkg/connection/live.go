package connection

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

const (
	// liveChannelBuffer is the capacity of the channel a live query's
	// notifications are delivered on. Notifications beyond it wait in the
	// subscription's queue rather than being dropped.
	liveChannelBuffer = 16

	// pendingTTL is how long notifications for a live query nobody has
	// subscribed to yet are kept. A live query's first notifications can arrive
	// between the response which names its id and the call to LiveNotifications.
	pendingTTL = 30 * time.Second

	// maxPending bounds the notifications kept for live queries nobody has
	// subscribed to, across all of them.
	maxPending = 10000

	// maxEnded bounds the ids of ended live queries that are remembered, so
	// that their stragglers are dropped rather than kept as pending.
	maxEnded = 1024
)

// ErrLiveNotificationsClosed is returned when subscribing to the notifications
// of a live query on a connection which has been closed or lost.
var ErrLiveNotificationsClosed = errors.New("live notifications closed: the connection is closed")

// subscription delivers the notifications of one live query in order, without
// ever blocking the goroutine reading from the connection: notifications queue
// up until the consumer takes them.
type subscription struct {
	id  string
	out chan Notification

	mu     sync.Mutex
	queue  []Notification
	ending bool // nothing more will be queued: close once the queue is delivered

	wake chan struct{} // signals that the queue grew
	stop chan struct{} // closed to end delivery at once
	done chan struct{} // closed once out is closed
}

func newSubscription(id string, queued []Notification) *subscription {
	s := &subscription{
		id:   id,
		out:  make(chan Notification, liveChannelBuffer),
		wake: make(chan struct{}, 1),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	for _, n := range queued {
		s.queue = append(s.queue, n)
		if n.Action == KilledAction {
			s.ending = true
			break
		}
	}
	return s
}

// push queues n for delivery. It never blocks. It returns false, dropping n,
// if the live query has already been killed.
func (s *subscription) push(n Notification) bool {
	s.mu.Lock()
	if s.ending {
		s.mu.Unlock()
		return false
	}
	s.queue = append(s.queue, n)
	if n.Action == KilledAction {
		s.ending = true
	}
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
	return true
}

// finish ends the subscription once what is already queued has been
// delivered, as no more notifications will come. It never blocks.
func (s *subscription) finish() {
	s.mu.Lock()
	s.ending = true
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// run delivers queued notifications until the live query is killed and its
// KILLED notification delivered, until the subscription is finished and its
// queue delivered, or until stop is closed. onEnd is called when delivery ends
// on its own rather than because stop was closed.
func (s *subscription) run(onEnd func(*subscription)) {
	defer close(s.done)
	defer close(s.out)

	for {
		s.mu.Lock()
		if len(s.queue) == 0 {
			ending := s.ending
			s.mu.Unlock()
			if ending {
				onEnd(s)
				return
			}
			select {
			case <-s.wake:
				continue
			case <-s.stop:
				return
			}
		}
		n := s.queue[0]
		s.queue[0] = Notification{}
		s.queue = s.queue[1:]
		s.mu.Unlock()

		select {
		case s.out <- n:
		case <-s.stop:
			return
		}
	}
}

type pendingNotifications struct {
	since time.Time
	items []Notification
}

// liveState is the part of Toolkit which routes live notifications.
// Its zero value is ready to use.
type liveState struct {
	subs       map[string]*subscription
	pending    map[string]*pendingNotifications
	pendingLen int
	ended      map[string]struct{}
	endedOrder []string

	// closed is set once the connection is closed or lost: no live query
	// can deliver notifications any more.
	closed bool
}

func (bc *Toolkit) liveInit() {
	if bc.live.subs == nil {
		bc.live.subs = make(map[string]*subscription)
		bc.live.pending = make(map[string]*pendingNotifications)
		bc.live.ended = make(map[string]struct{})
	}
	if bc.NotificationChannels == nil {
		bc.NotificationChannels = make(map[string]chan Notification)
	}
}

// CreateNotificationChannel subscribes to the notifications of a live query and
// returns the channel they are delivered on, in the order they arrived.
//
// Notifications are never dropped for a slow consumer: they queue until taken.
// Notifications which arrived before the subscription, after the live query was
// started, are delivered first. When the live query is killed, its KILLED
// notification is delivered and then the channel is closed.
func (bc *Toolkit) CreateNotificationChannel(liveQueryID string) (chan Notification, error) {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()
	bc.liveInit()

	if bc.live.closed {
		return nil, fmt.Errorf("%w: %v", ErrLiveNotificationsClosed, liveQueryID)
	}
	if _, ok := bc.live.subs[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, liveQueryID)
	}
	if _, ok := bc.NotificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, liveQueryID)
	}

	var queued []Notification
	if p, ok := bc.live.pending[liveQueryID]; ok {
		queued = p.items
		bc.live.pendingLen -= len(p.items)
		delete(bc.live.pending, liveQueryID)
	}
	delete(bc.live.ended, liveQueryID)

	s := newSubscription(liveQueryID, queued)
	bc.live.subs[liveQueryID] = s
	bc.NotificationChannels[liveQueryID] = s.out
	go s.run(bc.subscriptionEnded)

	return s.out, nil
}

// subscriptionEnded forgets a subscription whose live query was killed.
func (bc *Toolkit) subscriptionEnded(s *subscription) {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()

	if bc.live.subs[s.id] == s {
		delete(bc.live.subs, s.id)
		delete(bc.NotificationChannels, s.id)
		bc.markEnded(s.id)
	}
}

func (bc *Toolkit) markEnded(id string) {
	if _, ok := bc.live.ended[id]; ok {
		return
	}
	bc.live.ended[id] = struct{}{}
	bc.live.endedOrder = append(bc.live.endedOrder, id)
	if len(bc.live.endedOrder) > maxEnded {
		delete(bc.live.ended, bc.live.endedOrder[0])
		bc.live.endedOrder = bc.live.endedOrder[1:]
	}
}

func (bc *Toolkit) GetNotificationChannel(id string) (chan Notification, bool) {
	bc.NotificationChannelsLock.RLock()
	defer bc.NotificationChannelsLock.RUnlock()
	ch, ok := bc.NotificationChannels[id]

	return ch, ok
}

func (bc *Toolkit) LiveNotifications(liveQueryID string) (chan Notification, error) {
	c, err := bc.CreateNotificationChannel(liveQueryID)
	if err != nil && bc.Logger != nil {
		bc.Logger.Error(err.Error())
	}
	return c, err
}

// CloseLiveNotifications stops delivering the notifications of a live query and
// closes its channel. Notifications still queued are discarded; those already
// in the channel's buffer can still be read.
//
// It is not an error to close the notifications of a live query which ended
// with a KILLED notification, or which ended because the connection closed.
func (bc *Toolkit) CloseLiveNotifications(liveQueryID string) error {
	bc.NotificationChannelsLock.Lock()
	bc.liveInit()

	s, ok := bc.live.subs[liveQueryID]
	if !ok {
		ch, legacy := bc.NotificationChannels[liveQueryID]
		_, ended := bc.live.ended[liveQueryID]
		closed := bc.live.closed
		delete(bc.NotificationChannels, liveQueryID)
		bc.NotificationChannelsLock.Unlock()

		switch {
		case legacy:
			// A channel put in NotificationChannels directly.
			close(ch)
			return nil
		case ended, closed:
			return nil
		default:
			return fmt.Errorf("notification channel not found for live query ID: %s", liveQueryID)
		}
	}

	delete(bc.live.subs, liveQueryID)
	delete(bc.NotificationChannels, liveQueryID)
	bc.markEnded(liveQueryID)
	bc.NotificationChannelsLock.Unlock()

	close(s.stop)
	<-s.done
	return nil
}

// CloseAllLiveNotifications ends the notifications of every live query, for a
// connection which has been closed: no more notifications will come. Each
// subscription's channel is closed, without a KILLED notification, and its
// goroutine has stopped by the time this returns. Notifications still queued are
// discarded; those already in a channel's buffer can still be read. This
// includes subscriptions still delivering after DrainAllLiveNotifications.
//
// Afterwards, notifications are dropped and subscribing fails with
// ErrLiveNotificationsClosed. Closing the notifications of a live query which
// was ended this way is not an error.
//
// WebSocket connections call it when they are closed.
func (bc *Toolkit) CloseAllLiveNotifications() {
	bc.NotificationChannelsLock.Lock()
	bc.liveInit()

	bc.live.closed = true
	subs := make([]*subscription, 0, len(bc.live.subs))
	for id, s := range bc.live.subs {
		subs = append(subs, s)
		delete(bc.live.subs, id)
		delete(bc.NotificationChannels, id)
	}
	for id, ch := range bc.NotificationChannels {
		// A channel put in NotificationChannels directly. SendNotification
		// only sends on it with the lock held, so it is safe to close here.
		close(ch)
		delete(bc.NotificationChannels, id)
	}
	bc.live.pending = make(map[string]*pendingNotifications)
	bc.live.pendingLen = 0
	bc.NotificationChannelsLock.Unlock()

	// Without the lock: a subscription ending on its own takes it.
	for _, s := range subs {
		close(s.stop)
	}
	for _, s := range subs {
		<-s.done
	}
}

// DrainAllLiveNotifications ends the notifications of every live query, for a
// connection which has been lost: no more notifications will come, but those
// already received are not lost. Each subscription delivers what it has
// queued, then its channel is closed, without a KILLED notification. It does
// not wait for that: a subscription whose consumer is behind goes on
// delivering until its consumer catches up, CloseLiveNotifications closes it,
// or CloseAllLiveNotifications closes them all.
//
// Afterwards, as after CloseAllLiveNotifications, notifications are dropped and
// subscribing fails with ErrLiveNotificationsClosed.
//
// WebSocket connections call it when they lose their connection.
func (bc *Toolkit) DrainAllLiveNotifications() {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()
	bc.liveInit()

	bc.live.closed = true
	for _, s := range bc.live.subs {
		s.finish()
	}
	for id, ch := range bc.NotificationChannels {
		if _, ok := bc.live.subs[id]; ok {
			continue
		}
		// A channel put in NotificationChannels directly. SendNotification
		// only sends on it with the lock held, so it is safe to close here.
		close(ch)
		delete(bc.NotificationChannels, id)
	}
	bc.live.pending = make(map[string]*pendingNotifications)
	bc.live.pendingLen = 0
}

// SendNotification routes a notification to the subscription for its live
// query. It never blocks and never drops a notification for a slow consumer.
//
// A notification for a live query nobody has subscribed to yet is kept for a
// while, for the subscription which is likely about to be made. It returns
// false if the notification was dropped: because its live query has ended, or
// because too many notifications are waiting for a subscription.
func (bc *Toolkit) SendNotification(id string, notification Notification) bool {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()
	bc.liveInit()

	if bc.live.closed {
		return false
	}
	if s, ok := bc.live.subs[id]; ok {
		return s.push(notification)
	}

	if ch, ok := bc.NotificationChannels[id]; ok {
		// A channel put in NotificationChannels directly.
		select {
		case ch <- notification:
			return true
		default:
			return false
		}
	}

	if _, ok := bc.live.ended[id]; ok {
		return false
	}

	return bc.keepPending(id, notification)
}

func (bc *Toolkit) keepPending(id string, notification Notification) bool {
	now := time.Now()
	for pid, p := range bc.live.pending {
		if now.Sub(p.since) > pendingTTL {
			bc.live.pendingLen -= len(p.items)
			delete(bc.live.pending, pid)
		}
	}

	if bc.live.pendingLen >= maxPending {
		return false
	}

	p, ok := bc.live.pending[id]
	if !ok {
		p = &pendingNotifications{since: now}
		bc.live.pending[id] = p
	}
	p.items = append(p.items, notification)
	bc.live.pendingLen++
	return true
}

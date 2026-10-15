package connection

import (
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

// subscription delivers the notifications of one live query in order, without
// ever blocking the goroutine reading from the connection: notifications queue
// up until the consumer takes them.
type subscription struct {
	id  string
	out chan Notification

	mu     sync.Mutex
	queue  []Notification
	ending bool // a KILLED notification is queued: close once it is delivered

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

// run delivers queued notifications until the live query is killed and its
// KILLED notification delivered, or until stop is closed. onEnd is called when
// delivery ends because the live query was killed.
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
// with a KILLED notification.
func (bc *Toolkit) CloseLiveNotifications(liveQueryID string) error {
	bc.NotificationChannelsLock.Lock()
	bc.liveInit()

	s, ok := bc.live.subs[liveQueryID]
	if !ok {
		ch, legacy := bc.NotificationChannels[liveQueryID]
		_, ended := bc.live.ended[liveQueryID]
		delete(bc.NotificationChannels, liveQueryID)
		bc.NotificationChannelsLock.Unlock()

		switch {
		case legacy:
			// A channel put in NotificationChannels directly.
			close(ch)
			return nil
		case ended:
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

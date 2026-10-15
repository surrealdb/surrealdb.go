package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

const (
	// DefaultReconnectMinDelay is how long the client waits before its second
	// attempt to reconnect. The first attempt is made at once.
	DefaultReconnectMinDelay = 500 * time.Millisecond
	// DefaultReconnectMaxDelay bounds the wait between attempts to reconnect,
	// which doubles after each failed attempt.
	DefaultReconnectMaxDelay = 30 * time.Second

	// reconnectAttemptTimeout bounds one attempt: connecting and restoring.
	reconnectAttemptTimeout = 30 * time.Second
)

// ErrClientClosed is returned for requests made after [DB.Close].
var ErrClientClosed = errors.New("client closed")

// ConnectionStatus is the state of a client's connection.
type ConnectionStatus int

const (
	// StatusConnected means requests are sent to the server.
	StatusConnected ConnectionStatus = iota + 1
	// StatusDisconnected means the connection was lost, or an attempt to
	// reconnect failed. Requests wait for the client to reconnect.
	StatusDisconnected
	// StatusReconnecting means the client is trying to reconnect.
	StatusReconnecting
	// StatusClosed means the client was closed, or gave up reconnecting.
	// Requests fail.
	StatusClosed
)

func (s ConnectionStatus) String() string {
	switch s {
	case StatusConnected:
		return "connected"
	case StatusDisconnected:
		return "disconnected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusClosed:
		return "closed"
	}
	return fmt.Sprintf("ConnectionStatus(%d)", int(s))
}

// StatusEvent reports a change of the status of a client's connection.
type StatusEvent struct {
	Status ConnectionStatus
	// Attempt counts the attempts to reconnect since the connection was lost,
	// from 1. It is 0 for the event which reports the loss.
	Attempt int
	// Err is why the connection was lost or an attempt failed. On a
	// StatusConnected event after reconnecting, it is what could not be
	// restored on the new connection, such as an expired token, or nil if
	// everything was. On StatusClosed, it is why the client gave up.
	Err error
}

// reconnector is the connection of a client which reconnects. It sends each
// request on the current connection, and when the connection is lost, makes a
// new one, has the client restore its state on it, and swaps it in. Requests
// made meanwhile wait for it.
type reconnector struct {
	minDelay, maxDelay time.Duration
	maxAttempts        int

	// restore sets the client's state on a new connection before it is used.
	// An error leaves the connection in use, unless the connection was lost
	// too.
	restore func(ctx context.Context, con connection.Connection) error

	mu      sync.RWMutex
	cur     connection.Connection
	status  ConnectionStatus
	err     error
	changed chan struct{}

	stop     context.Context
	cancel   context.CancelFunc
	loopDone chan struct{}

	listenersMu sync.Mutex
	listeners   map[int]func(StatusEvent)
	nextID      int

	lives liveRouter
}

var (
	_ connection.WebSocketConnection = (*reconnector)(nil)
	_ connection.HealthChecker       = (*reconnector)(nil)
	_ connection.HTTPEndpoint        = (*reconnector)(nil)
)

// canReconnect reports whether a connection can be reconnected by the client.
func canReconnect(con connection.Connection) bool {
	_, redials := con.(connection.Redialer)
	_, notifies := con.(connection.CloseNotifier)
	return redials && notifies
}

// newReconnector watches a connected connection, and reconnects when it is
// lost. Each connection it makes comes from Redial of the one before.
func newReconnector(con connection.Connection, o *options) *reconnector {
	stop, cancel := context.WithCancel(context.Background())
	r := &reconnector{
		minDelay:    o.reconnectMinDelay,
		maxDelay:    o.reconnectMaxDelay,
		maxAttempts: o.reconnectAttempts,
		cur:         con,
		status:      StatusConnected,
		changed:     make(chan struct{}),
		stop:        stop,
		cancel:      cancel,
		loopDone:    make(chan struct{}),
		lives:       liveRouter{queries: map[string]*liveQuery{}},
	}
	go r.watch(con)
	return r
}

// setState changes the status, and wakes the requests waiting for a change.
func (r *reconnector) setState(status ConnectionStatus, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setStateLocked(status, nil, err)
}

func (r *reconnector) setStateLocked(status ConnectionStatus, con connection.Connection, err error) {
	if r.status == StatusClosed {
		return
	}
	if con != nil {
		r.cur = con
	}
	r.status = status
	r.err = err
	close(r.changed)
	r.changed = make(chan struct{})
}

// current returns the connection the client last connected with, connected or
// not.
func (r *reconnector) current() connection.Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cur
}

// conn returns the connection to send a request on, waiting until the client
// is connected.
func (r *reconnector) conn(ctx context.Context) (connection.Connection, error) {
	for {
		r.mu.RLock()
		cur, status, err, changed := r.cur, r.status, r.err, r.changed
		r.mu.RUnlock()

		switch status {
		case StatusConnected:
			return cur, nil
		case StatusClosed:
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrClientClosed, err)
			}
			return nil, ErrClientClosed
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting to reconnect: %w", ctx.Err())
		}
	}
}

// Status returns the current status.
func (r *reconnector) Status() ConnectionStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

func (r *reconnector) onStatus(fn func(StatusEvent)) func() {
	r.listenersMu.Lock()
	defer r.listenersMu.Unlock()
	if r.listeners == nil {
		r.listeners = map[int]func(StatusEvent){}
	}
	id := r.nextID
	r.nextID++
	r.listeners[id] = fn
	return func() {
		r.listenersMu.Lock()
		defer r.listenersMu.Unlock()
		delete(r.listeners, id)
	}
}

func (r *reconnector) emit(ev StatusEvent) {
	r.listenersMu.Lock()
	fns := make([]func(StatusEvent), 0, len(r.listeners))
	for i := 0; i < r.nextID; i++ {
		if fn, ok := r.listeners[i]; ok {
			fns = append(fns, fn)
		}
	}
	r.listenersMu.Unlock()

	for _, fn := range fns {
		fn(ev)
	}
}

// watch waits for the connection to be lost, and reconnects, until the client
// is closed or gives up.
func (r *reconnector) watch(con connection.Connection) {
	defer close(r.loopDone)
	for {
		select {
		case <-con.(connection.CloseNotifier).Closed():
		case <-r.stop.Done():
			return
		}
		if r.stop.Err() != nil {
			return
		}

		cause := con.(connection.CloseNotifier).CloseError()
		if cause == nil {
			cause = connection.ErrConnectionClosed
		}
		r.setState(StatusDisconnected, cause)
		r.emit(StatusEvent{Status: StatusDisconnected, Err: cause})

		next, err := r.reconnect(con)
		if err != nil {
			return
		}
		con = next
	}
}

// reconnect makes a new connection, with backoff between attempts, and
// returns it once the client's state is restored on it.
func (r *reconnector) reconnect(lost connection.Connection) (connection.Connection, error) {
	delay := r.minDelay
	for attempt := 1; ; attempt++ {
		r.setState(StatusReconnecting, nil)
		r.emit(StatusEvent{Status: StatusReconnecting, Attempt: attempt})

		next, partial, err := r.attempt(lost)
		if err == nil {
			r.mu.Lock()
			if r.status == StatusClosed {
				r.mu.Unlock()
				_ = next.Close(context.Background())
				return nil, ErrClientClosed
			}
			r.setStateLocked(StatusConnected, next, nil)
			r.mu.Unlock()
			r.emit(StatusEvent{Status: StatusConnected, Attempt: attempt, Err: partial})
			return next, nil
		}

		if r.stop.Err() != nil {
			return nil, ErrClientClosed
		}
		r.setState(StatusDisconnected, err)
		r.emit(StatusEvent{Status: StatusDisconnected, Attempt: attempt, Err: err})

		if r.maxAttempts > 0 && attempt >= r.maxAttempts {
			err = fmt.Errorf("gave up reconnecting after %d attempts: %w", attempt, err)
			r.setState(StatusClosed, err)
			r.lives.closeAll()
			r.emit(StatusEvent{Status: StatusClosed, Attempt: attempt, Err: err})
			return nil, err
		}

		select {
		case <-time.After(delay):
		case <-r.stop.Done():
			return nil, ErrClientClosed
		}
		delay = min(delay*2, r.maxDelay)
	}
}

// attempt connects once and restores the client's state. It fails when either
// fails because of the connection; what could not be restored otherwise is
// returned as partial.
func (r *reconnector) attempt(lost connection.Connection) (next connection.Connection, partial, err error) {
	ctx, cancel := context.WithTimeout(r.stop, reconnectAttemptTimeout)
	defer cancel()

	next = lost.(connection.Redialer).Redial()
	if err := next.Connect(ctx); err != nil {
		return nil, nil, err
	}

	if r.restore != nil {
		partial = r.restore(ctx, next)
	}
	select {
	case <-next.(connection.CloseNotifier).Closed():
		err := next.(connection.CloseNotifier).CloseError()
		if err == nil {
			err = connection.ErrConnectionClosed
		}
		if partial != nil {
			err = fmt.Errorf("%w (while restoring: %w)", err, partial)
		}
		return nil, nil, err
	default:
	}
	if ctx.Err() != nil {
		_ = next.Close(context.Background())
		return nil, nil, fmt.Errorf("restoring the client: %w", ctx.Err())
	}
	return next, partial, nil
}

// Connect does nothing: the connection is connected before the reconnector
// takes it over.
func (r *reconnector) Connect(context.Context) error {
	return nil
}

// Close stops reconnecting, and closes the current connection.
func (r *reconnector) Close(ctx context.Context) error {
	r.mu.Lock()
	if r.status == StatusClosed {
		r.mu.Unlock()
		return nil
	}
	r.setStateLocked(StatusClosed, nil, nil)
	r.mu.Unlock()

	r.cancel()
	select {
	case <-r.loopDone:
	case <-ctx.Done():
	}
	r.lives.closeAll()
	r.emit(StatusEvent{Status: StatusClosed})
	return r.current().Close(ctx)
}

// IsClosed reports whether the client was closed, or gave up reconnecting.
func (r *reconnector) IsClosed() bool {
	return r.Status() == StatusClosed
}

func (r *reconnector) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	con, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	return con.Send(ctx, method, params...)
}

func (r *reconnector) Call(ctx context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	con, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	return con.Call(ctx, req)
}

func (r *reconnector) Use(ctx context.Context, namespace, database string) error {
	con, err := r.conn(ctx)
	if err != nil {
		return err
	}
	return con.Use(ctx, namespace, database)
}

func (r *reconnector) Let(ctx context.Context, key string, value any) error {
	con, err := r.conn(ctx)
	if err != nil {
		return err
	}
	return con.Let(ctx, key, value)
}

func (r *reconnector) Unset(ctx context.Context, key string) error {
	con, err := r.conn(ctx)
	if err != nil {
		return err
	}
	return con.Unset(ctx, key)
}

func (r *reconnector) Authenticate(ctx context.Context, token string) error {
	con, err := r.conn(ctx)
	if err != nil {
		return err
	}
	return con.Authenticate(ctx, token)
}

func (r *reconnector) Invalidate(ctx context.Context) error {
	con, err := r.conn(ctx)
	if err != nil {
		return err
	}
	return con.Invalidate(ctx)
}

func (r *reconnector) SignUp(ctx context.Context, authData any) (string, error) {
	con, err := r.conn(ctx)
	if err != nil {
		return "", err
	}
	return con.SignUp(ctx, authData)
}

func (r *reconnector) SignIn(ctx context.Context, authData any) (string, error) {
	con, err := r.conn(ctx)
	if err != nil {
		return "", err
	}
	return con.SignIn(ctx, authData)
}

func (r *reconnector) SignUpWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	con, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	return con.SignUpWithRefresh(ctx, authData)
}

func (r *reconnector) SignInWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	con, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	return con.SignInWithRefresh(ctx, authData)
}

func (r *reconnector) GetUnmarshaler() codec.Unmarshaler {
	return r.current().GetUnmarshaler()
}

func (r *reconnector) Health(ctx context.Context) error {
	hc, ok := r.current().(connection.HealthChecker)
	if !ok {
		return fmt.Errorf("%w: health", constants.ErrMethodNotAvailable)
	}
	return hc.Health(ctx)
}

func (r *reconnector) HTTPEndpoint() (baseURL string, client *http.Client) {
	if he, ok := r.current().(connection.HTTPEndpoint); ok {
		return he.HTTPEndpoint()
	}
	return "", nil
}

func (r *reconnector) LiveNotifications(id string) (chan connection.Notification, error) {
	if ch, ok, err := r.lives.notifications(id, r.current()); ok {
		return ch, err
	}
	return r.current().LiveNotifications(id)
}

func (r *reconnector) CloseLiveNotifications(id string) error {
	if ok, err := r.lives.close(id, r.current()); ok {
		return err
	}
	return r.current().CloseLiveNotifications(id)
}

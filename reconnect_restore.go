package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// restore sets what the client holds on a new connection, after the one before
// was lost: the namespace, database, variables and authentication of the
// default session and of each attached session, and the live queries started
// with [Live]. It carries on past what fails, and returns all of it.
func (db *DB) restore(ctx context.Context, con connection.Connection) error {
	var errs []error

	if err := restoreSession(ctx, con, nil, &db.state, db.renew); err != nil {
		errs = append(errs, fmt.Errorf("restoring the default session: %w", err))
	}

	db.sessionsMu.Lock()
	sessions := make([]*Session, 0, len(db.sessions))
	for _, s := range db.sessions {
		sessions = append(sessions, s)
	}
	db.sessionsMu.Unlock()

	// A session being detached is still listed, and holds its lock while its
	// detach waits for the connection, so it is restored like the others.
	for _, s := range sessions {
		if err := callOn[any](ctx, con, s.id, connection.Attach); err != nil {
			errs = append(errs, fmt.Errorf("attaching session %s: %w", s.id, err))
			continue
		}
		if err := restoreSession(ctx, con, s.id, &s.state, s.renew); err != nil {
			errs = append(errs, fmt.Errorf("restoring session %s: %w", s.id, err))
		}
	}

	if db.rc != nil {
		errs = append(errs, db.rc.lives.restore(ctx, con)...)
	}
	return errors.Join(errs...)
}

// restoreSession sets the state of a session on a connection: its token
// first, since SurrealDB 2.x refuses variables to an anonymous session, then
// its namespace and database, and its variables. A token pair whose access
// token has expired is exchanged for a new one first. It carries on past what
// fails, and returns all of it.
func restoreSession(ctx context.Context, con connection.Connection, session *models.UUID, state *sessionState, renew *renewer) error {
	snap := state.snapshot(renew)
	var errs []error

	token := snap.token
	if pair := renew.current(); pair != nil && tokenExpired(pair.Access) {
		var next Tokens
		if err := callOnInto(ctx, con, session, &next, connection.Refresh, *pair); err != nil {
			errs = append(errs, fmt.Errorf("the access token expired, and renewing it failed: %w", err))
			token = ""
		} else {
			renew.set(&next)
			state.setToken(next.Access)
			token = next.Access
		}
	}
	if token != "" {
		if err := callOn[any](ctx, con, session, connection.Authenticate, token); err != nil {
			errs = append(errs, fmt.Errorf("authenticate: %w", err))
		}
	}

	if snap.namespace != "" || snap.database != "" {
		if err := callOn[any](ctx, con, session, connection.Use, snap.namespace, snap.database); err != nil {
			errs = append(errs, fmt.Errorf("use: %w", err))
		}
	}
	for key, val := range snap.vars {
		if err := callOn[any](ctx, con, session, connection.Let, key, val); err != nil {
			errs = append(errs, fmt.Errorf("let $%s: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

// tokenExpired reports whether a JWT has expired, or will in a moment.
func tokenExpired(token string) bool {
	exp, ok := tokenExpiry(token)
	return ok && time.Until(exp) < 5*time.Second
}

// callOn sends a request on a connection, in a session, or the default one
// when session is nil.
func callOn[T any](
	ctx context.Context, con connection.Connection, session *models.UUID, method connection.RPCFunction, params ...any,
) error {
	var res T
	return callOnInto(ctx, con, session, &res, method, params...)
}

func callOnInto[T any](
	ctx context.Context, con connection.Connection, session *models.UUID, out *T, method connection.RPCFunction, params ...any,
) error {
	var res connection.RPCResponse[T]
	req := &connection.RPCRequest{Method: string(method), Params: params, Session: session}
	if err := connection.Call(con, ctx, &res, req); err != nil {
		return err
	}
	if res.Result != nil {
		*out = *res.Result
	}
	return nil
}

// liveQuery is a live query the client started with [Live], which it starts
// again on each new connection. Its ID stays the one [Live] returned; the
// server gives it a new one each time.
type liveQuery struct {
	id       models.UUID
	session  *models.UUID
	table    models.Table
	diff     bool
	serverID string
	// out is the channel [DB.LiveNotifications] returned, if it was called,
	// and pump forwards the notifications of the current connection to it.
	out  chan connection.Notification
	pump *pump
}

// pump forwards notifications from a connection to a live query's channel,
// under the live query's ID.
type pump struct {
	stop chan struct{}
	done chan struct{}
}

func startPump(src <-chan connection.Notification, out chan<- connection.Notification, id models.UUID) *pump {
	p := &pump{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		for {
			select {
			case n, ok := <-src:
				if !ok {
					return
				}
				n.ID = &id
				select {
				case out <- n:
				case <-p.stop:
					return
				}
			case <-p.stop:
				return
			}
		}
	}()
	return p
}

func (p *pump) halt() {
	if p == nil {
		return
	}
	close(p.stop)
	<-p.done
}

// liveRouter keeps the live queries a reconnecting client restores.
type liveRouter struct {
	mu      sync.Mutex
	queries map[string]*liveQuery
}

func (l *liveRouter) track(id models.UUID, session *models.UUID, table models.Table, diff bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries[id.String()] = &liveQuery{id: id, session: session, table: table, diff: diff, serverID: id.String()}
}

// serverID returns the ID the server knows a live query by now.
func (l *liveRouter) serverID(id string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if q, ok := l.queries[id]; ok {
		return q.serverID
	}
	return id
}

// notifications returns the channel of a live query this router keeps; ok is
// false for one it does not.
func (l *liveRouter) notifications(id string, con connection.Connection) (ch chan connection.Notification, ok bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	q, ok := l.queries[id]
	if !ok {
		return nil, false, nil
	}
	if q.out != nil {
		return nil, true, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}
	src, err := con.LiveNotifications(q.serverID)
	if err != nil {
		return nil, true, err
	}
	q.out = make(chan connection.Notification, cap(src))
	q.pump = startPump(src, q.out, q.id)
	return q.out, true, nil
}

// close stops keeping a live query and closes its channel; ok is false for
// one this router does not keep.
func (l *liveRouter) close(id string, con connection.Connection) (ok bool, err error) {
	l.mu.Lock()
	q, ok := l.queries[id]
	delete(l.queries, id)
	l.mu.Unlock()
	if !ok {
		return false, nil
	}
	if q.out == nil {
		return true, nil
	}
	q.pump.halt()
	close(q.out)
	return true, con.CloseLiveNotifications(q.serverID)
}

// forgetSession stops keeping the live queries of a session, the default one
// when session is nil, which the server ended when the session was reset or
// detached. It closes their channels, and those con kept for them.
func (l *liveRouter) forgetSession(session *models.UUID, con connection.Connection) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, q := range l.queries {
		if !sameSession(q.session, session) {
			continue
		}
		if q.out != nil {
			q.pump.halt()
			close(q.out)
			_ = con.CloseLiveNotifications(q.serverID)
		}
		delete(l.queries, id)
	}
}

func sameSession(a, b *models.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// closeAll closes the channels of every live query, when the client closes.
func (l *liveRouter) closeAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, q := range l.queries {
		if q.out != nil {
			q.pump.halt()
			close(q.out)
		}
		delete(l.queries, id)
	}
}

// restore starts each live query again on a new connection, and forwards its
// notifications to the channel it had.
func (l *liveRouter) restore(ctx context.Context, con connection.Connection) []error {
	l.mu.Lock()
	queries := make([]*liveQuery, 0, len(l.queries))
	for _, q := range l.queries {
		queries = append(queries, q)
	}
	l.mu.Unlock()

	var errs []error
	for _, q := range queries {
		var serverID models.UUID
		if err := callOnInto(ctx, con, q.session, &serverID, connection.Live, q.table, q.diff); err != nil {
			errs = append(errs, fmt.Errorf("restarting live query %s: %w", q.id, err))
			continue
		}

		l.mu.Lock()
		if _, kept := l.queries[q.id.String()]; kept {
			q.serverID = serverID.String()
			if q.out != nil {
				q.pump.halt()
				q.pump = nil
				src, err := con.LiveNotifications(q.serverID)
				if err != nil {
					errs = append(errs, fmt.Errorf("forwarding live query %s: %w", q.id, err))
				} else {
					q.pump = startPump(src, q.out, q.id)
				}
			}
		}
		l.mu.Unlock()
	}
	return errs
}

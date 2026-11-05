package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/v2/pkg/connection"
	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// restore sets what the client holds on a new connection, after the one before
// was lost: the namespace, database, variables and authentication of the
// default session and of each attached session, and the live queries started
// with [Live]. It carries on past what fails, and returns all of it.
func (db *DB) restore(ctx context.Context, con connection.Connection) error {
	var errs []error
	sessions := make(map[string]restoredSession)

	set, err := restoreSession(ctx, con, nil, db.state, db.renew)
	if err != nil {
		errs = append(errs, fmt.Errorf("restoring the default session: %w", err))
	}
	sessions[sessionKey(nil)] = set

	// A session being detached is still listed, and holds its lock while its
	// detach waits for the connection, so it is restored like the others.
	for _, s := range db.sessions.all() {
		if err := callOn[any](ctx, con, s.id, connection.Attach); err != nil {
			errs = append(errs, fmt.Errorf("attaching session %s: %w", s.id, err))
			continue
		}
		set, err := restoreSession(ctx, con, s.id, &s.state, s.renew)
		if err != nil {
			errs = append(errs, fmt.Errorf("restoring session %s: %w", s.id, err))
		}
		sessions[sessionKey(s.id)] = set
	}

	if db.rc != nil {
		errs = append(errs, db.rc.lives.restore(ctx, con, sessions)...)
	}
	return errors.Join(errs...)
}

// restoredSession is what restoreSession set a session to on a new connection:
// the namespace and database it selected, and who it signed the session in as.
type restoredSession struct {
	namespace, database string
	who                 identity
}

// sessionKey keys the sessions restored on a connection: the default session,
// when session is nil, or an attached one.
func sessionKey(session *models.UUID) string {
	if session == nil {
		return ""
	}
	return session.String()
}

// restoreSession sets the state of a session on a connection: its token
// first, so that the rest is set as the session's user, then its namespace
// and database, and its variables. A token pair whose access
// token has expired is exchanged for a new one first, when the client holds
// the pair, as it does with [WithTokenRenewal]. It carries on past what fails,
// and returns all of it, with what it did set.
func restoreSession(
	ctx context.Context, con connection.Connection, session *models.UUID, state *sessionState, renew *renewer,
) (restoredSession, error) {
	snap := state.snapshot(renew)
	var errs []error
	var set restoredSession

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
		} else {
			set.who = identityOf(token)
		}
	}

	if snap.namespace != "" || snap.database != "" {
		if err := callOn[any](ctx, con, session, connection.Use, snap.namespace, snap.database); err != nil {
			errs = append(errs, fmt.Errorf("use: %w", err))
		} else {
			set.namespace, set.database = snap.namespace, snap.database
		}
	}
	for key, val := range snap.vars {
		if err := callOn[any](ctx, con, session, connection.Let, key, val); err != nil {
			errs = append(errs, fmt.Errorf("let $%s: %w", key, err))
		}
	}
	return set, errors.Join(errs...)
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
	// namespace, database and who are what its session had selected and was
	// signed in as when the query started, which the server bound it to.
	namespace, database string
	who                 identity
	// out is the channel [DB.LiveNotifications] returned, if it was called,
	// and pump forwards the notifications of the current connection to it.
	out  chan connection.Notification
	pump *pump
	// ended says why the client ended the query instead of starting it
	// again on a new connection. Its channel is closed then.
	ended error
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

// track keeps a live query started in a session with the given state, so that
// it is started again there on each new connection.
func (l *liveRouter) track(id models.UUID, session *models.UUID, table models.Table, diff bool, started sessionSnapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries[id.String()] = &liveQuery{
		id:        id,
		session:   session,
		table:     table,
		diff:      diff,
		serverID:  id.String(),
		namespace: started.namespace,
		database:  started.database,
		who:       identityOf(started.token),
	}
}

// serverID returns the ID the server knows a live query by now. ended is true
// for a live query the client ended on reconnecting, which the server no
// longer runs.
func (l *liveRouter) serverID(id string) (serverID string, ended bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if q, ok := l.queries[id]; ok {
		return q.serverID, q.ended != nil
	}
	return id, false
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
	if q.ended != nil {
		return nil, true, q.ended
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

// restore starts each live query again on a new connection, in the namespace
// and database it was started in, and forwards its notifications to the
// channel it had. sessions are what each session was restored to.
//
// A live query whose session is no longer signed in as it was when the query
// started is ended instead, as starting it again would run it as someone else.
// So is one the server refuses to start again. Its channel is closed, and the
// error returned for it wraps [ErrLiveQueryEnded].
func (l *liveRouter) restore(ctx context.Context, con connection.Connection, sessions map[string]restoredSession) []error {
	l.mu.Lock()
	queries := make([]*liveQuery, 0, len(l.queries))
	for _, q := range l.queries {
		if q.ended == nil {
			queries = append(queries, q)
		}
	}
	l.mu.Unlock()

	var errs []error
	for _, q := range queries {
		set, restored := sessions[sessionKey(q.session)]
		if !restored || set.who != q.who {
			why := "its session is no longer signed in as it was when the query started"
			if !restored {
				why = "its session could not be attached again"
			}
			errs = append(errs, l.endUnlessGone(ctx, con, q, errors.New(why)))
			continue
		}

		serverID, err := restartLive(ctx, con, q, &set)
		if err != nil {
			if refusedByServer(err) {
				errs = append(errs, l.endUnlessGone(ctx, con, q, err))
			} else {
				errs = append(errs, fmt.Errorf("restarting live query %s: %w", q.id, err))
			}
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

// endUnlessGone ends a live query which could not be started again on a new
// connection, and returns the error that says why. When the connection was
// lost meanwhile, the query is kept instead, as the next attempt to reconnect
// starts it again.
func (l *liveRouter) endUnlessGone(ctx context.Context, con connection.Connection, q *liveQuery, why error) error {
	if gone(ctx, con) {
		return fmt.Errorf("restarting live query %s: %w", q.id, why)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.queries[q.id.String()] != q {
		// Killed, or its notifications closed, meanwhile.
		return nil
	}
	err := fmt.Errorf("%w: %s was not started again after reconnecting: %w", ErrLiveQueryEnded, q.id, why)
	q.ended = err
	if q.out != nil {
		q.pump.halt()
		q.pump = nil
		close(q.out)
		q.out = nil
	}
	return err
}

// gone reports whether a restore can no longer go on, as its context ended or
// the connection closed.
func gone(ctx context.Context, con connection.Connection) bool {
	if ctx.Err() != nil {
		return true
	}
	if cn, ok := con.(connection.CloseNotifier); ok {
		select {
		case <-cn.Closed():
			return true
		default:
		}
	}
	return false
}

// restartLive starts a live query again on a connection, in the namespace and
// database it was started in, and returns the ID the server gave it there.
// When its session, as restored, has others selected, it selects the query's
// for the request, and the session's own again after it.
func restartLive(ctx context.Context, con connection.Connection, q *liveQuery, set *restoredSession) (models.UUID, error) {
	elsewhere := (q.namespace != "" || q.database != "") && (q.namespace != set.namespace || q.database != set.database)
	if !elsewhere {
		return startLive(ctx, con, q)
	}

	if err := callOn[any](ctx, con, q.session, connection.Use, q.namespace, q.database); err != nil {
		return models.UUID{}, fmt.Errorf("use: %w", err)
	}
	serverID, err := startLive(ctx, con, q)
	// NULL clears what is selected, where an empty name would be selected.
	var namespace, database any
	if set.namespace != "" {
		namespace = set.namespace
	}
	if set.database != "" {
		database = set.database
	}
	if backErr := callOn[any](ctx, con, q.session, connection.Use, namespace, database); backErr != nil {
		// Left so, the session would run the caller's requests in the live
		// query's database. The connection is closed instead, so that the
		// client connects and restores its state again.
		_ = con.Close(context.Background())
		return serverID, errors.Join(err, fmt.Errorf("selecting the session's own namespace and database again: %w", backErr))
	}
	return serverID, err
}

// startLive starts a live query on a connection, in its session as it is, with
// the statement [Live] started it with, and returns the ID the server gave it.
func startLive(ctx context.Context, con connection.Connection, q *liveQuery) (models.UUID, error) {
	var serverID models.UUID
	var results []statementResult
	vars := map[string]any{"what": q.table}
	if err := callOnInto(ctx, con, q.session, &results, connection.Query, liveStatement(q.diff), vars); err != nil {
		return serverID, err
	}
	raw, err := oneResult(&results, con.GetUnmarshaler())
	if err != nil {
		return serverID, err
	}
	err = con.GetUnmarshaler().Unmarshal(raw, &serverID)
	return serverID, err
}

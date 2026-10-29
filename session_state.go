package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// sessionState is what a session holds on the server that the client can set
// again on another session: its namespace and database, its variables and the
// token it is authenticated with.
type sessionState struct {
	mu        sync.Mutex
	namespace string
	database  string
	vars      map[string]any
	token     string
}

// sessionSnapshot is a copy of a sessionState.
type sessionSnapshot struct {
	namespace string
	database  string
	vars      map[string]any
	token     string
}

func (s *sessionState) use(namespace, database string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.namespace = namespace
	s.database = database
}

func (s *sessionState) let(key string, val any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vars == nil {
		s.vars = make(map[string]any)
	}
	s.vars[key] = val
}

func (s *sessionState) unset(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vars, key)
}

func (s *sessionState) setToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
}

func (s *sessionState) setTokens(tokens *Tokens) {
	if tokens != nil {
		s.setToken(tokens.Access)
	}
}

func (s *sessionState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.namespace = ""
	s.database = ""
	s.vars = nil
	s.token = ""
}

// snapshot copies the state. The access token of a pair the renewer holds is
// newer than the one recorded when the pair was obtained.
func (s *sessionState) snapshot(renew *renewer) sessionSnapshot {
	s.mu.Lock()
	snap := sessionSnapshot{
		namespace: s.namespace,
		database:  s.database,
		vars:      maps.Clone(s.vars),
		token:     s.token,
	}
	s.mu.Unlock()

	if tokens := renew.current(); tokens != nil {
		snap.token = tokens.Access
	}
	return snap
}

// marshalerGetter is a connection which exposes the marshaler it encodes
// requests with.
type marshalerGetter interface {
	GetMarshaler() codec.Marshaler
}

// letValue returns the value for Let to send and record. Over a WebSocket
// connection which exposes its marshaler, that is the value encoded once, so
// that a later change to the value reaches neither the server nor a fork.
// Otherwise it is the value itself.
func letValue(con connection.Connection, val any) (any, error) {
	if _, ok := con.(connection.WebSocketConnection); !ok {
		return val, nil
	}
	m, ok := con.(marshalerGetter)
	if !ok || m.GetMarshaler() == nil {
		return val, nil
	}
	data, err := m.GetMarshaler().Marshal(val)
	if err != nil {
		return nil, err
	}
	return cbor.RawMessage(data), nil
}

// newSession returns a Session object for a session ID. tracked is whether
// the client knows the session's state, as it does for a session it attached.
func (db *DB) newSession(id models.UUID, tracked bool) *Session {
	s := &Session{db: db, id: &id, tracked: tracked}
	if db.opts.tokenRenewal {
		s.renew = newRenewer(db.opts.expiryMargin, func(ctx context.Context, tokens Tokens) (*Tokens, error) {
			return sessionCall[Tokens](ctx, s, connection.Refresh, tokens)
		})
	}
	return s
}

// registerSessionLocked registers a Session object with the client. The
// caller holds sessionsMu.
func (db *DB) registerSessionLocked(s *Session) {
	if db.sessions == nil {
		db.sessions = make(map[models.UUID]*Session)
	}
	db.sessions[*s.id] = s
}

func (db *DB) forgetSession(id models.UUID) {
	db.sessionsMu.Lock()
	defer db.sessionsMu.Unlock()
	delete(db.sessions, id)
}

// Sessions lists the sessions attached to the connection, other than its
// default one, as the Session objects that [DB.Attach] and [DB.Fork] return.
// It needs SurrealDB v3+ over WebSocket.
//
// A session attached on the connection without this client, such as with
// [connection.Call] on the connection itself, is listed too. The client does not know its namespace, database,
// variables or authentication, so it cannot be forked: [Session.Fork] returns
// [constants.ErrForkUntracked] for it.
func (db *DB) Sessions(ctx context.Context) ([]*Session, error) {
	if _, ok := db.con.(connection.WebSocketConnection); !ok {
		return nil, constants.ErrSessionsNotSupported
	}

	ids, err := send[[]models.UUID](ctx, db, string(connection.Sessions))
	if err != nil {
		return nil, err
	}
	if ids == nil {
		return nil, nil
	}

	// The lookups and inserts happen under one hold of the lock, so that a
	// session Attach registers meanwhile is not replaced.
	db.sessionsMu.Lock()
	defer db.sessionsMu.Unlock()
	sessions := make([]*Session, 0, len(*ids))
	for _, id := range *ids {
		s, ok := db.sessions[id]
		if !ok {
			// Attached on this connection without the client.
			s = db.newSession(id, false)
			db.registerSessionLocked(s)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Fork attaches a new session which starts as a copy of the default session:
// the same namespace and database, the same variables, and authenticated with
// the same access token. The two sessions are independent from then on.
//
// The new session is signed in with the access token, not with the session:
// the server cannot copy a session's authentication. A session signed in with
// a username and password outlives its token. By default a token lasts an
// hour and the session has no limit. Once the token has expired, Fork returns
// an error wrapping [constants.ErrForkTokenExpired] without attaching a
// session. Attach a session and sign it in yourself then.
//
// A token pair is not copied, since a refresh token can be used only once, so
// the new session does not renew its access token by itself.
//
// The variables are copied as Let encoded them. The built-in WebSocket
// connections expose their marshaler for this. Over another connection, Let
// keeps the value itself instead, so a value passed to Let must not be
// changed afterwards.
//
// Fork needs SurrealDB v3+ over WebSocket.
func (db *DB) Fork(ctx context.Context) (*Session, error) {
	return db.fork(ctx, db.state.snapshot(db.renew))
}

// Fork attaches a new session which starts as a copy of this one. See [DB.Fork].
// Like it, the new session is signed in with this session's access token, so
// Fork returns an error wrapping [constants.ErrForkTokenExpired] once that
// token has expired.
//
// Fork returns [constants.ErrForkUntracked] for a session that [DB.Sessions]
// lists but this client did not attach, as the client does not know its state.
func (s *Session) Fork(ctx context.Context) (*Session, error) {
	if s.isClosed() {
		return nil, constants.ErrSessionClosed
	}
	if !s.tracked {
		return nil, constants.ErrForkUntracked
	}
	return s.db.fork(ctx, s.state.snapshot(s.renew))
}

func (db *DB) fork(ctx context.Context, snap sessionSnapshot) (*Session, error) {
	if snap.token != "" {
		if exp, ok := tokenExpiry(snap.token); ok && !time.Now().Before(exp) {
			return nil, fmt.Errorf("%w at %s", constants.ErrForkTokenExpired, exp.Format(time.RFC3339))
		}
	}

	child, err := db.Attach(ctx)
	if err != nil {
		return nil, err
	}

	if err := child.restore(ctx, snap); err != nil {
		return nil, errors.Join(err, child.discard(ctx))
	}
	return child, nil
}

// restore sets the state of a snapshot on this session.
func (s *Session) restore(ctx context.Context, snap sessionSnapshot) error {
	if snap.namespace != "" || snap.database != "" {
		if err := s.Use(ctx, snap.namespace, snap.database); err != nil {
			return err
		}
	}
	for key, val := range snap.vars {
		if err := s.Let(ctx, key, val); err != nil {
			return err
		}
	}
	if snap.token != "" {
		if err := s.Authenticate(ctx, snap.token); err != nil {
			return err
		}
	}
	return nil
}

package surrealdb

import (
	"context"
	"maps"
	"sync"

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

// newSession returns the Session object for an attached session ID, and
// registers it with the client.
func (db *DB) newSession(id models.UUID) *Session {
	s := &Session{db: db, id: &id}
	if db.opts.tokenRenewal {
		s.renew = newRenewer(db.opts.expiryMargin, func(ctx context.Context, tokens Tokens) (*Tokens, error) {
			return sessionCall[Tokens](ctx, s, connection.Refresh, tokens)
		})
	}

	db.sessionsMu.Lock()
	defer db.sessionsMu.Unlock()
	if db.sessions == nil {
		db.sessions = make(map[models.UUID]*Session)
	}
	db.sessions[id] = s
	return s
}

func (db *DB) forgetSession(id models.UUID) {
	db.sessionsMu.Lock()
	defer db.sessionsMu.Unlock()
	delete(db.sessions, id)
}

// Sessions lists the sessions attached to the connection, other than its
// default one, as the Session objects that [DB.Attach] and [DB.Fork] return.
// It needs SurrealDB v3+ over WebSocket.
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

	sessions := make([]*Session, 0, len(*ids))
	for _, id := range *ids {
		db.sessionsMu.Lock()
		s, ok := db.sessions[id]
		db.sessionsMu.Unlock()
		if !ok {
			// Attached on this connection without the client, such as with Send.
			s = db.newSession(id)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Fork attaches a new session which starts as a copy of the default session:
// the same namespace and database, the same variables, and authenticated with
// the same access token. The two sessions are independent from then on.
//
// A token pair is not copied, since a refresh token can be used only once, so
// the new session does not renew its access token by itself.
//
// Fork needs SurrealDB v3+ over WebSocket.
func (db *DB) Fork(ctx context.Context) (*Session, error) {
	return db.fork(ctx, db.state.snapshot(db.renew))
}

// Fork attaches a new session which starts as a copy of this one. See [DB.Fork].
func (s *Session) Fork(ctx context.Context) (*Session, error) {
	if s.isClosed() {
		return nil, constants.ErrSessionClosed
	}
	return s.db.fork(ctx, s.state.snapshot(s.renew))
}

func (db *DB) fork(ctx context.Context, snap sessionSnapshot) (*Session, error) {
	child, err := db.Attach(ctx)
	if err != nil {
		return nil, err
	}

	if err := child.restore(ctx, snap); err != nil {
		_ = child.Detach(ctx)
		return nil, err
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

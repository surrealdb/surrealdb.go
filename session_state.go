package surrealdb

import (
	"context"
	"maps"
	"sync"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// sessionState is what a session holds on the server that the client can set
// again on a new connection: its namespace and database, its variables and the
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
// that a later change to the value reaches neither the server nor the variable
// the client sets again on a new connection. Otherwise it is the value itself.
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

// newSession returns a Session object for a session ID.
func (db *DB) newSession(id models.UUID) *Session {
	s := &Session{db: db, id: &id}
	if db.opts.tokenRenewal {
		s.renew = newRenewer(db.opts.expiryMargin, func(ctx context.Context, tokens Tokens) (*Tokens, error) {
			return sessionCall[Tokens](ctx, s, connection.Refresh, tokens)
		})
	}
	return s
}

// sessionRegistry holds the Session objects of a client, by ID.
type sessionRegistry struct {
	mu   sync.Mutex
	byID map[models.UUID]*Session
}

func (r *sessionRegistry) add(s *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = make(map[models.UUID]*Session)
	}
	r.byID[*s.id] = s
}

func (r *sessionRegistry) remove(id models.UUID) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
}

// all returns the registered sessions.
func (r *sessionRegistry) all() []*Session {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sessions := make([]*Session, 0, len(r.byID))
	for _, s := range r.byID {
		sessions = append(sessions, s)
	}
	return sessions
}

func (db *DB) forgetSession(id models.UUID) {
	db.sessions.remove(id)
}

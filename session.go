//nolint:dupl // Session methods intentionally mirror DB methods with similar structure
package surrealdb

import (
	"context"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Session represents an additional SurrealDB session on a WebSocket connection.
// Sessions scope live notifications and can have their own transactions.
//
// Sessions are only supported on WebSocket connections (SurrealDB v3+).
// Each session starts unauthenticated and without a selected namespace/database,
// so you must call SignIn/Authenticate and Use after creating a session.
//
// Session satisfies the sendable constraint, so all surrealdb.Query,
// surrealdb.Create, etc. functions work with sessions directly.
type Session struct {
	db     *DB
	id     *models.UUID
	closed bool
	mu     sync.RWMutex
}

// Attach creates a new session on the WebSocket connection.
// Sessions are only supported on WebSocket connections (SurrealDB v3+).
//
// The new session starts unauthenticated and without a selected namespace/database.
// You must call SignIn/Authenticate and Use on the session before making queries.
//
// Example:
//
//	session, err := db.Attach(ctx)
//	if err != nil {
//	    return err
//	}
//	defer session.Detach(ctx)
//
//	// Authenticate the session
//	_, err = session.SignIn(ctx, Auth{Username: "root", Password: "root"})
//	if err != nil {
//	    return err
//	}
//
//	// Select namespace and database
//	err = session.Use(ctx, "test", "test")
//	if err != nil {
//	    return err
//	}
//
//	// Now the session is ready for queries
//	results, err := surrealdb.Query[[]User](ctx, session, "SELECT * FROM users", nil)
func (db *DB) Attach(ctx context.Context) (*Session, error) {
	// Check if the connection is a WebSocket connection
	if _, ok := db.con.(connection.WebSocketConnection); !ok {
		return nil, constants.ErrSessionsNotSupported
	}

	// Generate a new UUID for the session
	newUUID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	sessionID := models.UUID{UUID: newUUID}

	// Send the attach RPC request with the session UUID as a top-level field
	req := &connection.RPCRequest{
		Method:  string(connection.Attach),
		Session: &sessionID,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(db.con, ctx, &res, req); err != nil {
		return nil, err
	}

	return &Session{
		db: db,
		id: &sessionID,
	}, nil
}

// ID returns the session's UUID.
func (s *Session) ID() *models.UUID {
	return s.id
}

// Detach deletes the session from the server.
// After calling Detach, the session cannot be used anymore.
func (s *Session) Detach(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return constants.ErrSessionClosed
	}

	// Send the detach RPC request
	req := &connection.RPCRequest{
		Method:  string(connection.Detach),
		Session: s.id,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return err
	}

	s.closed = true
	return nil
}

// Begin starts a new interactive transaction in this session.
// Interactive transactions are only supported on WebSocket connections (SurrealDB v3+).
func (s *Session) Begin(ctx context.Context) (*Transaction, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	// Send the begin RPC request with the session UUID
	req := &connection.RPCRequest{
		Method:  string(connection.Begin),
		Session: s.id,
	}

	var res connection.RPCResponse[models.UUID]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return nil, err
	}

	return &Transaction{
		db:        s.db,
		id:        res.Result,
		sessionID: s.id,
	}, nil
}

// SignUp signs up a new user in this session.
func (s *Session) SignUp(ctx context.Context, authData any) (string, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return "", constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.SignUp),
		Params:  []any{authData},
		Session: s.id,
	}

	var res connection.RPCResponse[string]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return "", err
	}

	if res.Result == nil {
		return "", nil
	}
	return *res.Result, nil
}

// SignUpWithRefresh signs up a new user using a TYPE RECORD access method with WITH REFRESH enabled.
func (s *Session) SignUpWithRefresh(ctx context.Context, authData any) (*Tokens, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.SignUp),
		Params:  []any{authData},
		Session: s.id,
	}

	var res connection.RPCResponse[Tokens]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return nil, err
	}

	return res.Result, nil
}

// SignIn signs in an existing user in this session.
func (s *Session) SignIn(ctx context.Context, authData any) (string, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return "", constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.SignIn),
		Params:  []any{authData},
		Session: s.id,
	}

	var res connection.RPCResponse[string]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return "", err
	}

	if res.Result == nil {
		return "", nil
	}
	return *res.Result, nil
}

// SignInWithRefresh signs in using a TYPE RECORD access method with WITH REFRESH enabled.
func (s *Session) SignInWithRefresh(ctx context.Context, authData any) (*Tokens, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.SignIn),
		Params:  []any{authData},
		Session: s.id,
	}

	var res connection.RPCResponse[Tokens]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return nil, err
	}

	return res.Result, nil
}

// Authenticate authenticates the session with the provided token.
func (s *Session) Authenticate(ctx context.Context, token string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.Authenticate),
		Params:  []any{token},
		Session: s.id,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return err
	}

	return nil
}

// Invalidate invalidates the authentication for this session.
func (s *Session) Invalidate(ctx context.Context) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.Invalidate),
		Session: s.id,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return err
	}

	return nil
}

// Use selects the namespace and database for this session.
func (s *Session) Use(ctx context.Context, ns, database string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.Use),
		Params:  []any{ns, database},
		Session: s.id,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return err
	}

	return nil
}

// Let sets a variable in this session.
func (s *Session) Let(ctx context.Context, key string, val any) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.Let),
		Params:  []any{key, val},
		Session: s.id,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return err
	}

	return nil
}

// Unset removes a variable from this session.
func (s *Session) Unset(ctx context.Context, key string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.Unset),
		Params:  []any{key},
		Session: s.id,
	}

	var res connection.RPCResponse[any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return err
	}

	return nil
}

// Info returns information about the current session state.
func (s *Session) Info(ctx context.Context) (map[string]any, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	req := &connection.RPCRequest{
		Method:  string(connection.Info),
		Session: s.id,
	}

	var res connection.RPCResponse[map[string]any]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return nil, err
	}

	if res.Result == nil {
		return nil, nil
	}
	return *res.Result, nil
}

// Version returns the SurrealDB version information.
func (s *Session) Version(ctx context.Context) (*VersionData, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, constants.ErrSessionClosed
	}
	s.mu.RUnlock()

	// Version doesn't need session context, but we include it for consistency
	return s.db.Version(ctx)
}

// LiveNotifications returns a channel for receiving live query notifications.
func (s *Session) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return s.db.con.LiveNotifications(liveQueryID)
}

// CloseLiveNotifications closes the notification channel for a live query.
func (s *Session) CloseLiveNotifications(liveQueryID string) error {
	return s.db.con.CloseLiveNotifications(liveQueryID)
}

// isClosed returns whether the session is closed (for internal use by send function).
func (s *Session) isClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

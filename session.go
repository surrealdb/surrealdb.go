//nolint:dupl // Session methods intentionally mirror DB methods with similar structure
package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	// renew keeps the token pair of this session fresh. It is nil when token
	// renewal is turned off.
	renew *renewer
	// state is what this session holds, for Fork.
	state sessionState
	// tracked is whether the client knows state, as it does for a session it
	// attached, unlike one that Sessions found.
	tracked bool
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

	// The session is registered before it is attached, so that Sessions,
	// which may list it as soon as the server has it, returns this object.
	s := db.newSession(sessionID, true)
	db.sessionsMu.Lock()
	db.registerSessionLocked(s)
	db.sessionsMu.Unlock()

	// Send the attach RPC request with the session UUID as a top-level field
	req := &connection.RPCRequest{
		Method:  string(connection.Attach),
		Session: &sessionID,
	}

	// Once the request is sent, the server may attach the session even if ctx
	// ends. So ending ctx does not stop the request at once: the client waits
	// up to sessionCleanupTimeout for the answer, and then detaches the session.
	callCtx, cancelCall := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelCall()
	stop := context.AfterFunc(ctx, func() {
		time.AfterFunc(sessionCleanupTimeout, cancelCall)
	})
	defer stop()

	var res connection.RPCResponse[any]
	err = connection.Call(db.con, callCtx, &res, req)
	if ctxErr := ctx.Err(); ctxErr != nil {
		if err == nil {
			return nil, errors.Join(ctxErr, s.discard(ctx))
		}
		db.forgetSession(sessionID)
		return nil, ctxErr
	}
	if err != nil {
		db.forgetSession(sessionID)
		return nil, err
	}

	return s, nil
}

// sessionCleanupTimeout bounds how long the client takes to detach a session
// it attached but does not return, after the context of the call has ended.
const sessionCleanupTimeout = 5 * time.Second

// discard detaches a session that is not returned to the caller, with a
// deadline of its own, as ctx may have ended. If the session cannot be
// detached, the client forgets it all the same, and Sessions lists it as a
// session the client did not attach.
func (s *Session) discard(ctx context.Context) error {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionCleanupTimeout)
	defer cancel()
	if err := s.Detach(dctx); err != nil {
		s.renew.stop()
		s.db.forgetSession(*s.id)
		return fmt.Errorf("detaching the session: %w", err)
	}
	return nil
}

// sessionCall sends an RPC request in this session.
func sessionCall[T any](ctx context.Context, s *Session, method connection.RPCFunction, params ...any) (*T, error) {
	if s.isClosed() {
		return nil, constants.ErrSessionClosed
	}

	req := &connection.RPCRequest{
		Method:  string(method),
		Params:  params,
		Session: s.id,
	}

	var res connection.RPCResponse[T]
	if err := connection.Call(s.db.con, ctx, &res, req); err != nil {
		return nil, err
	}
	return res.Result, nil
}

// ID returns the session's UUID.
func (s *Session) ID() *models.UUID {
	return s.id
}

// Detach deletes the session from the server.
// After calling Detach, the session cannot be used anymore.
func (s *Session) Detach(ctx context.Context) error {
	if err := s.detach(ctx); err != nil {
		return err
	}
	// The renewer is stopped once the session is unlocked, as stopping it
	// waits for a renewal under way, whose request checks the session.
	s.renew.stop()
	s.db.forgetSession(*s.id)
	return nil
}

func (s *Session) detach(ctx context.Context) error {
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
	if s.db.rc != nil {
		s.db.rc.lives.forgetSession(s.id, s.db.rc.current())
	}
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
	if err := s.renew.lockAuth(ctx); err != nil {
		return "", err
	}
	defer s.renew.unlockAuth()

	token, err := sessionCall[string](ctx, s, connection.SignUp, authData)
	if err != nil {
		return "", err
	}
	s.renew.clear()
	if token == nil {
		return "", nil
	}
	s.state.setToken(*token)
	return *token, nil
}

// SignUpWithRefresh signs up a new user using a TYPE RECORD access method with WITH REFRESH enabled.
func (s *Session) SignUpWithRefresh(ctx context.Context, authData any) (*Tokens, error) {
	if err := s.renew.lockAuth(ctx); err != nil {
		return nil, err
	}
	defer s.renew.unlockAuth()

	tokens, err := sessionCall[Tokens](ctx, s, connection.SignUp, authData)
	if err != nil {
		return nil, err
	}
	s.renew.set(tokens)
	s.state.setTokens(tokens)
	return tokens, nil
}

// SignIn signs in an existing user in this session.
func (s *Session) SignIn(ctx context.Context, authData any) (string, error) {
	if err := s.renew.lockAuth(ctx); err != nil {
		return "", err
	}
	defer s.renew.unlockAuth()

	token, err := sessionCall[string](ctx, s, connection.SignIn, authData)
	if err != nil {
		return "", err
	}
	s.renew.clear()
	if token == nil {
		return "", nil
	}
	s.state.setToken(*token)
	return *token, nil
}

// SignInWithRefresh signs in using a TYPE RECORD access method with WITH REFRESH enabled.
func (s *Session) SignInWithRefresh(ctx context.Context, authData any) (*Tokens, error) {
	if err := s.renew.lockAuth(ctx); err != nil {
		return nil, err
	}
	defer s.renew.unlockAuth()

	tokens, err := sessionCall[Tokens](ctx, s, connection.SignIn, authData)
	if err != nil {
		return nil, err
	}
	s.renew.set(tokens)
	s.state.setTokens(tokens)
	return tokens, nil
}

// Refresh exchanges a token pair for a new one and authenticates this session
// with it. See [DB.Refresh].
func (s *Session) Refresh(ctx context.Context, tokens Tokens) (*Tokens, error) {
	if err := s.renew.lockAuth(ctx); err != nil {
		return nil, err
	}
	defer s.renew.unlockAuth()

	next, err := sessionCall[Tokens](ctx, s, connection.Refresh, tokens)
	if err != nil {
		return nil, err
	}
	s.renew.set(next)
	s.state.setTokens(next)
	return next, nil
}

// Revoke revokes the refresh token of a token pair. See [DB.Revoke].
func (s *Session) Revoke(ctx context.Context, tokens Tokens) error {
	if err := s.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer s.renew.unlockAuth()

	if _, err := sessionCall[any](ctx, s, connection.Revoke, tokens); err != nil {
		return err
	}
	if current := s.renew.current(); current != nil && current.Refresh == tokens.Refresh {
		// The session stays signed in with the latest access token, which
		// may be newer than the one recorded when the pair was obtained.
		s.state.setToken(current.Access)
		s.renew.clear()
	}
	return nil
}

// Tokens returns the token pair this session holds. See [DB.Tokens].
func (s *Session) Tokens() *Tokens {
	return s.renew.current()
}

// Authenticate authenticates the session with the provided token.
func (s *Session) Authenticate(ctx context.Context, token string) error {
	if err := s.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer s.renew.unlockAuth()

	if _, err := sessionCall[any](ctx, s, connection.Authenticate, token); err != nil {
		return err
	}
	s.renew.clear()
	s.state.setToken(token)
	return nil
}

// Invalidate invalidates the authentication for this session.
func (s *Session) Invalidate(ctx context.Context) error {
	if err := s.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer s.renew.unlockAuth()

	if _, err := sessionCall[any](ctx, s, connection.Invalidate); err != nil {
		return err
	}
	s.renew.clear()
	s.state.setToken("")
	return nil
}

// Reset returns this session to the state it was in when it was attached:
// signed out, with no namespace, database or variables, and with its live
// queries killed.
func (s *Session) Reset(ctx context.Context) error {
	if err := s.renew.lockAuth(ctx); err != nil {
		return err
	}
	defer s.renew.unlockAuth()

	if _, err := sessionCall[any](ctx, s, connection.Reset); err != nil {
		return err
	}
	s.renew.clear()
	s.state.reset()
	// The server killed the session's live queries: stop restarting them.
	if s.db.rc != nil {
		s.db.rc.lives.forgetSession(s.id, s.db.rc.current())
	}
	return nil
}

// Use selects the namespace and database for this session.
func (s *Session) Use(ctx context.Context, ns, database string) error {
	if _, err := sessionCall[any](ctx, s, connection.Use, ns, database); err != nil {
		return err
	}
	s.state.use(ns, database)
	return nil
}

// Let sets a variable in this session. See [DB.Let].
func (s *Session) Let(ctx context.Context, key string, val any) error {
	val, err := letValue(s.db.con, val)
	if err != nil {
		return err
	}
	if _, err := sessionCall[any](ctx, s, connection.Let, key, val); err != nil {
		return err
	}
	s.state.let(key, val)
	return nil
}

// Unset removes a variable from this session.
func (s *Session) Unset(ctx context.Context, key string) error {
	if _, err := sessionCall[any](ctx, s, connection.Unset, key); err != nil {
		return err
	}
	s.state.unset(key)
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

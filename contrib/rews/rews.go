package rews

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

type State int

const (
	StateUnknown State = iota
	StateDisconnected
	StateConnecting
	StateConnected
	StateClosing
	StateClosed
)

func (state State) String() string {
	switch state {
	case StateUnknown:
		return "Unknown"
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateClosing:
		return "Closing"
	case StateClosed:
		return "Closed"
	default:
		return "InvalidState"
	}
}

func (s State) validateTransitionTo(newState State) error {
	switch s {
	case StateDisconnected:
		switch newState {
		case StateConnecting, StateDisconnected:
			return nil
		}
	case StateConnecting:
		switch newState {
		case StateConnected, StateDisconnected:
			return nil
		}
	case StateConnected:
		switch newState {
		// Connected to Connecting is possible when the connection is lost
		// after the connection is established.
		case StateConnecting, StateClosing, StateDisconnected:
			return nil
		}
	case StateClosing:
		if newState == StateClosed {
			return nil
		}
	}

	return fmt.Errorf("invalid state transition from %v to %v", s, newState)
}

type Connection[C connection.WebSocketConnection] struct {
	connection.WebSocketConnection

	// connMu protects access to the WebSocketConnection field during reconnection
	connMu sync.RWMutex

	// NewFunc is a function that initializes the WebSocket connection.
	// It is used to create a new WebSocket connection object when the initial
	// connection is made, or when the reconnection is needed.
	// The function should return a WebSocket connection and an error.
	NewFunc func(context.Context) (C, error)

	// CheckInterval is the interval at which the reconnection attempts are made.
	// It is used to avoid busy-waiting and to control the frequency of reconnection
	// attempts.
	//
	// Default is 5 seconds, to prepare for rare cases that
	// this struct is used directly by the SDK consumer.
	//
	// However, the expected usage is that the SDK itself
	// uses this struct only when the user configured CheckInterval
	// greater than 0.
	//
	// If the user doesn't set the option, the reconnection should be disabled
	// and this struct must not be used in the first place.
	CheckInterval time.Duration

	// connCloseCh signals that the connection is being closed
	connCloseCh chan int

	// reconnLoopCloseCh is used to signal that the reconnection loop is closed,
	// by closing the channel.
	//
	// This is used solely to ensure that the reconnection loop stops
	// before Close() returns.
	reconnLoopCloseCh chan int

	// logger is used to log the state transitions and errors.
	logger logger.Logger

	// once is used to ensure that the reconnection loop is started only once.
	// This is to prevent multiple reconnection loops from being started
	// on second and subsequent Connect() calls for reconnection.
	once sync.Once

	// state is the current state of the connection.
	// It is used to track the state of the connection and to ensure that
	// the state transitions are valid.
	state State

	// stateMu is used to protect the state transitions and checks.
	// It is a mutex to ensure that the state transitions are atomic
	// and that the state checks are consistent.
	stateMu sync.Mutex

	// sessionVars is a map that holds the session variables.
	// It is used to store the session variables that are set by the user
	// and to restore them on reconnection.
	sessionVars map[string]any

	// sessionToken is the token used for authentication.
	// It is used to store the token that is returned by a successful SignIn or SignUp, or
	// used by a successful Authenticate call.
	// It is used to re-authenticate on reconnection.
	sessionToken string

	// sessionNS and sessionDB are the namespace and database used for the session.
	// They are used to restore the namespace and database on reconnection.
	sessionNS string
	sessionDB string

	// sessionRefresh is the refresh token returned by a successful SignInWithRefresh
	// or SignUpWithRefresh, used to obtain a new access token on reconnection when
	// the stored one has expired.
	sessionRefresh string

	// sessionAccess holds the namespace, database and access method the refresh
	// token was issued for, as a refresh signin must name them again.
	sessionAccess map[string]any

	// sessionLock protects the session fields above, which the reconnection
	// loop reads while the caller's goroutines change them. It is a semaphore
	// rather than a sync.Mutex so that waiting for it honors the caller's
	// context: see lockSession.
	//
	// It is held across each session-changing RPC (Use, Let, Unset, SignIn,
	// SignUp, Authenticate, Invalidate and the refresh variants) together with
	// the update of the fields, and across the whole replay of the session onto
	// a new socket. That serializes the two: a replay never applies a state
	// older than one a caller has already been told is in effect, and a session
	// change made while a replay is running waits for it and then applies on
	// top of the restored session.
	//
	// The replay takes it only once the new socket is in place, so a session
	// RPC holding it is never waiting for a socket that only the replay would
	// provide; an RPC still in flight on the dropped socket is given up once
	// that socket is seen to have closed (see lockSessionSocket), if it has
	// not failed or timed out before, and the replay proceeds after it.
	sessionLock     chan struct{}
	sessionLockInit sync.Once

	// reliableLQ contains all the state and functionality for reliable live query management
	reliableLQ *reliableLQ

	// unmarshaler is used to unmarshal CBOR data
	unmarshaler codec.Unmarshaler

	// Retryer defines the retry behavior for connection attempts.
	// If nil, connection attempts will not be retried.
	Retryer Retryer
}

var _ connection.Connection = (*Connection[connection.WebSocketConnection])(nil)
var _ connection.WebSocketConnection = (*Connection[connection.WebSocketConnection])(nil)

// New creates a new auto-reconnecting WebSocket connection.
//
// It takes a function that establishes the WebSocket connection,
// a check interval for reconnection attempts, an unmarshaler for CBOR data, and a logger.
func New[C connection.WebSocketConnection](
	newConn func(context.Context) (C, error),
	checkInterval time.Duration,
	unmarshaler codec.Unmarshaler,
	log logger.Logger,
) *Connection[C] {
	c := &Connection[C]{
		CheckInterval: checkInterval,
		NewFunc:       newConn,
		state:         StateDisconnected,
		logger:        log,
		sessionVars:   make(map[string]any),
		unmarshaler:   unmarshaler,
	}

	// Initialize reliableLQ with the unmarshaler
	c.reliableLQ = newReliableLQ(log, unmarshaler)

	return c
}

func (arws *Connection[C]) transitionTo(newState State) error {
	arws.stateMu.Lock()
	defer arws.stateMu.Unlock()

	if err := arws.state.validateTransitionTo(newState); err != nil {
		return err
	}

	arws.state = newState
	if arws.logger != nil {
		arws.logger.Debug("rews.Connection state transitioned", "new_state", newState)
	}

	return nil
}

// IsClosed returns true if this reconnecting WebSocket connection is closed.
// Once closed, it cannot be used to establish a new connection.
func (arws *Connection[C]) IsClosed() bool {
	arws.stateMu.Lock()
	defer arws.stateMu.Unlock()

	return arws.state == StateClosed
}

// Connect establishes the WebSocket connection and starts the reconnection loop.
//
// Once the initial connection is successful,
// the reconnection loop will be started automatically,
// which will attempt to reconnect if the connection is lost.
//
// It returns an error if the initial connection fails.
// The caller is responsible for handling the error and deciding what to do next,
// such as retrying the initial connection, log and exit, etc.
//
// The intention for this behavior is to provide flexibility to the SDK consumer,
// because the initial connection failure is often due to misconfiguration,
// such as wrong URL, authentication failure, etc, which cannot be fixed automatically
// by retrying the connection.
//
// For example, the application consuming the SDK is running under a process manager
// or a container orchestrator, error-exiting the process or container
// could be a valid way to handle the initial connection failure.
// You might configure the manager or orchestrator to detect crash-looping applications
// and alert the operator to fix the misconfiguration.
//
// If Retryer is set, connection attempts will be retried
// according to the configured strategy (e.g., exponential backoff).
func (arws *Connection[C]) Connect(ctx context.Context) error {
	return arws.connectWithRetry(ctx, false)
}

// connectWithRetry handles the connection logic with optional retry
//
//nolint:gocyclo // Retry logic requires handling multiple conditions
func (arws *Connection[C]) connectWithRetry(ctx context.Context, isReconnect bool) error {
	if err := arws.transitionTo(StateConnecting); err != nil {
		return err
	}

	// Use the configured retryer if available
	retryer := arws.Retryer

	attempt := 0
	for {
		var err error

		// Try to create and connect
		conn, err := arws.NewFunc(ctx)
		if err == nil {
			err = conn.Connect(ctx)
			if err == nil {
				// Success! Update the connection while holding the lock
				arws.connMu.Lock()
				arws.WebSocketConnection = conn
				arws.connMu.Unlock()

				// Start reconnection loop if this is the first successful connection
				arws.once.Do(func() {
					if arws.logger != nil {
						arws.logger.Debug("rews.Connection is starting reconnection loop")
					}
					arws.connCloseCh = make(chan int, 1)
					arws.reconnLoopCloseCh = make(chan int, 1)
					go arws.reconnectionLoop()
				})

				if stateErr := arws.transitionTo(StateConnected); stateErr != nil {
					panic(fmt.Sprintf("BUG: rews.Connection failed to transition to connected state: %v", stateErr))
				}

				// Reset retryer if it has state
				if retryer != nil {
					retryer.Reset()
				}

				return nil
			}
		}

		// Connection failed - log the error
		if err != nil && arws.logger != nil {
			if isReconnect {
				arws.logger.Error("rews.Connection failed to reconnect", "error", err, "attempt", attempt+1)
			} else {
				arws.logger.Error("rews.Connection failed to connect", "error", err, "attempt", attempt+1)
			}
		}

		// Check if we should retry
		if retryer == nil {
			// No retryer - fail immediately
			if stateErr := arws.transitionTo(StateDisconnected); stateErr != nil && arws.logger != nil {
				arws.logger.Error("BUG: rews.Connection failed to transition to disconnected state", "error", stateErr)
			}
			return fmt.Errorf("rews.Connection failed to connect: %w", err)
		}

		// Get next retry delay
		delay, shouldRetry := retryer.NextDelay(attempt, err)
		if !shouldRetry {
			// Max retries exceeded or retryer says to stop
			if stateErr := arws.transitionTo(StateDisconnected); stateErr != nil && arws.logger != nil {
				arws.logger.Error("BUG: rews.Connection failed to transition to disconnected state", "error", stateErr)
			}
			return fmt.Errorf("rews.Connection failed to connect after %d attempts: %w", attempt+1, err)
		}

		// Wait before retrying
		if arws.logger != nil {
			arws.logger.Debug("rews.Connection will retry connection", "delay", delay, "attempt", attempt+1)
		}
		select {
		case <-ctx.Done():
			if stateErr := arws.transitionTo(StateDisconnected); stateErr != nil && arws.logger != nil {
				arws.logger.Error("BUG: rews.Connection failed to transition to disconnected state", "error", stateErr)
			}
			return fmt.Errorf("rews.Connection connection canceled: %w", ctx.Err())
		case <-time.After(delay):
			attempt++
		}
	}
}

// reconnect attempts to reconnect the WebSocket connection.
//
// This enhances the Connect method by re-authenticating with the token in the following cases:
// - SignUp succeeded before the reconnection (SignUp returns a token and it can be used for re-authentication).
// - SignIn succeeded before the reconnection (SignIn returns a token and it can be used for re-authentication).
// - Authenticate succeeded before the reconnection (Authenticate accepts the token and it can be used for re-authentication)
//
// The token might be expired or invalid which could result in an re-authentication failure.
// But rews does not handle the re-authentication failure.
// It is the caller's responsibility to handle the re-authentication failure.
//
//nolint:gocyclo // Session restoration requires handling multiple state conditions
func (arws *Connection[C]) reconnect(ctx context.Context) error {
	if err := arws.connectWithRetry(ctx, true); err != nil {
		return fmt.Errorf("rews.Connection failed to reconnect: %w", err)
	}

	// Hold the session lock for the whole replay, so that a session change
	// cannot complete between reading the state below and applying it, and
	// give the replay up if the new socket closes during it, rather than hold
	// the lock waiting for responses that will never come.
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return err
	}
	defer release()

	ns, db := arws.sessionNS, arws.sessionDB
	token, refresh := arws.sessionToken, arws.sessionRefresh
	access := arws.sessionAccess
	vars := arws.sessionVars

	if ns != "" && db != "" {
		if arws.logger != nil {
			arws.logger.Debug("rews.Connection is restoring namespace and database", "namespace", ns, "database", db)
		}
		if err := conn.Use(ctx, ns, db); err != nil {
			if arws.logger != nil {
				arws.logger.Error("rews.Connection failed to restore namespace and database", "error", err)
			}
			return fmt.Errorf("rews.Connection failed to restore namespace and database: %w", err)
		}
		if arws.logger != nil {
			arws.logger.Debug("rews.Connection restored namespace and database successfully")
		}
	}

	if err := arws.restoreAuth(ctx, conn, token, refresh, access); err != nil {
		return err
	}

	for key, value := range vars {
		if arws.logger != nil {
			arws.logger.Debug("rews.Connection is restoring session variable", "key", key, "value", value)
		}
		if err := conn.Let(ctx, key, value); err != nil {
			if arws.logger != nil {
				arws.logger.Error("rews.Connection failed to restore session variable", "key", key, "error", err)
			}
			return fmt.Errorf("rews.Connection failed to restore session variable %s: %w", key, err)
		}
	}

	if err := arws.reliableLQ.restoreLiveQueries(ctx, conn, conn, arws.logger); err != nil {
		if arws.logger != nil {
			arws.logger.Error("rews.Connection failed to restore live queries", "error", err)
		}
		return fmt.Errorf("rews.Connection failed to restore live queries: %w", err)
	}

	return nil
}

// restoreAuth authenticates a new connection as the old one was. When the access
// token is refused, as it is once it has expired, and a refresh token was kept,
// it signs in with the refresh token instead and keeps the tokens that returns.
//
// The caller must hold sessionLock.
func (arws *Connection[C]) restoreAuth(
	ctx context.Context, conn connection.WebSocketConnection, token, refresh string, access map[string]any,
) error {
	if token == "" {
		return nil
	}

	if arws.logger != nil {
		arws.logger.Debug("rews.Connection is re-authenticating with the session token")
	}
	err := conn.Authenticate(ctx, token)
	if err == nil {
		if arws.logger != nil {
			arws.logger.Debug("rews.Connection re-authenticated successfully with the session token")
		}
		return nil
	}

	if refresh == "" {
		if arws.logger != nil {
			arws.logger.Error("rews.Connection failed to re-authenticate with the session token", "error", err)
		}
		return fmt.Errorf("rews.Connection failed to re-authenticate with the session token: %w", err)
	}

	if arws.logger != nil {
		arws.logger.Debug("rews.Connection is refreshing the session token", "error", err)
	}
	params := copyMap(access)
	if params == nil {
		params = map[string]any{}
	}
	params["refresh"] = refresh
	tokens, refreshErr := conn.SignInWithRefresh(ctx, params)
	if refreshErr != nil {
		if arws.logger != nil {
			arws.logger.Error("rews.Connection failed to refresh the session token", "error", refreshErr)
		}
		return fmt.Errorf("rews.Connection failed to re-authenticate with the session token (%w) or refresh it: %w", err, refreshErr)
	}

	// The caller holds sessionLock, so the session cannot have changed since.
	arws.sessionToken = tokens.Access
	arws.sessionRefresh = tokens.Refresh

	return nil
}

// lockSession acquires sessionLock, or returns the context's error if it is
// done first, so that a caller whose deadline passes while another session
// change or a replay holds the lock is not kept waiting for it.
func (arws *Connection[C]) lockSession(ctx context.Context) error {
	arws.sessionLockInit.Do(func() { arws.sessionLock = make(chan struct{}, 1) })
	select {
	case arws.sessionLock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("rews.Connection gave up waiting for the session state: %w", ctx.Err())
	}
}

// unlockSession releases sessionLock, acquired by lockSession.
func (arws *Connection[C]) unlockSession() {
	<-arws.sessionLock
}

// errSocketClosed is the cause of the context of an RPC given up because the
// socket it was sent on closed.
var errSocketClosed = errors.New("rews.Connection: the socket the request was sent on closed")

// checkInterval returns how often to check whether the socket has closed.
func (arws *Connection[C]) checkInterval() time.Duration {
	if arws.CheckInterval > 0 {
		return arws.CheckInterval
	}
	return 5 * time.Second
}

// lockSessionSocket acquires sessionLock and returns the socket in use, with
// a context for RPCs on it that is also canceled once that socket closes, and
// a function that releases both.
//
// A response to a request on a socket that has closed never comes, and some
// sockets, such as gorillaws with no Timeout, wait for it for as long as the
// context allows. Without this, a session change sent with a context that
// never ends just before the socket drops would hold sessionLock for ever, and
// the reconnection replay, which needs it, would never run.
func (arws *Connection[C]) lockSessionSocket(ctx context.Context) (
	context.Context, connection.WebSocketConnection, func(), error,
) {
	if err := arws.lockSession(ctx); err != nil {
		return nil, nil, nil, err
	}

	arws.connMu.RLock()
	conn := arws.WebSocketConnection
	arws.connMu.RUnlock()

	ctx, stop := arws.untilSocketCloses(ctx, conn)
	return ctx, conn, func() {
		stop()
		arws.unlockSession()
	}, nil
}

// untilSocketCloses returns a context that is canceled with errSocketClosed
// once conn closes, checked every checkInterval, and a function to stop
// watching.
func (arws *Connection[C]) untilSocketCloses(
	parent context.Context, conn connection.WebSocketConnection,
) (ctx context.Context, stop func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(arws.checkInterval())
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if conn.IsClosed() {
					cancel(errSocketClosed)
					return
				}
			}
		}
	}()
	return ctx, func() {
		close(done)
		cancel(nil)
	}
}

// socketErr adds to err, returned by an RPC with a context from
// untilSocketCloses, that the socket closed, if that is why it failed.
func socketErr(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); errors.Is(cause, errSocketClosed) && !errors.Is(err, errSocketClosed) {
		return fmt.Errorf("%w: %w", cause, err)
	}
	return err
}

// setToken records the tokens of the session. The caller must hold sessionLock.
func (arws *Connection[C]) setToken(token, refresh string, access map[string]any) {
	arws.sessionToken = token
	arws.sessionRefresh = refresh
	arws.sessionAccess = access
}

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// accessKeys are the keys of signin data naming where a user signs in, which a
// refresh signin must repeat.
var accessKeys = map[string]bool{
	"NS": true, "NAMESPACE": true,
	"DB": true, "DATABASE": true,
	"AC": true, "ACCESS": true,
}

// accessOf extracts the namespace, database and access method from the
// authentication data of a signin or signup, which a refresh signin must repeat.
//
// authData may be any map with string keys, including named map types such as
// `type Credentials map[string]any`, or a struct (or pointer to one) whose
// fields are tagged NS, DB and AC, such as surrealdb.Auth, including fields
// promoted from embedded structs.
func accessOf(authData any) map[string]any {
	access := map[string]any{}
	v := reflect.ValueOf(authData)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return access
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return access
		}
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			if accessKeys[strings.ToUpper(k)] {
				access[k] = iter.Value().Interface()
			}
		}
	case reflect.Struct:
		structAccess(v, access)
	}
	return access
}

// structAccess adds the access fields of the struct v to access, finding
// them as encoding/json and the CBOR encoders find the fields they encode:
//
//   - fields of embedded structs without a tag name are promoted, level by
//     level, and a field hides any of the same name at a deeper level;
//   - of fields of the same name at the same level, a tagged one beats
//     untagged ones, and two or more of the same kind cancel out, so that
//     none of them is encoded.
func structAccess(v reflect.Value, access map[string]any) {
	hidden := map[string]bool{}
	visited := map[reflect.Type]bool{}
	level := []embedding{{typ: v.Type()}}
	for len(level) > 0 {
		count := map[reflect.Type]int{}
		for _, e := range level {
			count[e.typ]++
		}

		var next []embedding
		names := map[string]bool{}
		candidates := map[string][]accessField{}
		for _, e := range level {
			if visited[e.typ] {
				continue
			}
			visited[e.typ] = true
			next = append(next, levelFields(e, count[e.typ], hidden, names, candidates)...)
		}

		for name, fields := range candidates {
			if f, ok := dominantField(fields); ok {
				if fv, err := v.FieldByIndexErr(f.index); err == nil {
					if s, ok := stringValue(fv); ok && s != "" {
						access[name] = s
					}
				}
			}
		}
		for name := range names {
			hidden[name] = true
		}
		level = next
	}
}

// embedding is a struct type found at some level of a struct, with the index
// path to it.
type embedding struct {
	typ   reflect.Type
	index []int
}

// accessField is a field with an access name, with the index path to it and
// whether a tag gave it its name.
type accessField struct {
	index  []int
	tagged bool
}

// levelFields records the names of the fields of the struct e in names and
// the access fields not hidden by a shallower level in candidates, as many
// times as the struct occurs at this level, and returns the structs it embeds.
func levelFields(e embedding, times int, hidden, names map[string]bool, candidates map[string][]accessField) []embedding {
	var embedded []embedding
	for i := 0; i < e.typ.NumField(); i++ {
		f := e.typ.Field(i)
		index := append(append([]int(nil), e.index...), i)
		name := fieldName(f.Tag)
		if f.Anonymous && name == "" {
			t := f.Type
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			if t.Kind() == reflect.Struct {
				embedded = append(embedded, embedding{typ: t, index: index})
				continue
			}
		}
		if !f.IsExported() || name == "-" {
			continue
		}
		tagged := name != ""
		if !tagged {
			name = f.Name
		}
		names[name] = true
		if hidden[name] || !accessKeys[strings.ToUpper(name)] {
			continue
		}
		for n := 0; n < min(times, 2); n++ {
			candidates[name] = append(candidates[name], accessField{index: index, tagged: tagged})
		}
	}
	return embedded
}

// dominantField returns the one of fields of the same name at the same level
// that is encoded: the only tagged one, or else the only one; none if two or
// more tie.
func dominantField(fields []accessField) (accessField, bool) {
	var tagged []accessField
	for _, f := range fields {
		if f.tagged {
			tagged = append(tagged, f)
		}
	}
	switch {
	case len(tagged) == 1:
		return tagged[0], true
	case len(tagged) == 0 && len(fields) == 1:
		return fields[0], true
	default:
		return accessField{}, false
	}
}

// stringValue returns the string a field holds, through any pointers and
// interfaces, as the encoder sends it, and whether it holds one.
func stringValue(fv reflect.Value) (string, bool) {
	for fv.Kind() == reflect.Pointer || fv.Kind() == reflect.Interface {
		if fv.IsNil() {
			return "", false
		}
		fv = fv.Elem()
	}
	if fv.Kind() != reflect.String {
		return "", false
	}
	return fv.String(), true
}

// fieldName returns the name a struct field's tag gives it for encoding, or
// "" if it gives none. As the CBOR encoders do, it reads the cbor tag, and the
// json tag only when there is no cbor tag.
func fieldName(tag reflect.StructTag) string {
	t := tag.Get("cbor")
	if t == "" {
		t = tag.Get("json")
	}
	return strings.Split(t, ",")[0]
}

func (arws *Connection[C]) Use(ctx context.Context, namespace, database string) error {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return err
	}
	defer release()

	if err := conn.Use(ctx, namespace, database); err != nil {
		return fmt.Errorf("rews.Connection failed to use namespace and database: %w", socketErr(ctx, err))
	}
	arws.sessionNS = namespace
	arws.sessionDB = database
	return nil
}

func (arws *Connection[C]) Authenticate(ctx context.Context, token string) error {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return err
	}
	defer release()

	if err := conn.Authenticate(ctx, token); err != nil {
		return fmt.Errorf("rews.Connection failed to authenticate: %w", socketErr(ctx, err))
	}

	arws.setToken(token, "", nil)

	return nil
}

func (arws *Connection[C]) Let(ctx context.Context, key string, value any) error {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return err
	}
	defer release()

	if err := conn.Let(ctx, key, value); err != nil {
		return fmt.Errorf("rews.Connection failed to set session variable %s: %w", key, socketErr(ctx, err))
	}
	arws.sessionVars[key] = value
	return nil
}

func (arws *Connection[C]) Unset(ctx context.Context, key string) error {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return err
	}
	defer release()

	if err := conn.Unset(ctx, key); err != nil {
		return fmt.Errorf("rews.Connection failed to unset session variable %s: %w", key, socketErr(ctx, err))
	}
	delete(arws.sessionVars, key)
	return nil
}

func (arws *Connection[C]) SignUp(ctx context.Context, authData any) (string, error) {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return "", err
	}
	defer release()

	token, err := conn.SignUp(ctx, authData)
	if err != nil {
		return "", fmt.Errorf("rews.Connection failed to sign up: %w", socketErr(ctx, err))
	}

	arws.setToken(token, "", nil)
	return token, nil
}

func (arws *Connection[C]) SignIn(ctx context.Context, authData any) (string, error) {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return "", err
	}
	defer release()

	token, err := conn.SignIn(ctx, authData)
	if err != nil {
		return "", fmt.Errorf("rews.Connection failed to sign in: %w", socketErr(ctx, err))
	}

	arws.setToken(token, "", nil)
	return token, nil
}

// SignInWithRefresh signs in with a TYPE RECORD access method WITH REFRESH and
// keeps both tokens, so that a reconnection can re-authenticate, refreshing the
// access token if it has expired by then.
func (arws *Connection[C]) SignInWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	return arws.signWithRefresh(ctx, authData, "sign in", connection.WebSocketConnection.SignInWithRefresh)
}

// SignUpWithRefresh signs up with a TYPE RECORD access method WITH REFRESH and
// keeps both tokens, as SignInWithRefresh does.
func (arws *Connection[C]) SignUpWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	return arws.signWithRefresh(ctx, authData, "sign up", connection.WebSocketConnection.SignUpWithRefresh)
}

// signWithRefresh runs a signin or signup that returns a token pair and keeps
// the pair, with the access fields of authData a refresh signin must repeat.
func (arws *Connection[C]) signWithRefresh(
	ctx context.Context, authData any, verb string,
	rpc func(connection.WebSocketConnection, context.Context, any) (*connection.Tokens, error),
) (*connection.Tokens, error) {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	tokens, err := rpc(conn, ctx, authData)
	if err != nil {
		return nil, fmt.Errorf("rews.Connection failed to %s: %w", verb, socketErr(ctx, err))
	}

	arws.setToken(tokens.Access, tokens.Refresh, accessOf(authData))
	return tokens, nil
}

// Invalidate signs the session out and forgets its tokens, so that a
// reconnection does not authenticate the session again.
func (arws *Connection[C]) Invalidate(ctx context.Context) error {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return err
	}
	defer release()

	if err := conn.Invalidate(ctx); err != nil {
		return fmt.Errorf("rews.Connection failed to invalidate: %w", socketErr(ctx, err))
	}

	arws.setToken("", "", nil)
	return nil
}

// Close stops the reconnection loop and closes the WebSocket connection.
//
// You should call this method only when the application is shutting down,
// and the connection is not needed anymore.
//
// Once this function returns, the reconnection loop is guaranteed to stop.
// However, the WebSocket connection or TCP connection is not guaranteed to be closed immediately.
//
// Although it may potentially leak resources on the caller and the SurrealDB server side,
// it should be better than blocking on potentially never ending connection close operation,
// because those leaked resources can be eventually cleaned up by the operation system
// once the application process exits.
func (arws *Connection[C]) Close(ctx context.Context) error {
	if err := arws.transitionTo(StateClosing); err != nil {
		return fmt.Errorf("rews.Connection is already closing or closed: %w", err)
	}

	defer func() {
		if err := arws.transitionTo(StateClosed); err != nil && arws.logger != nil {
			arws.logger.Error("BUG: rews.Connection failed to transition to closed state", "error", err)
		}
	}()

	// Ensure the reconnection loop stops first,
	// so that it doesn't try to reconnect after the this reconnecting connection is closed.
	//
	// This implies a possible edge case where the reconnection loop
	// stops even though Close failed.
	//
	// But we accept this trade-off for simplicity,
	// assuming that the user would call rews.Connection.Close()
	// only when the connection is absolutely not needed anymore,
	// like the app is gracefully shutting down before exiting the program.
	close(arws.connCloseCh)
	<-arws.reconnLoopCloseCh

	arws.connMu.RLock()
	conn := arws.WebSocketConnection
	arws.connMu.RUnlock()
	if err := conn.Close(ctx); err != nil {
		return err
	}

	return nil
}

func (arws *Connection[C]) reconnectionLoop() {
	checkInterval := arws.checkInterval()

	defer func() {
		close(arws.reconnLoopCloseCh)
	}()

	for {
		if arws.logger != nil {
			arws.logger.Debug("rews.Connection is waiting for reconnection check interval", "interval", checkInterval)
		}
		select {
		case <-arws.connCloseCh:
			return
		case <-time.After(checkInterval):
		}

		arws.connMu.RLock()
		isClosed := arws.WebSocketConnection != nil && arws.WebSocketConnection.IsClosed()
		arws.connMu.RUnlock()
		if isClosed {
			if arws.logger != nil {
				arws.logger.Info("rews.Connection detected closed connection, attempting to reconnect")
			}

			// Use Retryer if configured, otherwise single attempt
			if err := arws.reconnect(context.Background()); err != nil {
				if arws.logger != nil {
					arws.logger.Error("rews.Connection reconnection attempt failed", "error", err)
				}
				// If we have a retryer, it already handled retries
				// Continue to next check interval
				continue
			}
		}
	}
}

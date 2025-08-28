package rews

import (
	"context"
	"fmt"
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

	// reliableLQ contains all the state and functionality for reliable live query management
	reliableLQ *reliableLQ

	// unmarshaler is used to unmarshal CBOR data
	unmarshaler codec.Unmarshaler
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
	arws.logger.Debug("rews.Connection state transitioned", "new_state", newState)

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
func (arws *Connection[C]) Connect(ctx context.Context) error {
	if err := arws.transitionTo(StateConnecting); err != nil {
		return err
	}

	var err error

	conn, err := arws.NewFunc(ctx)
	if err != nil {
		if stateErr := arws.transitionTo(StateDisconnected); stateErr != nil {
			arws.logger.Error("BUG: rews.Connection failed to transition to disconnected state", "error", stateErr)
		}
		return fmt.Errorf("rews.Connection failed to create a new connection: %w", err)
	}

	err = conn.Connect(ctx)
	if err != nil {
		if stateErr := arws.transitionTo(StateDisconnected); stateErr != nil {
			arws.logger.Error("BUG: rews.Connection failed to transition to disconnected state", "error", stateErr)
		}
		return fmt.Errorf("rews.Connection failed to connect: %w", err)
	}

	arws.WebSocketConnection = conn

	arws.once.Do(func() {
		arws.logger.Debug("rews.Connection is starting reconnection loop")

		arws.connCloseCh = make(chan int, 1)
		arws.reconnLoopCloseCh = make(chan int, 1)

		go arws.reconnectionLoop()
	})

	if err := arws.transitionTo(StateConnected); err != nil {
		panic(fmt.Sprintf("BUG: rews.Connection failed to transition to connected state: %v", err))
	}

	return nil
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
func (arws *Connection[C]) reconnect(ctx context.Context) error {
	if err := arws.Connect(ctx); err != nil {
		arws.logger.Error("rews.Connection failed to reconnect", "error", err)
		return fmt.Errorf("rews.Connection failed to reconnect: %w", err)
	}

	if arws.sessionNS != "" && arws.sessionDB != "" {
		arws.logger.Debug("rews.Connection is restoring namespace and database", "namespace", arws.sessionNS, "database", arws.sessionDB)
		if err := arws.Use(ctx, arws.sessionNS, arws.sessionDB); err != nil {
			arws.logger.Error("rews.Connection failed to restore namespace and database", "error", err)
			return fmt.Errorf("rews.Connection failed to restore namespace and database: %w", err)
		}
		arws.logger.Debug("rews.Connection restored namespace and database successfully")
	}

	if arws.sessionToken != "" {
		arws.logger.Debug("rews.Connection is re-authenticating with the session token")
		if err := arws.Authenticate(ctx, arws.sessionToken); err != nil {
			arws.logger.Error("rews.Connection failed to re-authenticate with the session token", "error", err)
			return fmt.Errorf("rews.Connection failed to re-authenticate with the session token: %w", err)
		}
		arws.logger.Debug("rews.Connection re-authenticated successfully with the session token")
	}

	for key, value := range arws.sessionVars {
		arws.logger.Debug("rews.Connection is restoring session variable", "key", key, "value", value)
		if err := arws.Let(ctx, key, value); err != nil {
			arws.logger.Error("rews.Connection failed to restore session variable", "key", key, "error", err)
			return fmt.Errorf("rews.Connection failed to restore session variable %s: %w", key, err)
		}
	}

	// Restore live queries after session state is restored
	// This will also setup notification routing for each restored query
	if err := arws.reliableLQ.restoreLiveQueries(ctx, arws.WebSocketConnection, arws.WebSocketConnection, arws.logger); err != nil {
		arws.logger.Error("rews.Connection failed to restore live queries", "error", err)
		return fmt.Errorf("rews.Connection failed to restore live queries: %w", err)
	}

	return nil
}

func (arws *Connection[C]) Use(ctx context.Context, namespace, database string) error {
	if err := arws.WebSocketConnection.Use(ctx, namespace, database); err != nil {
		return fmt.Errorf("rews.Connection failed to use namespace and database: %w", err)
	}
	arws.sessionNS = namespace
	arws.sessionDB = database
	return nil
}

func (arws *Connection[C]) Authenticate(ctx context.Context, token string) error {
	if err := arws.WebSocketConnection.Authenticate(ctx, token); err != nil {
		return fmt.Errorf("rews.Connection failed to authenticate: %w", err)
	}

	arws.sessionToken = token

	return nil
}

func (arws *Connection[C]) Let(ctx context.Context, key string, value any) error {
	if err := arws.WebSocketConnection.Let(ctx, key, value); err != nil {
		return fmt.Errorf("rews.Connection failed to set session variable %s: %w", key, err)
	}
	arws.sessionVars[key] = value
	return nil
}

func (arws *Connection[C]) Unset(ctx context.Context, key string) error {
	if err := arws.WebSocketConnection.Unset(ctx, key); err != nil {
		return fmt.Errorf("rews.Connection failed to unset session variable %s: %w", key, err)
	}
	delete(arws.sessionVars, key)
	return nil
}

func (arws *Connection[C]) SignUp(ctx context.Context, authData any) (string, error) {
	token, err := arws.WebSocketConnection.SignUp(ctx, authData)
	if err != nil {
		return "", fmt.Errorf("rews.Connection failed to sign up: %w", err)
	}

	arws.sessionToken = token
	return token, nil
}

func (arws *Connection[C]) SignIn(ctx context.Context, authData any) (string, error) {
	token, err := arws.WebSocketConnection.SignIn(ctx, authData)
	if err != nil {
		return "", fmt.Errorf("rews.Connection failed to sign in: %w", err)
	}

	arws.sessionToken = token
	return token, nil
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
		if err := arws.transitionTo(StateClosed); err != nil {
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

	if err := arws.WebSocketConnection.Close(ctx); err != nil {
		return err
	}

	return nil
}

func (arws *Connection[C]) reconnectionLoop() {
	checkInterval := 5 * time.Second
	if arws.CheckInterval > 0 {
		checkInterval = arws.CheckInterval
	}

	defer func() {
		close(arws.reconnLoopCloseCh)
	}()

	for {
		arws.logger.Debug("rews.Connection is waiting for reconnection check interval", "interval", checkInterval)
		select {
		case <-arws.connCloseCh:
			return
		case <-time.After(checkInterval):
		}

		if arws.WebSocketConnection.IsClosed() {
			arws.logger.Info("rews.Connection is attempting to reconnect")

			if err := arws.reconnect(context.Background()); err != nil {
				arws.logger.Error("rews.Connection failed to reconnect", "error", err)
				continue
			}
		}
	}
}

package rews

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

type State int

const (
	StateUnknown State = iota
	StateConnecting
	StateConnected
	StateDisconnecting
	StateDisconnected
)

func (s State) TransitionTo(
	newState State,
) (State, error) {
	switch s {
	case StateConnecting:
		switch newState {
		case StateConnected, StateDisconnected:
			return newState, nil
		}
	case StateConnected:
		switch newState {
		case StateDisconnecting, StateDisconnected:
			return newState, nil
		}
	case StateDisconnecting:
		if newState == StateDisconnected {
			return newState, nil
		}
	case StateDisconnected:
		switch newState {
		case StateConnecting, StateDisconnected:
			return newState, nil
		}
	}

	return StateUnknown, fmt.Errorf("invalid state transition from %v to %v", s, newState)
}

type Connection[C connection.WebSocketConnection] struct {
	connection.WebSocketConnection

	connect func(context.Context) (C, error)

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

	reconnLoopCloseCh chan int

	// logger is used to log the state transitions and errors.
	logger logger.Logger

	state State

	mu sync.Mutex
}

var _ connection.Connection = (*Connection[connection.WebSocketConnection])(nil)

// New creates a new auto-reconnecting WebSocket connection.
//
// It takes a function that establishes the WebSocket connection,
// a check interval for reconnection attempts, and a logger.
func New[C connection.WebSocketConnection](
	connect func(context.Context) (C, error),
	checkInterval time.Duration,
	logger logger.Logger,
) *Connection[C] {
	return &Connection[C]{
		CheckInterval: checkInterval,
		connect:       connect,
		state:         StateDisconnected,
		logger:        logger,
	}
}

func (arws *Connection[C]) transitionTo(newState State) error {
	arws.mu.Lock()
	defer arws.mu.Unlock()

	newState, err := arws.state.TransitionTo(newState)
	if err != nil {
		return err
	}

	arws.state = newState
	arws.logger.Debug("ReconnectingWebSocketConnection state transitioned", "new_state", newState)

	return nil
}

func (arws *Connection[C]) mustTransitionTo(newState State) {
	if err := arws.transitionTo(newState); err != nil {
		panic(fmt.Sprintf("BUG: %v", err))
	}
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

	arws.WebSocketConnection, err = arws.connect(ctx)
	if err != nil {
		arws.mustTransitionTo(StateDisconnected)
		return fmt.Errorf("failed to connect: %w", err)
	}

	arws.connCloseCh = make(chan int, 1)
	arws.reconnLoopCloseCh = make(chan int, 1)

	go arws.reconnectionLoop()

	arws.mustTransitionTo(StateConnected)

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
	if err := arws.transitionTo(StateDisconnecting); err != nil {
		return fmt.Errorf("Connection is already closing or closed: %w", err)
	}

	defer func() {
		arws.mustTransitionTo(StateDisconnected)
	}()

	// Ensure the reconnection loop stops first,
	// so that it doesn't try to reconnect after the connection is closed.
	//
	// This implies a possible edge case where the reconnection loop
	// stops even though Close failed.
	//
	// But we accept this trade-off for simplicity,
	// assuming that the user would call AutoReconnectingWebSocketconnection.Close
	// only when the connection is absolutely not needed anymore,
	// like when gracefully shutting things down before exiting the program.
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
		arws.logger.Debug("ReconnectingWebSocketconnection: waiting for reconnection check interval", "interval", checkInterval)
		select {
		case <-arws.connCloseCh:
			return
		case <-time.After(checkInterval):
		}

		if arws.IsClosed() {
			arws.logger.Info("ReconnectingWebSocketConnection: attempting to reconnect")
			if err := arws.WebSocketConnection.Connect(context.Background()); err != nil {
				arws.logger.Error("ReconnectingWebSocketConnection: failed to reconnect", "error", err)
			} else {
				arws.logger.Info("ReconnectingWebSocketConnection: reconnected successfully")
			}
		}
	}
}

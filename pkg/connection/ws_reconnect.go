package connection

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ReconnectingWebSocketConnectionState int

const (
	ReconnectingWebSocketStateUnknown ReconnectingWebSocketConnectionState = iota
	ReconnectingWebSocketStateConnecting
	ReconnectingWebSocketStateConnected
	ReconnectingWebSocketStateDisconnecting
	ReconnectingWebSocketStateDisconnected
)

func (s ReconnectingWebSocketConnectionState) TransitionTo(
	newState ReconnectingWebSocketConnectionState,
) (ReconnectingWebSocketConnectionState, error) {
	switch s {
	case ReconnectingWebSocketStateConnecting:
		switch newState {
		case ReconnectingWebSocketStateConnected, ReconnectingWebSocketStateDisconnected:
			return newState, nil
		}
	case ReconnectingWebSocketStateConnected:
		switch newState {
		case ReconnectingWebSocketStateDisconnecting, ReconnectingWebSocketStateDisconnected:
			return newState, nil
		}
	case ReconnectingWebSocketStateDisconnecting:
		if newState == ReconnectingWebSocketStateDisconnected {
			return newState, nil
		}
	case ReconnectingWebSocketStateDisconnected:
		switch newState {
		case ReconnectingWebSocketStateConnecting, ReconnectingWebSocketStateDisconnected:
			return newState, nil
		}
	}

	return ReconnectingWebSocketStateUnknown, fmt.Errorf("invalid state transition from %v to %v", s, newState)
}

type ReconnectingWebSocketConnection struct {
	*WebSocketConnection

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

	state ReconnectingWebSocketConnectionState

	mu sync.Mutex
}

var _ Connection = (*ReconnectingWebSocketConnection)(nil)

func NewAutoReconnectingWebSocketConnection(c *WebSocketConnection, checkInterval time.Duration) *ReconnectingWebSocketConnection {
	return &ReconnectingWebSocketConnection{
		WebSocketConnection: c,
		state:               ReconnectingWebSocketStateDisconnected,
		CheckInterval:       checkInterval,
	}
}

func (arws *ReconnectingWebSocketConnection) transitionTo(newState ReconnectingWebSocketConnectionState) error {
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

func (arws *ReconnectingWebSocketConnection) mustTransitionTo(newState ReconnectingWebSocketConnectionState) {
	if err := arws.transitionTo(newState); err != nil {
		panic(fmt.Sprintf("BUG: %v", err))
	}
}

// Connect establishes the WebSocket connection and starts the reconnection loop.
// The difference from the regular WebSocketConnection is that
// it will automatically attempt to reconnect if the connection is lost after a successful connection.
//
// Note that the reconnection loop is started only after the initial connection is successful.
// This means that if the initial connection fails, the caller needs to decide what
// to do (e.g. retry, log an error, etc.).
//
// The intension for this behavior is to provide flexibility to the SDK consumer
// about how to retry the initial connection.
//
// For example, the application consuming the SDK is running under a process manager
// or a container orchestrator, error-exiting the process or container
// is a valid way to handle the initial connection failure.
func (arws *ReconnectingWebSocketConnection) Connect(ctx context.Context) error {
	if err := arws.transitionTo(ReconnectingWebSocketStateConnecting); err != nil {
		return err
	}

	if err := arws.WebSocketConnection.Connect(ctx); err != nil {
		arws.mustTransitionTo(ReconnectingWebSocketStateDisconnected)
		return err
	}

	arws.connCloseCh = make(chan int, 1)
	arws.reconnLoopCloseCh = make(chan int, 1)

	go arws.reconnectionLoop()

	arws.mustTransitionTo(ReconnectingWebSocketStateConnected)

	return nil
}

// Close stops the reconnection loop and closes the WebSocket connection.
//
// Althonugh the reconnection loop is guaranteed to stop,
// it tries to Close the underlying connection only once.
//
// This assumes that the SDK consumer will call this method only when
// the application is shutting down gracefully,
// and the connection is not needed anymore.
// The consumer would like to proceed existing anyway,
// without blocking on potentially never ending connection close operation.
//
// Although it may potentially leak resources on the caller and the SurrealDB server side,
// it should be OK because those resources will be eventually cleaned up
// on the process exist and by the operation system in the end.
func (arws *ReconnectingWebSocketConnection) Close(ctx context.Context) error {
	if err := arws.transitionTo(ReconnectingWebSocketStateDisconnecting); err != nil {
		return fmt.Errorf("Connection is already closing or closed: %w", err)
	}

	defer func() {
		arws.mustTransitionTo(ReconnectingWebSocketStateDisconnected)
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

func (arws *ReconnectingWebSocketConnection) reconnectionLoop() {
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

		if arws.IsDisconnected() {
			arws.logger.Info("ReconnectingWebSocketConnection: attempting to reconnect")
			if err := arws.WebSocketConnection.Connect(context.Background()); err != nil {
				arws.logger.Error("ReconnectingWebSocketConnection: failed to reconnect", "error", err)
			} else {
				arws.logger.Info("ReconnectingWebSocketConnection: reconnected successfully")
			}
		}
	}
}

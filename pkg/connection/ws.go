package connection

import "errors"

type WebSocketConnection interface {
	Connection

	IsClosed() bool
}

// ErrConnectionClosed is returned for a request which was waiting for its
// response when the connection closed. The request may or may not have run.
var ErrConnectionClosed = errors.New("connection closed")

// CloseNotifier is implemented by connections which signal when they close,
// whether Close was called or the connection was lost.
type CloseNotifier interface {
	// Closed returns a channel which is closed when the connection closes.
	Closed() <-chan struct{}
	// CloseError returns why the connection closed: nil after Close, or the
	// error which ended it.
	CloseError() error
}

// Redialer is implemented by connections which can make a new connection to
// the same server, with the same configuration, so that a client can
// reconnect after the connection is lost.
type Redialer interface {
	// Redial returns a new connection, not yet connected.
	Redial() Connection
}

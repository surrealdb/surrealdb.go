package connection

import (
	"errors"
	"fmt"
)

type WebSocketConnection interface {
	Connection

	IsClosed() bool
}

// ErrConnectionClosed is returned for a request which was waiting for its
// response when the connection closed. The request may or may not have run.
var ErrConnectionClosed = errors.New("connection closed")

// ClosedError is the error for a request on a closed connection: never nil,
// and wrapping why the connection closed when that is known. A cause which
// already wraps ErrConnectionClosed is returned as it is, so that applying
// ClosedError twice keeps the cause.
func ClosedError(cause error) error {
	if cause == nil {
		return ErrConnectionClosed
	}
	if errors.Is(cause, ErrConnectionClosed) {
		return cause
	}
	return fmt.Errorf("%w: %w", ErrConnectionClosed, cause)
}

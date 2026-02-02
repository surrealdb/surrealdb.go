package constants

import "errors"

// Errors
var (
	InvalidResponse = errors.New("invalid SurrealDB response") //nolint:staticcheck
	ErrQuery        = errors.New("error occurred processing the SurrealDB query")
	ErrNoRow        = errors.New("error no row")
)

var (
	ErrIDInUse            = errors.New("id already in use")
	ErrNoBaseURL          = errors.New("base url not set")
	ErrNoMarshaler        = errors.New("marshaler is not set")
	ErrNoUnmarshaler      = errors.New("unmarshaler is not set")
	ErrNoNamespaceOrDB    = errors.New("namespace or database or both are not set")
	ErrMethodNotAvailable = errors.New("method not available on this connection")

	// Session and transaction errors (SurrealDB v3+)
	ErrSessionsNotSupported     = errors.New("sessions require WebSocket connection")
	ErrTransactionsNotSupported = errors.New("interactive transactions require WebSocket connection")
	ErrSessionClosed            = errors.New("session already detached")
	ErrTransactionClosed        = errors.New("transaction already committed or canceled")
)

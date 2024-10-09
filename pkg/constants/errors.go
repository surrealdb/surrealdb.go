package constants

import "errors"

// Errors
var (
	InvalidResponse = errors.New("invalid SurrealDB response") //nolint:stylecheck
	ErrQuery        = errors.New("error occurred processing the SurrealDB query")
	ErrNoRow        = errors.New("error no row")
)
var (
	ErrIDInUse            = errors.New("id already in use")
	ErrTimeout            = errors.New("timeout")
	ErrNoBaseURL          = errors.New("base url not set")
	ErrNoMarshaler        = errors.New("marshaler is not set")
	ErrNoUnmarshaler      = errors.New("unmarshaler is not set")
	ErrNoNamespaceOrDB    = errors.New("namespace or database or both are not set")
	ErrMethodNotAvailable = errors.New("method not available on this connection")
)

package constants

import "errors"

// Errors
var (
	InvalidResponse = errors.New("invalid SurrealDB response") //nolint:stylecheck
	ErrQuery        = errors.New("error occurred processing the SurrealDB query")
	ErrNoRow        = errors.New("error no row")
)

var (
	WebsocketScheme      = "ws"
	WebsocketSucerScheme = "wss"
	HTTPScheme           = "http"
	HTTPSecureScheme     = "https"
)

package constants

import "time"

var (
	AuthTokenKey = "auth_token"
)

const (
	// RequestIDLength size of id sent on WS request
	RequestIDLength = 16
	// CloseMessageCode identifier the message id for a close request
	CloseMessageCode = 1000
	// DefaultTimeout timeout in seconds

	DefaultWSTimeout = 30 * time.Second

	DefaultHTTPTimeout = 10 * time.Second

	OneSecondToNanoSecond = 1_000_000_000
)

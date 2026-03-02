package connection

// ServerError represents a structured error from SurrealDB v3.
// Only use this when you know you are running against a SurrealDB v3 server.
//
// Extract from RPC errors using errors.As:
//
//	var se *connection.ServerError
//	if errors.As(err, &se) {
//	    fmt.Println(se.Kind, se.Details)
//	}
//
// ServerError carries structured information including Kind, Details, and a cause chain.
// Use the helper functions in the surrealdb package (IsNotAllowed, IsNotFound, etc.)
// for ergonomic kind checking, or inspect Kind directly.
type ServerError struct {
	// Code is the JSON-RPC numeric error code.
	Code int

	// Message is the error message.
	Message string

	// Kind is the structured error kind (e.g. "NotFound", "NotAllowed").
	Kind string

	// Details contains kind-specific structured error details.
	// SurrealDB v3 only. nil for v2 servers.
	//
	// In SurrealDB v3, this follows the { "kind": "...", "details": ... } format (internally-tagged).
	// In older versions, this may be a string (unit variants) or a map with the
	// variant name as key (externally-tagged format).
	Details any

	// Cause is the underlying error in the cause chain.
	Cause *ServerError
}

// Error implements the error interface.
// When a cause chain is present, the messages are joined with ": ".
func (e ServerError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e ServerError) Is(target error) bool {
	_, ok := target.(ServerError)
	return ok
}

func (e *ServerError) As(err any) bool {
	switch dst := err.(type) {
	case *ServerError:
		*dst = *e
		return true
	case **ServerError:
		*dst = e
		return true
	default:
		return false
	}
}

// Unwrap implements the Go errors.Unwrap interface, enabling
// errors.Unwrap(), errors.Is(), and errors.As() to traverse the
// server error cause chain.
func (e ServerError) Unwrap() error {
	return e.Cause
}

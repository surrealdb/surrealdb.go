package spectron

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors. Callers compare using [errors.Is]:
//
//	if errors.Is(err, spectron.ErrAuth) { ... }
//
// Use [errors.As] to pull out the full *APIError details:
//
//	var apiErr *spectron.APIError
//	if errors.As(err, &apiErr) {
//	    fmt.Println(apiErr.StatusCode, apiErr.TraceID)
//	}
var (
	// ErrAuth signals a 401 — bearer token missing, malformed, or rejected.
	ErrAuth = errors.New("spectron: authentication failed")
	// ErrScope signals a 403 — token does not authorize the requested principal scope.
	ErrScope = errors.New("spectron: scope forbidden")
	// ErrNotFound signals a 404 — addressed entity / document / session does not exist.
	ErrNotFound = errors.New("spectron: not found")
)

// APIError describes a non-2xx response from the Spectron API.
type APIError struct {
	// StatusCode is the HTTP status returned by the server, or 0 when the
	// request never reached the server (e.g. connection failure).
	StatusCode int
	// Message is the human-readable error extracted from the response body
	// or a synthetic description for transport-level failures.
	Message string
	// TraceID is the X-Trace-Id header value, if present.
	TraceID string
	// Body is the decoded response body. Best-effort: JSON when possible,
	// otherwise the raw string, otherwise nil.
	Body any

	sentinel error
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.StatusCode == 0 {
		return fmt.Sprintf("spectron: %s", e.Message)
	}
	return fmt.Sprintf("spectron: [%d] %s", e.StatusCode, e.Message)
}

// Unwrap exposes the matching sentinel (if any) so [errors.Is] succeeds.
func (e *APIError) Unwrap() error { return e.sentinel }

// errorFromResponse builds an *APIError from a parsed HTTP response.
func errorFromResponse(status int, body any, headers http.Header) *APIError {
	msg := "request failed"
	switch v := body.(type) {
	case map[string]any:
		for _, k := range []string{"message", "title", "error"} {
			if s, ok := v[k].(string); ok && s != "" {
				msg = s
				break
			}
		}
	case string:
		if v != "" {
			msg = v
		}
	}

	var trace string
	if headers != nil {
		trace = headers.Get("X-Trace-Id")
	}

	return &APIError{
		StatusCode: status,
		Message:    msg,
		TraceID:    trace,
		Body:       body,
		sentinel:   sentinelFor(status),
	}
}

func sentinelFor(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return ErrAuth
	case http.StatusForbidden:
		return ErrScope
	case http.StatusNotFound:
		return ErrNotFound
	}
	return nil
}

// decodeJSON best-effort decodes body bytes into a Go value: JSON when
// parseable, raw string otherwise, nil when empty.
func decodeJSON(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(body, &out); err == nil {
		return out
	}
	return string(body)
}

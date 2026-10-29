package connection

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HealthChecker is implemented by connections which can ask the server whether
// it is healthy.
type HealthChecker interface {
	// Health returns nil if the server answers GET /health with a 2xx status.
	Health(ctx context.Context) error
}

// HTTPEndpoint is implemented by connections which know the server's HTTP base
// URL, for the endpoints which have no RPC method, such as /import and /export.
type HTTPEndpoint interface {
	// HTTPEndpoint returns the base URL, which may be a WebSocket URL, and the
	// HTTP client to use, or nil for a default one.
	HTTPEndpoint() (baseURL string, client *http.Client)
}

// DefaultHealthClient is the HTTP client CheckHealth uses when given none.
var DefaultHealthClient = &http.Client{Timeout: 10 * time.Second}

// HTTPBaseURL returns the HTTP form of a connection's base URL: ws becomes http
// and wss becomes https. A path, such as the prefix of a reverse proxy, is kept,
// as it is for /rpc, but a trailing /rpc is dropped, so that the URL of the RPC
// endpoint gives the base URL too.
func HTTPBaseURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("cannot derive an HTTP URL from %q", baseURL)
	}
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/rpc")
	u.RawPath = ""
	u.RawQuery = ""
	return strings.TrimSuffix(u.String(), "/"), nil
}

// HealthError is returned by a health check which the server answered with a
// status other than 2xx, as it does when it cannot reach its datastore.
//
// Extract it from a health check's error using errors.As:
//
//	var he *connection.HealthError
//	if errors.As(err, &he) {
//	    fmt.Println(he.StatusCode, he.Body)
//	}
type HealthError struct {
	// StatusCode is the HTTP status code of the answer, such as 503.
	StatusCode int
	// Status is the HTTP status of the answer, such as "503 Service Unavailable".
	Status string
	// Body is the start of the answer's body, at most 4 KiB, with surrounding
	// white space removed. It is empty when the answer had none.
	Body string
}

// Error implements the error interface.
func (e *HealthError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("health check failed: %s", e.Status)
	}
	return fmt.Sprintf("health check failed: %s: %s", e.Status, e.Body)
}

// CheckHealth sends GET /health to the server at baseURL, which may be an HTTP
// or a WebSocket URL, and returns nil if it answers with a 2xx status. If it
// answers with another status, CheckHealth returns a [*HealthError].
//
// SurrealDB has no health RPC method; the HTTP endpoint is the health check for
// every transport. It needs no authentication.
func CheckHealth(ctx context.Context, client *http.Client, baseURL string) error {
	base, err := HTTPBaseURL(baseURL)
	if err != nil {
		return err
	}
	if client == nil {
		client = DefaultHealthClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HealthError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	return nil
}

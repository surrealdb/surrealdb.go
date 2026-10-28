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

// CheckHealth sends GET /health to the server at baseURL, which may be an HTTP
// or a WebSocket URL, and returns nil if it answers with a 2xx status.
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
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			return fmt.Errorf("health check failed: %s", resp.Status)
		}
		return fmt.Errorf("health check failed: %s: %s", resp.Status, msg)
	}
	return nil
}

package memory

import (
	"context"
	"net/http"
)

// Health checks server liveness via GET /api/v1/health. The endpoint is not
// context-scoped, so this bypasses the Client's per-context base path. It
// returns nil when the server reports healthy and an *APIError otherwise.
func (c *Client) Health(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodGet, "/api/v1/health", nil, nil, false)
}

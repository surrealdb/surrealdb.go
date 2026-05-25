package spectron

import (
	"context"
	"net/http"
)

// Remember writes a single fact (or extraction directive) to the context.
//
// Remember is treated as idempotent: a retry within a 30-second window
// reuses the previous Idempotency-Key, allowing the server to collapse
// retried attempts onto the original.
func (c *Client) Remember(ctx context.Context, req RememberRequest) (*RememberResponse, error) {
	var out RememberResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/facts", req, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

// RememberMany writes a batch of messages to the context.
//
// Like [Client.Remember], RememberMany is idempotent within the 30-second
// bucket.
func (c *Client) RememberMany(ctx context.Context, req RememberManyRequest) (*RememberBatchResponse, error) {
	var out RememberBatchResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/facts/batch", req, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

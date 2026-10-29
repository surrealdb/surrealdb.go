package memory

import (
	"context"
	"net/http"
)

// Recall searches the context for hits matching the query.
//
// Recall is a read behind a POST, so it carries an Idempotency-Key and is
// retried on transport failures and 5xx.
func (c *Client) Recall(ctx context.Context, req *RecallRequest) (*RecallResponse, error) {
	var out RecallResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/query", req, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

// ForgetOption tweaks a [Client.Forget] call.
type ForgetOption func(*forgetRequest)

type forgetRequest struct {
	Query  string `json:"query"`
	Purge  bool   `json:"purge,omitempty"`
	DryRun bool   `json:"dryRun,omitempty"`
}

// WithPurge requests a hard purge instead of a soft delete.
func WithPurge() ForgetOption {
	return func(r *forgetRequest) { r.Purge = true }
}

// WithDryRun reports what Forget would remove without removing it.
func WithDryRun() ForgetOption {
	return func(r *forgetRequest) { r.DryRun = true }
}

// Forget removes facts matching the query from the context.
func (c *Client) Forget(ctx context.Context, query string, opts ...ForgetOption) (*ForgetResponse, error) {
	req := forgetRequest{Query: query}
	for _, opt := range opts {
		opt(&req)
	}
	var out ForgetResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/forget", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

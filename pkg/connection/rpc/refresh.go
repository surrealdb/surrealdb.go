package rpc

import (
	"context"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// Refresh exchanges a token pair for a new one, using the refresh RPC of
// SurrealDB v3+. The server revokes the refresh token it is given, so only the
// returned pair can be refreshed again.
func Refresh(c connection.Connection, ctx context.Context, tokens connection.Tokens) (*connection.Tokens, error) {
	return authWithRefresh(c, ctx, "refresh", tokens)
}

// Revoke revokes the refresh token of a token pair, so that it can no longer
// be used to obtain new tokens. It does not sign the session out.
func Revoke(c connection.Connection, ctx context.Context, tokens connection.Tokens) error {
	return connection.Send[any](c, ctx, nil, "revoke", tokens)
}

// Reset returns the session to the state it was in when it was created: signed
// out, with no namespace, database or variables, and with its live queries
// killed.
func Reset(c connection.Connection, ctx context.Context) error {
	return connection.Send[any](c, ctx, nil, "reset")
}

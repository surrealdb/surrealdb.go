package rpc

import (
	"context"

	"github.com/surrealdb/surrealdb.go/v2/pkg/connection"
)

func SignUp(c connection.Connection, ctx context.Context, authData any) (string, error) {
	return authToken(c, ctx, "signup", authData)
}

// SignUpWithRefresh signs up using a TYPE RECORD access method with WITH REFRESH enabled.
// It returns both an access token and a refresh token.
func SignUpWithRefresh(c connection.Connection, ctx context.Context, authData any) (*connection.Tokens, error) {
	return authWithRefresh(c, ctx, "signup", authData)
}

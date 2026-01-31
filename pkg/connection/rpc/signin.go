package rpc

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

func SignIn(c connection.Connection, ctx context.Context, authData any) (string, error) {
	var token connection.RPCResponse[string]
	if err := connection.Send(c, ctx, &token, "signin", authData); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// SignInWithRefresh signs in using a TYPE RECORD access method with WITH REFRESH enabled.
// This is only supported in SurrealDB v3+ and returns both an access token and a refresh token.
func SignInWithRefresh(c connection.Connection, ctx context.Context, authData any) (*connection.Tokens, error) {
	var response connection.RPCResponse[connection.Tokens]
	if err := connection.Send(c, ctx, &response, "signin", authData); err != nil {
		return nil, err
	}

	if response.Result == nil {
		return nil, fmt.Errorf("signin response is nil; ensure WITH REFRESH is enabled on the access method")
	}

	return response.Result, nil
}

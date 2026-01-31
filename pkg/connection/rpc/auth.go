package rpc

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// authToken sends an RPC method that returns a string token (like signin/signup without WITH REFRESH).
func authToken(c connection.Connection, ctx context.Context, method string, authData any) (string, error) {
	var token connection.RPCResponse[string]
	if err := connection.Send(c, ctx, &token, method, authData); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// authWithRefresh sends an RPC method that returns Tokens (like signin/signup WITH REFRESH).
func authWithRefresh(c connection.Connection, ctx context.Context, method string, authData any) (*connection.Tokens, error) {
	var response connection.RPCResponse[connection.Tokens]
	if err := connection.Send(c, ctx, &response, method, authData); err != nil {
		return nil, err
	}

	if response.Result == nil {
		return nil, fmt.Errorf("%s response is nil; ensure WITH REFRESH is enabled on the access method", method)
	}

	return response.Result, nil
}

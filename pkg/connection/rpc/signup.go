package rpc

import (
	"context"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

func SignUp(c connection.Connection, ctx context.Context, authData any) (string, error) {
	var token connection.RPCResponse[string]
	if err := connection.Send(c, ctx, &token, "signup", authData); err != nil {
		return "", err
	}

	return *token.Result, nil
}

package rpc

import (
	"context"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

func Authenticate(c connection.Connection, ctx context.Context, token string) error {
	if err := connection.Send[any](c, ctx, nil, "authenticate", token); err != nil {
		return err
	}

	return nil
}

package rpc

import (
	"context"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

func Invalidate(c connection.Connection, ctx context.Context) error {
	if err := connection.Send[any](c, ctx, nil, "invalidate"); err != nil {
		return err
	}

	return nil
}

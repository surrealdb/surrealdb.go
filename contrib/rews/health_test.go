package rews_test

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

// TestHealth checks that Health works through a reconnecting connection.
func TestHealth(t *testing.T) {
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	require.NoError(t, err)
	config := connection.NewConfig(u)

	conn := rews.New(func(context.Context) (*gorillaws.Connection, error) {
		return gorillaws.New(config), nil
	}, time.Second, config.Unmarshaler, logger.New(slog.NewTextHandler(io.Discard, nil)))

	ctx := context.Background()
	db, err := surrealdb.FromConnection(ctx, conn)
	require.NoError(t, err)
	defer db.Close(ctx)

	assert.NoError(t, conn.Health(ctx))
	assert.NoError(t, db.Health(ctx))
}

package surrealdb_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/internal/fakesdb"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
)

func TestHealth(t *testing.T) {
	ctx := context.Background()

	for family, endpoint := range endpoints() {
		t.Run(family, func(t *testing.T) {
			db, err := surrealdb.FromEndpointURLString(ctx, endpoint)
			require.NoError(t, err)
			defer db.Close(ctx)

			assert.NoError(t, db.Health(ctx))
		})
	}

	t.Run("server gone", func(t *testing.T) {
		require.Error(t, connection.CheckHealth(ctx, nil, "http://127.0.0.1:1"))
	})
}

// TestVersionCheck connects to fake servers reporting versions inside and
// outside the supported range.
func TestVersionCheck(t *testing.T) {
	ctx := context.Background()

	connect := func(t *testing.T, version string, opts ...surrealdb.Option) (*surrealdb.DB, error) {
		t.Helper()
		server := fakesdb.NewServer("127.0.0.1:0")
		server.Version = version
		require.NoError(t, server.Start())
		t.Cleanup(func() { _ = server.Stop() })

		u, err := url.ParseRequestURI("ws://" + server.Address())
		require.NoError(t, err)
		return surrealdb.FromConnection(ctx, gorillaws.New(connection.NewConfig(u)), opts...)
	}

	for _, v := range []string{"2.6.0", "3.3.4", "3.4.0-nightly", "4.0.0", "4.7.1"} {
		db, err := connect(t, v)
		require.NoError(t, err, v)
		_ = db.Close(ctx)
	}

	for _, v := range []string{"1.5.6", "2.0.4", "5.0.0", "5.1.0-alpha"} {
		_, err := connect(t, v)
		var unsupported *surrealdb.UnsupportedVersionError
		require.ErrorAs(t, err, &unsupported, v)
		assert.Equal(t, v, unsupported.Version)
	}

	db, err := connect(t, "5.0.0", surrealdb.WithVersionCheck(false))
	require.NoError(t, err, "the check can be turned off")
	_ = db.Close(ctx)
}

// TestVersionCheck_realServer checks that the server under test is supported,
// over both transports, before anything else has been called.
func TestVersionCheck_realServer(t *testing.T) {
	ctx := context.Background()
	for family, endpoint := range endpoints() {
		db, err := surrealdb.FromEndpointURLString(ctx, endpoint)
		require.NoError(t, err, family)
		_ = db.Close(ctx)
	}
}

func endpoints() map[string]string {
	return map[string]string{"ws": testenv.GetSurrealDBWSURL(), "http": httpEndpoint()}
}

package surrealdb_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/fxamacker/cbor/v2"

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

// FromConnection and FromEndpointURLString keep the types they had before
// options were added, so code that holds them as values still builds.
var (
	_ func(context.Context, connection.Connection) (*surrealdb.DB, error) = surrealdb.FromConnection
	_ func(context.Context, string) (*surrealdb.DB, error)                = surrealdb.FromEndpointURLString
)

// startVersionServer starts a fake server reporting version, answering version
// as stubs say if any match, and returns its URL.
func startVersionServer(t *testing.T, version string, stubs ...fakesdb.StubResponse) *url.URL {
	t.Helper()
	server := fakesdb.NewServer("127.0.0.1:0")
	server.Version = version
	for _, stub := range stubs {
		server.AddStubResponse(stub)
	}
	require.NoError(t, server.Start())
	t.Cleanup(func() { _ = server.Stop() })

	u, err := url.ParseRequestURI("ws://" + server.Address())
	require.NoError(t, err)
	return u
}

// TestVersionCheck connects to fake servers reporting versions inside and
// outside the supported range.
func TestVersionCheck(t *testing.T) {
	ctx := context.Background()
	check := surrealdb.WithVersionCheck(true)

	connect := func(t *testing.T, version string, opts ...surrealdb.Option) (*surrealdb.DB, error) {
		t.Helper()
		u := startVersionServer(t, version)
		return surrealdb.FromConnectionWithOptions(ctx, gorillaws.New(connection.NewConfig(u)), opts...)
	}

	for _, v := range []string{"2.6.0", "3.3.4", "3.4.0-nightly", "4.0.0", "4.7.1"} {
		db, err := connect(t, v, check)
		require.NoError(t, err, v)
		_ = db.Close(ctx)
	}

	for _, v := range []string{"1.5.6", "2.0.4", "5.0.0", "5.1.0-alpha"} {
		_, err := connect(t, v, check)
		var unsupported *surrealdb.UnsupportedVersionError
		require.ErrorAs(t, err, &unsupported, v)
		assert.Equal(t, v, unsupported.Version)
		assert.Contains(t, err.Error(), "WithVersionCheck(false)", "the error names the way out")

		db, err := connect(t, v)
		require.NoError(t, err, "%s: the check is off by default", v)
		_ = db.Close(ctx)

		db, err = connect(t, v, check, surrealdb.WithVersionCheck(false))
		require.NoError(t, err, "%s: the check can be turned off", v)
		_ = db.Close(ctx)
	}
}

// TestVersionCheck_off checks that the constructors without options, and the
// deprecated ones, connect to a server whatever its version, as they did
// before the check was added.
func TestVersionCheck_off(t *testing.T) {
	ctx := context.Background()
	u := startVersionServer(t, "5.0.0")

	db, err := surrealdb.FromConnection(ctx, gorillaws.New(connection.NewConfig(u)))
	require.NoError(t, err, "FromConnection")
	_ = db.Close(ctx)

	for name, connect := range map[string]func() (*surrealdb.DB, error){
		"FromEndpointURLString": func() (*surrealdb.DB, error) {
			return surrealdb.FromEndpointURLString(ctx, u.String())
		},
		"FromEndpointURLStringWithOptions": func() (*surrealdb.DB, error) {
			return surrealdb.FromEndpointURLStringWithOptions(ctx, u.String())
		},
		"Connect": func() (*surrealdb.DB, error) {
			return surrealdb.Connect(ctx, u.String()) //nolint:staticcheck // the deprecated constructor must keep working
		},
		"New": func() (*surrealdb.DB, error) {
			return surrealdb.New(u.String()) //nolint:staticcheck // the deprecated constructor must keep working
		},
	} {
		db, err := connect()
		require.NoError(t, err, name)
		_ = db.Close(ctx)
	}
}

// TestVersionCheck_versionRefused connects to a server that refuses the
// version method, as one started with --deny-rpc version does. Only a client
// that turned the check on fails, and the error names the way out.
func TestVersionCheck_versionRefused(t *testing.T) {
	ctx := context.Background()
	u := startVersionServer(t, "", fakesdb.ErrorStubResponse("version", -32602, "Method not allowed"))

	db, err := surrealdb.FromConnection(ctx, gorillaws.New(connection.NewConfig(u)))
	require.NoError(t, err)
	_ = db.Close(ctx)

	_, err = surrealdb.FromConnectionWithOptions(ctx, gorillaws.New(connection.NewConfig(u)),
		surrealdb.WithVersionCheck(true))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Method not allowed")
	assert.Contains(t, err.Error(), "WithVersionCheck(false)", "the error names the way out")
}

// TestVersionCheck_realServer checks that the server under test is supported,
// over both transports, before anything else has been called.
func TestVersionCheck_realServer(t *testing.T) {
	ctx := context.Background()
	for family, endpoint := range endpoints() {
		db, err := surrealdb.FromEndpointURLStringWithOptions(ctx, endpoint, surrealdb.WithVersionCheck(true))
		require.NoError(t, err, family)
		_ = db.Close(ctx)
	}
}

func endpoints() map[string]string {
	return map[string]string{"ws": testenv.GetSurrealDBWSURL(), "http": httpEndpoint()}
}

// A version answered as null, or without a result, fails the version check
// and Version rather than panicking.
func TestVersionCheck_noVersion(t *testing.T) {
	ctx := context.Background()
	u := startVersionServer(t, "", fakesdb.SimpleStubResponse("version", nil))

	connections := map[string]func() connection.Connection{
		"null": func() connection.Connection { return gorillaws.New(connection.NewConfig(u)) },
		"missing": func() connection.Connection {
			return &withoutVersionResult{gorillaws.New(connection.NewConfig(u))}
		},
	}
	for name, newConnection := range connections {
		t.Run(name, func(t *testing.T) {
			_, err := surrealdb.FromConnectionWithOptions(ctx, newConnection(), surrealdb.WithVersionCheck(true))
			require.ErrorContains(t, err, "unexpected version data")

			db, err := surrealdb.FromConnection(ctx, newConnection())
			require.NoError(t, err)
			defer db.Close(ctx)
			_, err = db.Version(ctx)
			require.ErrorContains(t, err, "unexpected version data")
		})
	}
}

// withoutVersionResult answers version successfully, but without a result.
type withoutVersionResult struct {
	*gorillaws.Connection
}

func (c *withoutVersionResult) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	if method == "version" {
		return &connection.RPCResponse[cbor.RawMessage]{ID: "version"}, nil
	}
	return c.Connection.Send(ctx, method, params...)
}

package surrealdb_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

func httpEndpoint() string {
	return strings.Replace(strings.TrimSuffix(testenv.GetSurrealDBWSURL(), "/rpc"), "ws", "http", 1)
}

// TestHTTP_withoutUse checks that an HTTP client can do what needs no namespace
// or database before calling Use: ask for the version, sign in as root, and run
// a query at the root level.
func TestHTTP_withoutUse(t *testing.T) {
	ctx := context.Background()

	db, err := surrealdb.FromEndpointURLString(ctx, httpEndpoint())
	require.NoError(t, err)
	defer db.Close(ctx)

	v, err := db.Version(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, v.Version)

	_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)

	res, err := surrealdb.Query[map[string]any](ctx, db, "INFO FOR ROOT", nil)
	require.NoError(t, err)
	require.Len(t, *res, 1)
	assert.Contains(t, (*res)[0].Result, "namespaces")

	// A root user can create a namespace and database without having chosen one.
	// Other test packages define namespaces and databases on the same server at
	// the same time, which can fail with a transaction conflict, so retry as
	// testenv.DefineNamespaceAndDatabase does.
	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		_, err = surrealdb.Query[any](ctx, db, `
			DEFINE NAMESPACE IF NOT EXISTS http_without_use;
			USE NS http_without_use;
			DEFINE DATABASE IF NOT EXISTS http_without_use;
		`, nil)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
	}
	require.NoError(t, err)

	// Once a namespace and database are chosen, they are used.
	require.NoError(t, db.Use(ctx, "http_without_use", "http_without_use"))
	require.NoError(t, testenv.DefineSchemalessTables(db, "thing"))
}

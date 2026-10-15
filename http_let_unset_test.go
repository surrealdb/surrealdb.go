package surrealdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
)

// TestHTTP_letAndUnset checks that variables defined with Let are visible to
// later queries over HTTP, that Unset removes them, and that a variable named
// like a piece of connection state does not change that state.
func TestHTTP_letAndUnset(t *testing.T) {
	ctx := context.Background()

	db, err := surrealdb.FromEndpointURLString(ctx, httpEndpoint())
	require.NoError(t, err)
	defer db.Close(ctx)

	_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, `
		DEFINE NAMESPACE IF NOT EXISTS http_let;
		USE NS http_let;
		DEFINE DATABASE IF NOT EXISTS http_let;
	`, nil)
	require.NoError(t, err)
	require.NoError(t, db.Use(ctx, "http_let", "http_let"))

	require.NoError(t, db.Let(ctx, "name", "tobie"))
	require.NoError(t, db.Let(ctx, "namespace", "somewhere_else"))
	require.NoError(t, db.Let(ctx, "auth_token", "not a token"))

	res, err := surrealdb.Query[map[string]any](ctx, db, "RETURN { name: $name, namespace: $namespace, ns: session::ns() }", nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "tobie", "namespace": "somewhere_else", "ns": "http_let"}, (*res)[0].Result)

	// Variables passed with a query take precedence.
	res, err = surrealdb.Query[map[string]any](ctx, db, "RETURN { name: $name }", map[string]any{"name": "jaime"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "jaime"}, (*res)[0].Result)

	require.NoError(t, db.Unset(ctx, "name"))
	unset, err := surrealdb.Query[any](ctx, db, "RETURN $name", nil)
	require.NoError(t, err)
	assert.Nil(t, (*unset)[0].Result)

	// Still signed in as root: the variable named auth_token did not replace the token.
	_, err = surrealdb.Query[any](ctx, db, "INFO FOR ROOT", nil)
	require.NoError(t, err)
}

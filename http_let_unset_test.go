package surrealdb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

	// A NONE value removes the variable, as on a WebSocket.
	require.NoError(t, db.Let(ctx, "name", "tobie"))
	require.NoError(t, db.Let(ctx, "name", models.None))
	unset, err = surrealdb.Query[any](ctx, db, "RETURN $name", nil)
	require.NoError(t, err)
	assert.Nil(t, (*unset)[0].Result)

	// A name the server refuses to bind is refused, and later queries still work.
	for _, name := range []string{"access", "auth", "token", "session"} {
		require.Error(t, db.Let(ctx, name, "x"), name)
	}
	_, err = surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.NoError(t, err)
}

// TestHTTP_expiredTokenCanBeReplaced checks that a connection whose token the
// server no longer accepts can sign in again, and invalidate it, though the
// server answers 401 to every request carrying it.
func TestHTTP_expiredTokenCanBeReplaced(t *testing.T) {
	ctx := context.Background()

	db, err := surrealdb.FromEndpointURLString(ctx, httpEndpoint())
	require.NoError(t, err)
	defer db.Close(ctx)

	root := surrealdb.Auth{Username: "root", Password: "root"}
	_, err = db.SignIn(ctx, root)
	require.NoError(t, err)
	// A name of its own, as runs against the same server may overlap.
	user := fmt.Sprintf("http_short_lived_%d", time.Now().UnixNano())
	_, err = surrealdb.Query[any](ctx, db,
		"DEFINE USER "+user+" ON ROOT PASSWORD 'short-lived' ROLES VIEWER DURATION FOR TOKEN 1s", nil)
	require.NoError(t, err)
	defer func() {
		_, _ = db.SignIn(ctx, root)
		_, _ = surrealdb.Query[any](ctx, db, "REMOVE USER IF EXISTS "+user+" ON ROOT", nil)
	}()

	expire := func() {
		t.Helper()
		_, signInErr := db.SignIn(ctx, surrealdb.Auth{Username: user, Password: "short-lived"})
		require.NoError(t, signInErr)
		time.Sleep(2 * time.Second)
		_, queryErr := surrealdb.Query[any](ctx, db, "RETURN 1", nil)
		require.Error(t, queryErr, "the server refuses the expired token")
	}

	expire()
	_, err = db.SignIn(ctx, root)
	require.NoError(t, err, "signin is sent without the expired token")
	_, err = surrealdb.Query[any](ctx, db, "INFO FOR ROOT", nil)
	require.NoError(t, err)

	expire()
	require.NoError(t, db.Invalidate(ctx), "invalidate forgets the expired token")
	_, err = db.SignIn(ctx, root)
	require.NoError(t, err)
}

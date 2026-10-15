package surrealdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

func TestSessions_listsAttached(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	db := mustNewWS("sessions", "list", "t")
	defer db.Close(ctx)

	first, err := db.Attach(ctx)
	require.NoError(t, err)
	second, err := db.Attach(ctx)
	require.NoError(t, err)
	defer func() { _ = second.Detach(ctx) }()

	sessions, err := db.Sessions(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []*surrealdb.Session{first, second}, sessions,
		"the list holds the Session objects Attach returned, and not the default session")

	require.NoError(t, first.Detach(ctx))
	sessions, err = db.Sessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*surrealdb.Session{second}, sessions)
}

// sessionView returns the namespace, database and $x of a session.
func sessionView[S *surrealdb.DB | *surrealdb.Session](t *testing.T, s S) []any {
	t.Helper()
	res, err := surrealdb.Query[[]any](context.Background(), s, "RETURN [session::ns(), session::db(), $x]", nil)
	require.NoError(t, err)
	return (*res)[0].Result
}

func TestFork(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	db := mustNewWS("sessions", "fork", "t")
	defer db.Close(ctx)
	require.NoError(t, db.Let(ctx, "x", "from the default session"))

	fork, err := db.Fork(ctx)
	require.NoError(t, err)
	defer func() { _ = fork.Detach(ctx) }()

	assert.Equal(t, []any{"sessions", "fork", "from the default session"}, sessionView(t, fork),
		"the fork has the namespace, database and variables, and is signed in")

	require.NoError(t, fork.Let(ctx, "x", "changed in the fork"))
	require.NoError(t, fork.Use(ctx, "sessions", "elsewhere"))
	assert.Equal(t, []any{"sessions", "fork", "from the default session"}, sessionView(t, db),
		"the sessions are independent")

	t.Run("a fork of a session", func(t *testing.T) {
		again, err := fork.Fork(ctx)
		require.NoError(t, err)
		defer func() { _ = again.Detach(ctx) }()
		assert.Equal(t, []any{"sessions", "elsewhere", "changed in the fork"}, sessionView(t, again))
	})

	t.Run("a fork of a signed out session is signed out", func(t *testing.T) {
		require.NoError(t, fork.Invalidate(ctx))
		signedOut, err := fork.Fork(ctx)
		require.NoError(t, err)
		defer func() { _ = signedOut.Detach(ctx) }()
		_, err = surrealdb.Query[any](ctx, signedOut, "RETURN 1", nil)
		require.Error(t, err)
	})
}

func TestFork_httpNotSupported(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNewHTTP("sessions", "fork_http", "t")
	defer db.Close(ctx)

	_, err := db.Fork(ctx)
	require.ErrorIs(t, err, constants.ErrSessionsNotSupported)
	_, err = db.Sessions(ctx)
	require.ErrorIs(t, err, constants.ErrSessionsNotSupported)
}

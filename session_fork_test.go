package surrealdb_test

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

// sessionCount returns how many sessions the server lists on the connection.
func sessionCount(t *testing.T, db *surrealdb.DB) int {
	t.Helper()
	sessions, err := db.Sessions(context.Background())
	require.NoError(t, err)
	return len(sessions)
}

// defineRootUser defines a root user whose tokens live for tokenDuration, and
// removes it when the test ends.
func defineRootUser(t *testing.T, name, tokenDuration string) {
	t.Helper()
	ctx := context.Background()
	root := mustNewWS("sessions", "fork_users")
	t.Cleanup(func() {
		_, _ = surrealdb.Query[any](ctx, root, "REMOVE USER IF EXISTS "+name+" ON ROOT", nil)
		_ = root.Close(ctx)
	})
	_, err := surrealdb.Query[any](ctx, root, fmt.Sprintf(
		"DEFINE USER OVERWRITE %s ON ROOT PASSWORD 'secret' ROLES OWNER DURATION FOR TOKEN %s, FOR SESSION NONE",
		name, tokenDuration), nil)
	require.NoError(t, err)
}

// A session signed in with a password outlives its token, but its fork is
// signed in with the token, so Fork fails once the token has expired, without
// attaching a session.
func TestFork_tokenExpired(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	defineRootUser(t, "fork_expiring", "2s")

	db := newClient(t, testenv.GetSurrealDBWSURL())
	_, err := db.SignIn(ctx, surrealdb.Auth{Username: "fork_expiring", Password: "secret"})
	require.NoError(t, err)
	before := sessionCount(t, db)

	time.Sleep(3 * time.Second)
	_, err = surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.NoError(t, err, "the session is still signed in")

	_, err = db.Fork(ctx)
	require.ErrorIs(t, err, constants.ErrForkTokenExpired)
	assert.Equal(t, before, sessionCount(t, db), "no session is attached")
}

// A fork whose restore fails is detached.
func TestFork_failedRestoreDetaches(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	defineRootUser(t, "fork_removed", "1h")

	db := newClient(t, testenv.GetSurrealDBWSURL())
	_, err := db.SignIn(ctx, surrealdb.Auth{Username: "fork_removed", Password: "secret"})
	require.NoError(t, err)
	before := sessionCount(t, db)

	root := mustNewWS("sessions", "fork_users")
	defer root.Close(ctx)
	_, err = surrealdb.Query[any](ctx, root, "REMOVE USER fork_removed ON ROOT", nil)
	require.NoError(t, err)

	_, err = db.Fork(ctx)
	require.Error(t, err, "the token of a removed user does not authenticate")
	assert.Equal(t, before, sessionCount(t, db), "the fork is detached")
}

// A Fork whose context ends while the session is restored detaches the fork
// all the same.
func TestFork_cancelledDetaches(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	db := mustNewWS("sessions", "fork_cancel")
	defer db.Close(ctx)
	for i := range 2000 {
		require.NoError(t, db.Let(ctx, fmt.Sprintf("v%d", i), i))
	}
	before := sessionCount(t, db)

	short, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	_, err := db.Fork(short)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, before, sessionCount(t, db), "the fork is detached")
}

// Let keeps the value as it was when it was set, so a later change to it does
// not reach a fork.
func TestFork_letValueChangedAfterwards(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	db := mustNewWS("sessions", "fork_let")
	defer db.Close(ctx)

	m := map[string]any{"a": 1}
	require.NoError(t, db.Let(ctx, "x", m))
	m["a"] = 2

	fork, err := db.Fork(ctx)
	require.NoError(t, err)
	defer func() { _ = fork.Detach(ctx) }()

	res, err := surrealdb.Query[map[string]any](ctx, fork, "RETURN $x", nil)
	require.NoError(t, err)
	assert.EqualValues(t, map[string]any{"a": uint64(1)}, (*res)[0].Result)
}

// After Revoke, the session stays signed in with the last renewed token, and
// so does its fork.
func TestFork_afterRevoke(t *testing.T) {
	skipBeforeV3(t)
	defineRefreshAccess(t, "6s")
	ctx := context.Background()

	// signedIn returns whether s is signed in as a record user.
	signedIn := func(t *testing.T, s *surrealdb.Session) bool {
		t.Helper()
		res, err := surrealdb.Query[any](ctx, s, "RETURN $session.rd", nil)
		require.NoError(t, err)
		return (*res)[0].Result != nil
	}
	// renewedAndRevoked waits until the pair is renewed, revokes it, and waits
	// until the first access token has expired.
	renewedAndRevoked := func(t *testing.T, start time.Time, first *surrealdb.Tokens,
		tokens func() *surrealdb.Tokens, revoke func(context.Context, surrealdb.Tokens) error) {
		t.Helper()
		require.Eventually(t, func() bool {
			current := tokens()
			return current != nil && current.Refresh != first.Refresh
		}, 5*time.Second, 50*time.Millisecond)
		require.NoError(t, revoke(ctx, *tokens()))
		time.Sleep(time.Until(start.Add(7 * time.Second)))
	}

	t.Run("default session", func(t *testing.T) {
		t.Parallel()
		db := newClient(t, testenv.GetSurrealDBWSURL())
		start := time.Now()
		first := signUpWithRefresh(t, db, "fork_revoke"+uniq())
		renewedAndRevoked(t, start, first, db.Tokens, db.Revoke)
		require.NotNil(t, whoAmI(t, db))

		fork, err := db.Fork(ctx)
		require.NoError(t, err)
		defer func() { _ = fork.Detach(ctx) }()
		assert.True(t, signedIn(t, fork))
	})

	t.Run("attached session", func(t *testing.T) {
		t.Parallel()
		db := newClient(t, testenv.GetSurrealDBWSURL())
		session, err := db.Attach(ctx)
		require.NoError(t, err)
		defer func() { _ = session.Detach(ctx) }()
		require.NoError(t, session.Use(ctx, refreshNS, refreshDB))
		start := time.Now()
		first, err := session.SignUpWithRefresh(ctx, map[string]any{
			"NS": refreshNS, "DB": refreshDB, "AC": "user", "user": "fork_revoke_session" + uniq(), "pass": "secret",
		})
		require.NoError(t, err)
		renewedAndRevoked(t, start, first, session.Tokens, session.Revoke)
		require.True(t, signedIn(t, session))

		fork, err := session.Fork(ctx)
		require.NoError(t, err)
		defer func() { _ = fork.Detach(ctx) }()
		assert.True(t, signedIn(t, fork))
	})
}

// Sessions run alongside Attach returns the Session object Attach returned.
func TestSessions_attachRace(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	db := mustNewWS("sessions", "race")
	defer db.Close(ctx)

	for range 300 {
		var (
			wg       sync.WaitGroup
			attached *surrealdb.Session
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			s, err := db.Attach(ctx)
			assert.NoError(t, err)
			attached = s
		}()
		go func() {
			defer wg.Done()
			_, err := db.Sessions(ctx)
			assert.NoError(t, err)
		}()
		wg.Wait()
		require.NotNil(t, attached)

		sessions, err := db.Sessions(ctx)
		require.NoError(t, err)
		require.Contains(t, sessions, attached)
		require.NoError(t, attached.Detach(ctx))
	}
}

// A session attached on the connection without the client is listed, but
// cannot be forked, as the client does not know its state.
func TestFork_untrackedSession(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	require.NoError(t, err)
	conf := connection.NewConfig(u)
	conf.Logger = nil
	con := gorillaws.New(conf)
	db, err := surrealdb.FromConnection(ctx, con)
	require.NoError(t, err)
	defer db.Close(ctx)

	id := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	var res connection.RPCResponse[any]
	require.NoError(t, connection.Call(con, ctx, &res, &connection.RPCRequest{
		Method: string(connection.Attach), Session: &id,
	}))

	sessions, err := db.Sessions(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, id, *sessions[0].ID())

	_, err = sessions[0].Fork(ctx)
	require.ErrorIs(t, err, constants.ErrForkUntracked)
	require.NoError(t, sessions[0].Detach(ctx))
}

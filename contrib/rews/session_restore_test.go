package rews_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

// newServerConnection connects a rews.Connection to the test server.
func newServerConnection(t *testing.T) (*rews.Connection[*gorillaws.Connection], *surrealdb.DB) {
	t.Helper()

	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	require.NoError(t, err)
	conf := connection.NewConfig(u)
	log := logger.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	conn := rews.New(func(ctx context.Context) (*gorillaws.Connection, error) {
		return gorillaws.New(conf), nil
	}, 20*time.Millisecond, conf.Unmarshaler, log)

	db, err := surrealdb.FromConnection(context.Background(), conn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	return conn, db
}

// dropAndAwaitReconnection closes the socket under conn, as a network failure
// would, and waits for the reconnection loop to replace and restore it.
func dropAndAwaitReconnection(t *testing.T, conn *rews.Connection[*gorillaws.Connection]) {
	t.Helper()

	old, err := conn.DropSocket(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return conn.CurrentConnection() != old && conn.IsConnected()
	}, 10*time.Second, 10*time.Millisecond)

	// The session is restored after the socket is replaced; give it a moment.
	time.Sleep(200 * time.Millisecond)
}

// TestReconnect_afterInvalidate checks that a session which signed out is not
// signed back in by a reconnection.
func TestReconnect_afterInvalidate(t *testing.T) {
	ctx := context.Background()
	conn, db := newServerConnection(t)

	_, err := db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, "INFO FOR ROOT", nil)
	require.NoError(t, err)

	require.NoError(t, db.Invalidate(ctx))
	_, err = surrealdb.Query[any](ctx, db, "INFO FOR ROOT", nil)
	require.Error(t, err, "signed out")

	dropAndAwaitReconnection(t, conn)

	_, err = surrealdb.Query[any](ctx, db, "INFO FOR ROOT", nil)
	require.Error(t, err, "a reconnection must not sign the session back in")
}

// TestReconnect_restoresRefreshSignin checks that a reconnection restores a
// session which signed in with a refresh token, and refreshes its access token
// when the reconnection happens after the access token has expired.
func TestReconnect_restoresRefreshSignin(t *testing.T) {
	ctx := context.Background()
	ns, database := "rews_refresh", "rews_refresh"

	root, err := testenv.New(ns, database, "user")
	require.NoError(t, err)
	defer root.Close(ctx)

	v, err := testenv.GetVersion(ctx, root)
	require.NoError(t, err)
	if !v.IsV3OrLater() {
		t.Skip("WITH REFRESH needs SurrealDB v3")
	}

	_, err = surrealdb.Query[any](ctx, root, `
		DEFINE TABLE OVERWRITE user SCHEMALESS PERMISSIONS FOR select WHERE id = $auth.id;
		DEFINE ACCESS OVERWRITE user_access ON DATABASE TYPE RECORD
			SIGNIN (SELECT * FROM user WHERE name = $user AND crypto::argon2::compare(pass, $pass))
			SIGNUP (CREATE user SET name = $user, pass = crypto::argon2::generate($pass))
			WITH REFRESH
			DURATION FOR TOKEN 1s, FOR SESSION 1h;
	`, nil)
	require.NoError(t, err)

	conn, db := newServerConnection(t)
	require.NoError(t, db.Use(ctx, ns, database))

	_, err = db.SignUpWithRefresh(ctx, map[string]any{
		"NS": ns, "DB": database, "AC": "user_access",
		"user": fmt.Sprintf("u-%s", uuid.Must(uuid.NewV4())), "pass": "secret",
	})
	require.NoError(t, err)

	whoami := func() any {
		res, err := surrealdb.Query[any](ctx, db, "RETURN $auth.id", nil)
		require.NoError(t, err)
		return (*res)[0].Result
	}
	me := whoami()
	require.NotNil(t, me)

	dropAndAwaitReconnection(t, conn)
	require.Equal(t, me, whoami(), "the session is authenticated again after reconnecting")

	before := conn.SessionToken()

	// Let the access token expire, then reconnect: it must be refreshed.
	// Expiry is checked in whole seconds, so wait past the next second boundary but one.
	time.Sleep(2200 * time.Millisecond)
	dropAndAwaitReconnection(t, conn)
	require.Equal(t, me, whoami(), "the session is refreshed after reconnecting with an expired token")

	after := conn.SessionToken()

	// Some servers accept a recently expired token within a leeway, in which case
	// there was nothing to refresh. Where the server refuses it, it must have been refreshed.
	probe, probeErr := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	require.NoError(t, probeErr)
	defer probe.Close(ctx)
	if authErr := probe.Authenticate(ctx, before); authErr != nil {
		require.NotEqual(t, before, after, "the refreshed access token is kept")
	} else {
		t.Log("the server still accepts the expired token; the refresh path was not exercised")
	}
}

package surrealdb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

const (
	refreshNS = "auth_refresh"
	refreshDB = "auth_refresh"
)

// defineRefreshAccess defines a record access method WITH REFRESH whose access
// tokens live for tokenDuration.
func defineRefreshAccess(t *testing.T, tokenDuration string) {
	t.Helper()
	ctx := context.Background()

	db := mustNewWS(refreshNS, refreshDB, "user")
	defer db.Close(ctx)

	_, err := surrealdb.Query[any](ctx, db, fmt.Sprintf(`
		DEFINE ACCESS OVERWRITE user ON DATABASE TYPE RECORD
			SIGNUP (CREATE type::record("user", $user) CONTENT { pass: crypto::argon2::generate($pass) })
			SIGNIN (SELECT * FROM type::record("user", $user) WHERE crypto::argon2::compare(pass, $pass))
			WITH REFRESH
			DURATION FOR TOKEN %s, FOR SESSION 1h;
	`, tokenDuration), nil)
	require.NoError(t, err)
}

func signUpWithRefresh(t *testing.T, db *surrealdb.DB, user string) *surrealdb.Tokens {
	t.Helper()
	tokens, err := db.SignUpWithRefresh(context.Background(), map[string]any{
		"NS": refreshNS, "DB": refreshDB, "AC": "user", "user": user, "pass": "secret",
	})
	require.NoError(t, err)
	require.NotNil(t, tokens)
	require.NotEmpty(t, tokens.Refresh)
	return tokens
}

// whoAmI returns the record the session is signed in as, or nil.
func whoAmI(t *testing.T, db *surrealdb.DB) any {
	t.Helper()
	res, err := surrealdb.Query[any](context.Background(), db, "RETURN $session.rd", nil)
	require.NoError(t, err)
	return (*res)[0].Result
}

func skipBeforeV3(t *testing.T) {
	t.Helper()
	if v := getVersion(t); !v.IsV3OrLater() {
		t.Skipf("SurrealDB %s has no refresh tokens (requires v3+)", v)
	}
}

func newClient(t *testing.T, endpoint string, opts ...surrealdb.Option) *surrealdb.DB {
	t.Helper()
	db, err := surrealdb.FromEndpointURLString(context.Background(), endpoint, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	require.NoError(t, db.Use(context.Background(), refreshNS, refreshDB))
	return db
}

func TestRefreshAndRevoke(t *testing.T) {
	skipBeforeV3(t)
	defineRefreshAccess(t, "1h")
	ctx := context.Background()

	for family, endpoint := range endpoints() {
		t.Run(family, func(t *testing.T) {
			db := newClient(t, endpoint)
			first := signUpWithRefresh(t, db, "refresh_"+family+uniq())
			assert.Equal(t, first, db.Tokens())

			second, err := db.Refresh(ctx, *first)
			require.NoError(t, err)
			assert.NotEqual(t, first.Refresh, second.Refresh)
			assert.Equal(t, second, db.Tokens())
			assert.NotNil(t, whoAmI(t, db), "the session is authenticated with the new pair")

			_, err = db.Refresh(ctx, *first)
			require.Error(t, err, "a refresh token can only be used once")

			require.NoError(t, db.Revoke(ctx, *second))
			assert.Nil(t, db.Tokens(), "a revoked pair is no longer renewed")
			assert.NotNil(t, whoAmI(t, db), "revoking leaves the session signed in")

			_, err = db.Refresh(ctx, *second)
			require.Error(t, err, "a revoked refresh token cannot be used")
		})
	}
}

func TestReset(t *testing.T) {
	ctx := context.Background()

	for family, endpoint := range endpoints() {
		t.Run(family, func(t *testing.T) {
			db := newClient(t, endpoint)
			_, err := db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
			require.NoError(t, err)
			require.NoError(t, db.Let(ctx, "greeting", "hello"))

			require.NoError(t, db.Reset(ctx))
			assert.Nil(t, db.Tokens())
			assertSignedOut(t, db)

			_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
			require.NoError(t, err)
			res, err := surrealdb.Query[[]any](ctx, db, "RETURN [session::ns(), session::db()]", nil)
			require.NoError(t, err)
			assert.Equal(t, []any{nil, nil}, (*res)[0].Result, "reset clears the namespace and database")

			require.NoError(t, db.Use(ctx, refreshNS, refreshDB))
			greeting, err := surrealdb.Query[any](ctx, db, "RETURN $greeting", nil)
			require.NoError(t, err)
			assert.Nil(t, (*greeting)[0].Result, "reset clears the variables")
		})
	}
}

// TestTokenRenewal signs in with access tokens which live for 3 seconds and
// checks that the client renews them before they expire.
func TestTokenRenewal(t *testing.T) {
	skipBeforeV3(t)
	defineRefreshAccess(t, "3s")

	for family, endpoint := range endpoints() {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			db := newClient(t, endpoint)
			first := signUpWithRefresh(t, db, "renew_"+family+uniq())

			// Renewal happens half way through a token's life, so two renewals
			// are due by now, and the first token has expired.
			time.Sleep(4 * time.Second)

			current := db.Tokens()
			require.NotNil(t, current)
			assert.NotEqual(t, first.Refresh, current.Refresh)
			assert.NotNil(t, whoAmI(t, db), "the session is still signed in")
		})
	}

	t.Run("off", func(t *testing.T) {
		db := newClient(t, testenv.GetSurrealDBWSURL(), surrealdb.WithTokenRenewal(false))
		signUpWithRefresh(t, db, "norenew"+uniq())
		assert.Nil(t, db.Tokens())
	})
}

func TestSession_refreshAndReset(t *testing.T) {
	skipBeforeV3(t)
	defineRefreshAccess(t, "3s")
	ctx := context.Background()

	db := newClient(t, testenv.GetSurrealDBWSURL())
	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer func() { _ = session.Detach(ctx) }()
	require.NoError(t, session.Use(ctx, refreshNS, refreshDB))

	first, err := session.SignUpWithRefresh(ctx, map[string]any{
		"NS": refreshNS, "DB": refreshDB, "AC": "user", "user": "session" + uniq(), "pass": "secret",
	})
	require.NoError(t, err)

	time.Sleep(4 * time.Second)
	renewed := session.Tokens()
	require.NotNil(t, renewed)
	assert.NotEqual(t, first.Refresh, renewed.Refresh, "the session renews its own pair")
	assert.Nil(t, db.Tokens(), "the default session holds no pair")

	next, err := session.Refresh(ctx, *renewed)
	require.NoError(t, err)
	require.NoError(t, session.Revoke(ctx, *next))
	assert.Nil(t, session.Tokens())

	require.NoError(t, session.Reset(ctx))
	_, err = surrealdb.Query[any](ctx, session, "RETURN 1", nil)
	require.Error(t, err, "a reset session is signed out")
}

func assertSignedOut(t *testing.T, db *surrealdb.DB) {
	t.Helper()
	_, err := surrealdb.Query[any](context.Background(), db, "RETURN 1", nil)
	require.Error(t, err, "a reset session is signed out")
}

func uniq() string {
	return fmt.Sprintf("_%d", time.Now().UnixNano())
}

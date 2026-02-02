package testenv

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// This file contains tests for TYPE RECORD access method WITH REFRESH behavior
// between SurrealDB v2.x and v3.x.
//
// WITH REFRESH is only valid for TYPE RECORD access methods (not TYPE BEARER).
//
// Findings:
//
// v2.6.0 (with --allow-experimental bearer_access):
//   - WITH REFRESH syntax IS accepted (no parse error)
//   - However, signin/signup still return string token (not object with refresh)
//   - The refresh token feature is NOT actually implemented in v2.6.0
//   - Without the experimental flag, WITH REFRESH causes parse error
//   - SignIn with "key" parameter set to the JWT token does NOT work (returns error)
//
// v3.0.0-beta.2:
//   - WITH REFRESH IS supported for TYPE RECORD access methods
//   - With WITH REFRESH, both signin and signup return object: {"access": "JWT...", "refresh": "surreal-refresh-..."}
//   - Without WITH REFRESH, signin/signup return string (JWT token)
//   - The "access" field contains the JWT token
//   - The "refresh" field contains a refresh token (format: "surreal-refresh-...")
//   - Refresh token can be used for subsequent signin via {"refresh": refreshToken} parameter
//   - Refresh signin also returns new {access, refresh} pair
//   - SignIn with "key" parameter set to the JWT token does NOT work (returns error)
//     The "key" parameter is for bearer access grants, not JWT tokens
//
// Note that the only scenario where SignIn/SignUp returns an object (instead of a string token)
// is SurrealDB v3 with TYPE RECORD access method combined with WITH REFRESH.
// Also note that refresh tokens use the "refresh" parameter, never "key" (which is for bearer grants only).

func TestBehavior_RecordAccessWithRefresh_v2(t *testing.T) {
	// Enable experimental bearer_access feature which may also enable WITH REFRESH
	db, cleanup := setupVersionTestWithArgs(t, "v2.6.0", "--allow-experimental", "bearer_access")
	defer cleanup()
	ctx := context.Background()

	// Sign in as root to set up the access method
	_, err := db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)

	err = db.Use(ctx, "test_ns", "test_db")
	require.NoError(t, err)

	// Get version for type::thing vs type::record
	v, err := GetVersion(ctx, db)
	require.NoError(t, err)
	recordFn := v.ThingOrRecordFn()

	// 1. Define user table
	_, err = surrealdb.Query[any](ctx, db, `DEFINE TABLE user SCHEMAFULL`, nil)
	require.NoError(t, err)

	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD password ON user TYPE string`, nil)
	require.NoError(t, err)

	// 2. Define record access method WITH REFRESH
	defineQuery := fmt.Sprintf(`
		DEFINE ACCESS user_access ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM %s("user", $user) WHERE crypto::argon2::compare(password, $pass)
			)
			SIGNUP (
				CREATE %s("user", $user) CONTENT {
					password: crypto::argon2::generate($pass)
				}
			)
			WITH REFRESH
	`, recordFn, recordFn)

	_, err = surrealdb.Query[any](ctx, db, defineQuery, nil)
	// With experimental flag, v2.6.0 accepts WITH REFRESH syntax
	require.NoError(t, err, "v2.6.0 with experimental flag should accept WITH REFRESH syntax")

	// 3. Sign up a user
	_, err = db.SignUp(ctx, surrealdb.Auth{
		Access:    "user_access",
		Namespace: "test_ns",
		Database:  "test_db",
		Username:  "testuser",
		Password:  "testpass",
	})
	require.NoError(t, err)

	// 4. Sign in - v2.6.0 accepts WITH REFRESH but still returns string token
	// (refresh functionality not actually implemented in v2.x)
	token, err := db.SignIn(ctx, surrealdb.Auth{
		Access:    "user_access",
		Namespace: "test_ns",
		Database:  "test_db",
		Username:  "testuser",
		Password:  "testpass",
	})
	require.NoError(t, err)
	require.IsType(t, "", token, "v2.6.0 WITH REFRESH still returns string token (not implemented)")

	// Verify token works with Authenticate
	err = db.Authenticate(ctx, token)
	require.NoError(t, err)

	// 5. Test if v2 supports using token as "key" parameter for signin
	// (similar to how v3 uses refresh token as "refresh" parameter)
	_, err = db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "user_access",
		"key": token,
	})
	// v2.6.0 does NOT support using JWT token as "key" parameter
	// This is expected to fail - the "key" parameter is for bearer access grants, not JWT tokens
	require.Error(t, err, "v2.6.0 should NOT support JWT token as 'key' parameter in SignIn")
}

func TestBehavior_RecordAccessWithRefresh_v3(t *testing.T) {
	wsURL, db, cleanup := setupVersionTestWithHTTPURL(t, "v3.0.0-beta.2")
	defer cleanup()
	ctx := context.Background()

	// Sign in as root to set up the access method
	_, err := db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)

	err = db.Use(ctx, "test_ns", "test_db")
	require.NoError(t, err)

	// Get version for type::thing vs type::record
	v, err := GetVersion(ctx, db)
	require.NoError(t, err)
	recordFn := v.ThingOrRecordFn()

	// 1. Define user table
	_, err = surrealdb.Query[any](ctx, db, `DEFINE TABLE user SCHEMAFULL`, nil)
	require.NoError(t, err)

	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD password ON user TYPE string`, nil)
	require.NoError(t, err)

	// 2. Define record access method WITH REFRESH
	defineQuery := fmt.Sprintf(`
		DEFINE ACCESS user_access ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM %s("user", $user) WHERE crypto::argon2::compare(password, $pass)
			)
			SIGNUP (
				CREATE %s("user", $user) CONTENT {
					password: crypto::argon2::generate($pass)
				}
			)
			WITH REFRESH
	`, recordFn, recordFn)

	_, err = surrealdb.Query[any](ctx, db, defineQuery, nil)
	if err != nil {
		t.Fatalf("v3.x should support WITH REFRESH for TYPE RECORD: %v", err)
	}

	// 3. Create a low-level connection because WITH REFRESH causes signup/signin
	// to return object {token, refresh} instead of just string token
	codec := surrealcbor.New()
	// wsURL is "ws://localhost:PORT/rpc", but gorillaws.Connect appends "/rpc"
	// so we need to strip it from the BaseURL
	baseURL := wsURL[:len(wsURL)-4] // Remove "/rpc" suffix
	conn := gorillaws.New(&connection.Config{
		BaseURL:     baseURL,
		Marshaler:   codec,
		Unmarshaler: codec,
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Set namespace and database on the low-level connection
	err = conn.Use(ctx, "test_ns", "test_db")
	require.NoError(t, err)

	// 4. Sign up a user using low-level connection
	signupData := map[string]any{
		"NS":   "test_ns",
		"DB":   "test_db",
		"AC":   "user_access",
		"user": "testuser",
		"pass": "testpass",
	}

	var signupResponse connection.RPCResponse[map[string]any]
	err = connection.Send(conn, ctx, &signupResponse, "signup", signupData)
	require.NoError(t, err, "signup should succeed")
	// Signup also returns {access, refresh} with WITH REFRESH

	// 5. Sign in using low-level connection.Send to capture the full response
	authData := map[string]any{
		"NS":   "test_ns",
		"DB":   "test_db",
		"AC":   "user_access",
		"user": "testuser",
		"pass": "testpass",
	}

	var signinResponse connection.RPCResponse[map[string]any]
	err = connection.Send(conn, ctx, &signinResponse, "signin", authData)
	require.NoError(t, err, "signin should succeed")

	result := *signinResponse.Result

	// v3 WITH REFRESH returns object with "access" (JWT token) and "refresh" fields
	// The field is named "access" and contains the JWT token
	token, hasToken := result["access"].(string)
	require.True(t, hasToken, "v3 WITH REFRESH should return 'access' field (JWT token)")
	require.NotEmpty(t, token, "access token should not be empty")

	refresh, hasRefresh := result["refresh"].(string)
	require.True(t, hasRefresh, "v3 WITH REFRESH should return 'refresh' field")
	require.NotEmpty(t, refresh, "refresh token should not be empty")

	// 6. Verify the main token (access) works with Authenticate()
	err = conn.Authenticate(ctx, token)
	require.NoError(t, err, "access token should work with Authenticate")

	// 7. Verify we can execute queries after authenticating
	var queryResponse connection.RPCResponse[any]
	err = connection.Send(conn, ctx, &queryResponse, "query", `RETURN "authenticated"`, nil)
	require.NoError(t, err)

	// 8. Verify refresh token can be used for SignIn (without username/password)
	// The refresh token is passed via the "refresh" parameter (not "key")
	refreshAuthData := map[string]any{
		"NS":      "test_ns",
		"DB":      "test_db",
		"AC":      "user_access",
		"refresh": refresh, // Use refresh token - no username/password needed
	}

	var refreshSigninResponse connection.RPCResponse[map[string]any]
	err = connection.Send(conn, ctx, &refreshSigninResponse, "signin", refreshAuthData)
	require.NoError(t, err, "refresh token signin should succeed")

	refreshResult := *refreshSigninResponse.Result

	// Refresh signin also returns {access, refresh} object with new tokens
	newToken, hasNewToken := refreshResult["access"].(string)
	require.True(t, hasNewToken, "refresh signin should return new 'access' token")
	require.NotEmpty(t, newToken, "new access token should not be empty")

	newRefresh, hasNewRefresh := refreshResult["refresh"].(string)
	require.True(t, hasNewRefresh, "refresh signin should return new 'refresh' token")
	require.NotEmpty(t, newRefresh, "new refresh token should not be empty")

	// 9. Verify the new token from refresh also works with Authenticate
	err = conn.Authenticate(ctx, newToken)
	require.NoError(t, err, "token from refresh signin should work with Authenticate")

	// 10. Verify we can execute queries with the new token
	err = connection.Send(conn, ctx, &queryResponse, "query", `RETURN "authenticated with refreshed token"`, nil)
	require.NoError(t, err)

	// 11. Test if v3 supports using access token (JWT) as "key" parameter for signin
	// (the "key" parameter is meant for bearer access grants, not JWT tokens)
	keyAuthData := map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "user_access",
		"key": token, // Try using JWT token as "key"
	}

	var keySigninResponse connection.RPCResponse[map[string]any]
	err = connection.Send(conn, ctx, &keySigninResponse, "signin", keyAuthData)
	// v3 should NOT support using JWT token as "key" parameter for TYPE RECORD access
	// The "key" parameter is for bearer access grants (surreal-bearer-...), not JWT tokens
	require.Error(t, err, "v3 should NOT support JWT token as 'key' parameter in SignIn for TYPE RECORD access")
}

// Baseline tests without WITH REFRESH for comparison

func TestBehavior_RecordAccessWithoutRefresh_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()
	ctx := context.Background()

	// Sign in as root to set up the access method
	_, err := db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)

	err = db.Use(ctx, "test_ns", "test_db")
	require.NoError(t, err)

	// Get version for type::thing vs type::record
	v, err := GetVersion(ctx, db)
	require.NoError(t, err)
	recordFn := v.ThingOrRecordFn()

	// 1. Define user table
	_, err = surrealdb.Query[any](ctx, db, `DEFINE TABLE user SCHEMAFULL`, nil)
	require.NoError(t, err)

	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD password ON user TYPE string`, nil)
	require.NoError(t, err)

	// 2. Define record access method WITHOUT REFRESH (baseline)
	defineQuery := fmt.Sprintf(`
		DEFINE ACCESS user_access ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM %s("user", $user) WHERE crypto::argon2::compare(password, $pass)
			)
			SIGNUP (
				CREATE %s("user", $user) CONTENT {
					password: crypto::argon2::generate($pass)
				}
			)
	`, recordFn, recordFn)

	_, err = surrealdb.Query[any](ctx, db, defineQuery, nil)
	require.NoError(t, err)

	// 3. Sign up a user
	_, err = db.SignUp(ctx, surrealdb.Auth{
		Access:    "user_access",
		Namespace: "test_ns",
		Database:  "test_db",
		Username:  "testuser",
		Password:  "testpass",
	})
	require.NoError(t, err)

	// 4. Sign in - without WITH REFRESH, returns string token
	token, err := db.SignIn(ctx, surrealdb.Auth{
		Access:    "user_access",
		Namespace: "test_ns",
		Database:  "test_db",
		Username:  "testuser",
		Password:  "testpass",
	})
	require.NoError(t, err)
	require.IsType(t, "", token, "v2 WITHOUT REFRESH should return string token")

	// Verify token works with Authenticate
	err = db.Authenticate(ctx, token)
	require.NoError(t, err)

	// 5. Test if v2 supports using token as "key" parameter for signin
	// (the "key" parameter is meant for bearer access grants, not JWT tokens)
	_, err = db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "user_access",
		"key": token,
	})
	// v2 should NOT support using JWT token as "key" parameter for TYPE RECORD access
	// The "key" parameter is for bearer access grants (surreal-bearer-...), not JWT tokens
	require.Error(t, err, "v2 should NOT support JWT token as 'key' parameter in SignIn for TYPE RECORD access")
}

func TestBehavior_RecordAccessWithoutRefresh_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()
	ctx := context.Background()

	// Sign in as root to set up the access method
	_, err := db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)

	err = db.Use(ctx, "test_ns", "test_db")
	require.NoError(t, err)

	// Get version for type::thing vs type::record
	v, err := GetVersion(ctx, db)
	require.NoError(t, err)
	recordFn := v.ThingOrRecordFn()

	// 1. Define user table
	_, err = surrealdb.Query[any](ctx, db, `DEFINE TABLE user SCHEMAFULL`, nil)
	require.NoError(t, err)

	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD password ON user TYPE string`, nil)
	require.NoError(t, err)

	// 2. Define record access method WITHOUT REFRESH (baseline)
	defineQuery := fmt.Sprintf(`
		DEFINE ACCESS user_access ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM %s("user", $user) WHERE crypto::argon2::compare(password, $pass)
			)
			SIGNUP (
				CREATE %s("user", $user) CONTENT {
					password: crypto::argon2::generate($pass)
				}
			)
	`, recordFn, recordFn)

	_, err = surrealdb.Query[any](ctx, db, defineQuery, nil)
	require.NoError(t, err)

	// 3. Sign up a user
	_, err = db.SignUp(ctx, surrealdb.Auth{
		Access:    "user_access",
		Namespace: "test_ns",
		Database:  "test_db",
		Username:  "testuser",
		Password:  "testpass",
	})
	require.NoError(t, err)

	// 4. Sign in - without WITH REFRESH, returns string token
	token, err := db.SignIn(ctx, surrealdb.Auth{
		Access:    "user_access",
		Namespace: "test_ns",
		Database:  "test_db",
		Username:  "testuser",
		Password:  "testpass",
	})
	require.NoError(t, err)
	require.IsType(t, "", token, "v3 WITHOUT REFRESH should return string token")

	// Verify token works with Authenticate
	err = db.Authenticate(ctx, token)
	require.NoError(t, err)

	// 5. Test if v3 supports using token as "key" parameter for signin
	// (the "key" parameter is meant for bearer access grants, not JWT tokens)
	_, err = db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "user_access",
		"key": token,
	})
	// v3 should NOT support using JWT token as "key" parameter for TYPE RECORD access
	// The "key" parameter is for bearer access grants (surreal-bearer-...), not JWT tokens
	require.Error(t, err, "v3 should NOT support JWT token as 'key' parameter in SignIn for TYPE RECORD access")
}

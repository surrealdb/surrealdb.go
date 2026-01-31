package testenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

// This file contains tests for TYPE BEARER access method FOR USER behavior
// between SurrealDB v2.x and v3.x.
//
// Bearer access methods allow generating bearer grants with an associated key
// that can be used to authenticate as a specific system user or record user.
//
// Findings from these tests:
//
// 1. Grant Result Structure (same for v2 and v3):
//    {
//      ac: "bearer_api",              // access method name
//      creation: {Time: ...},         // grant creation time
//      expiration: {Time: ...},       // grant expiration time (default 30 days)
//      grant: {
//        id: "abc123...",             // grant ID
//        key: "surreal-bearer-...",   // the bearer key for signin
//      },
//      id: "abc123...",               // same as grant.id
//      revocation: nil,               // null if not revoked
//      subject: {
//        user: "testuser"             // for system user grants
//      },
//      type: "bearer"
//    }
//
// 2. SignIn Response: Both v2 and v3 return a string JWT token
//
// 3. Experimental Flag:
//    - v2.x: requires --allow-experimental bearer_access
//    - v3.x: bearer access is enabled by default (no experimental flag needed)
//
// 4. Authenticate Behavior:
//    - Authenticate does NOT work with bearer-obtained JWT tokens
//    - Authenticate does NOT work with bearer keys directly
//    - Bearer keys can ONLY be used with SignIn
//
// Note: WITH REFRESH is NOT valid for TYPE BEARER access methods.
// WITH REFRESH only applies to TYPE RECORD access methods.
// See record_access_method_refresh_test.go for WITH REFRESH tests.

func TestBehavior_BearerAccessSystemUser_v2(t *testing.T) {
	// In v2.x, bearer access is an experimental feature that must be explicitly enabled
	db, cleanup := setupVersionTestWithArgs(t, "v2.6.0", "--allow-experimental", "bearer_access")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a database-level user
	_, err := surrealdb.Query[any](ctx, db, `DEFINE USER testuser ON DATABASE PASSWORD 'testpass' ROLES EDITOR`, nil)
	require.NoError(t, err)

	// 2. Define bearer access method for users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	require.NoError(t, err)

	// 3. Grant bearer key for the user (returns a single object, not an array)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER testuser`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 4. Extract bearer key from grant result (key is nested in grant.key)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 5. Sign in with bearer key - v2 returns string JWT token directly
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v2.x signin should return string token")

	// 6. SignIn established the authenticated session on the WebSocket connection.
	// Verify we can perform operations as the authenticated user.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin")

	// 7. Test if Authenticate works with the token from bearer signin
	// NOTE: In v2, Authenticate does NOT work with bearer-obtained JWT tokens for system users
	// Same behavior as v3 - parse error occurs
	err = db.Authenticate(ctx, token)
	require.Error(t, err, "v2 bearer signin token does NOT work with Authenticate for system users")

	// 8. Test if Authenticate works with the bearer key directly
	// Bearer keys are meant for SignIn, not Authenticate
	err = db.Authenticate(ctx, bearerKey)
	require.Error(t, err, "v2 bearer key should NOT work with Authenticate (bearer keys are for SignIn only)")
}

func TestBehavior_BearerAccessSystemUser_v3(t *testing.T) {
	// In v3.x, bearer access is enabled by default (no experimental flag needed)
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a database-level user
	_, err := surrealdb.Query[any](ctx, db, `DEFINE USER testuser ON DATABASE PASSWORD 'testpass' ROLES EDITOR`, nil)
	require.NoError(t, err)

	// 2. Define bearer access method for users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	require.NoError(t, err)

	// 3. Grant bearer key for the user (returns a single object, not an array)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER testuser`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 4. Extract bearer key from grant result (key is nested in grant.key, same as v2)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 5. Sign in with bearer key - v3 returns string JWT token (same as v2)
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v3.x signin should return string token")

	// 6. SignIn established the authenticated session. Verify we can perform operations.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin")

	// 7. Test if Authenticate works with the token from bearer signin
	// NOTE: In v3, Authenticate does NOT work with bearer-obtained JWT tokens for system users
	// This appears to be a limitation or intentional behavior in SurrealDB v3
	err = db.Authenticate(ctx, token)
	require.Error(t, err, "v3 bearer signin token does NOT work with Authenticate for system users")

	// 8. Test if Authenticate works with the bearer key directly
	// Bearer keys are meant for SignIn, not Authenticate
	err = db.Authenticate(ctx, bearerKey)
	require.Error(t, err, "v3 bearer key should NOT work with Authenticate (bearer keys are for SignIn only)")
}

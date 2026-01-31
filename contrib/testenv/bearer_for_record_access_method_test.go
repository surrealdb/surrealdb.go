package testenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

// This file contains tests for TYPE BEARER access method FOR RECORD behavior
// between SurrealDB v2.x and v3.x.
//
// Record user bearer grants work similarly to system user grants, but the
// subject contains a record reference instead of a user name:
//   subject: { record: { Table: "users", ID: "testrecord" } }
//
// Findings:
//
// v2.6.0 (with --allow-experimental bearer_access):
//   - Bearer access for records requires experimental flag
//   - SignIn with bearer key returns JWT token string
//   - Authenticate does NOT work with bearer-obtained JWT tokens
//   - Authenticate does NOT work with bearer keys directly
//   - Bearer keys can ONLY be used with SignIn
//
// v3.0.0-beta.2:
//   - Bearer access for records is enabled by default (no experimental flag)
//   - SignIn with bearer key returns JWT token string (same as v2)
//   - Authenticate does NOT work with bearer-obtained JWT tokens
//   - Authenticate does NOT work with bearer keys directly
//   - Bearer keys can ONLY be used with SignIn
//
// Note: WITH REFRESH is NOT valid for TYPE BEARER access methods.
// WITH REFRESH only applies to TYPE RECORD access methods.
// See record_access_method_refresh_test.go for WITH REFRESH tests.

func TestBehavior_BearerAccessRecordUser_v2(t *testing.T) {
	// In v2.x, bearer access is an experimental feature that must be explicitly enabled
	db, cleanup := setupVersionTestWithArgs(t, "v2.6.0", "--allow-experimental", "bearer_access")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a table for record users
	_, err := surrealdb.Query[any](ctx, db, `DEFINE TABLE users SCHEMAFULL`, nil)
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD name ON users TYPE string`, nil)
	require.NoError(t, err)

	// 2. Create a record user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:testrecord SET name = 'Test Record User'`, nil)
	require.NoError(t, err)

	// 3. Define bearer access method for records
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_record_api ON DATABASE TYPE BEARER FOR RECORD`, nil)
	require.NoError(t, err)

	// 4. Grant bearer key for the record (returns single object with grant.key)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_record_api GRANT FOR RECORD users:testrecord`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 5. Extract bearer key from grant result (key is nested in grant.key)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 6. Sign in with bearer key - v2 returns string JWT token
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_record_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v2.x signin should return string token")

	// 7. SignIn established the authenticated session. Verify we can perform operations.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin as record user")

	// 8. Test if Authenticate works with the token from bearer signin
	// NOTE: In v2, Authenticate does NOT work with bearer-obtained JWT tokens for record users
	// Error: "The access method cannot be used in the requested operation"
	err = db.Authenticate(ctx, token)
	require.Error(t, err, "v2 bearer signin token does NOT work with Authenticate for record users")

	// 9. Test if Authenticate works with the bearer key directly
	// Bearer keys are meant for SignIn, not Authenticate
	err = db.Authenticate(ctx, bearerKey)
	require.Error(t, err, "v2 bearer key should NOT work with Authenticate (bearer keys are for SignIn only)")
}

func TestBehavior_BearerAccessRecordUser_v3(t *testing.T) {
	// In v3.x, bearer access is enabled by default (no experimental flag needed)
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a table for record users
	_, err := surrealdb.Query[any](ctx, db, `DEFINE TABLE users SCHEMAFULL`, nil)
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD name ON users TYPE string`, nil)
	require.NoError(t, err)

	// 2. Create a record user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:testrecord SET name = 'Test Record User'`, nil)
	require.NoError(t, err)

	// 3. Define bearer access method for records
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_record_api ON DATABASE TYPE BEARER FOR RECORD`, nil)
	require.NoError(t, err)

	// 4. Grant bearer key for the record (returns single object with grant.key, same as v2)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_record_api GRANT FOR RECORD users:testrecord`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 5. Extract bearer key from grant result (key is nested in grant.key)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 6. Sign in with bearer key - v3 returns string JWT token (same as v2)
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_record_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v3.x signin should return string token")

	// 7. SignIn established the authenticated session. Verify we can perform operations.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin as record user")

	// 8. Test if Authenticate works with the token from bearer signin
	// NOTE: In v3, Authenticate does NOT work with bearer-obtained JWT tokens for record users
	// Error: "The access method cannot be used in the requested operation"
	err = db.Authenticate(ctx, token)
	require.Error(t, err, "v3 bearer signin token does NOT work with Authenticate for record users")

	// 9. Test if Authenticate works with the bearer key directly
	// Bearer keys are meant for SignIn, not Authenticate
	err = db.Authenticate(ctx, bearerKey)
	require.Error(t, err, "v3 bearer key should NOT work with Authenticate (bearer keys are for SignIn only)")
}

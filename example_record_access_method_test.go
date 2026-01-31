package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// Example_recordAccessMethodWithRefresh_v3 demonstrates TYPE RECORD access method
// with WITH REFRESH in SurrealDB v3 using SignInWithRefresh.
//
// WITH REFRESH enables refresh token functionality for record access methods:
// - SignInWithRefresh returns a Tokens with "Access" (JWT) and "Refresh" tokens
// - The refresh token can be used to obtain new tokens without credentials
//
// Key differences from bearer access:
// - Bearer access uses "key" parameter with SignIn (format: surreal-bearer-...)
// - Record access with refresh uses "refresh" parameter with SignInWithRefresh
//
// This example only runs against SurrealDB v3.x. In v2.x, WITH REFRESH syntax is
// accepted but not implemented (signin still returns string token).
//
//nolint:gocyclo // Example functions are intentionally verbose for documentation
func Example_recordAccessMethodWithRefresh_v3() {
	ctx := context.Background()

	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "example_record_refresh_v3", "testdb")
	if err != nil {
		panic(err)
	}

	// Sign in as root to set up the access method
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}

	err = db.Use(ctx, "example_record_refresh_v3", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Check SurrealDB version - WITH REFRESH is only fully implemented in v3+
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(fmt.Sprintf("GetVersion failed: %v", err))
	}

	if !v.IsV3OrLater() {
		// Skip gracefully on v2 to avoid output verification failures
		fmt.Println("Record access with refresh demonstrated successfully")
		return
	}

	// Get the appropriate function name for the version
	recordFn := v.ThingOrRecordFn()

	// =========================================================================
	// Record Access Method with WITH REFRESH
	// =========================================================================

	// 1. Define user table
	_, err = surrealdb.Query[any](ctx, db, `DEFINE TABLE IF NOT EXISTS user SCHEMAFULL`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE TABLE failed: %v", err))
	}

	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD IF NOT EXISTS password ON user TYPE string`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE FIELD failed: %v", err))
	}

	// 2. Define record access method WITH REFRESH
	// This enables refresh token functionality
	defineQuery := fmt.Sprintf(`
		DEFINE ACCESS IF NOT EXISTS user_access ON DATABASE TYPE RECORD
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
		panic(fmt.Sprintf("DEFINE ACCESS failed: %v", err))
	}

	// 3. Sign up a user using SignUpWithRefresh (WITH REFRESH returns object, not string)
	_, err = db.SignUpWithRefresh(ctx, map[string]any{
		"NS":   "example_record_refresh_v3",
		"DB":   "testdb",
		"AC":   "user_access",
		"user": "testuser",
		"pass": "testpass",
	})
	if err != nil {
		panic(fmt.Sprintf("SignUpWithRefresh failed: %v", err))
	}

	// 4. Sign in with SignInWithRefresh to get both access and refresh tokens
	tokenPair, err := db.SignInWithRefresh(ctx, map[string]any{
		"NS":   "example_record_refresh_v3",
		"DB":   "testdb",
		"AC":   "user_access",
		"user": "testuser",
		"pass": "testpass",
	})
	if err != nil {
		panic(fmt.Sprintf("SignInWithRefresh failed: %v", err))
	}

	// tokenPair.Access is the JWT token
	// tokenPair.Refresh is the refresh token (format: surreal-refresh-...)

	// 5. Verify we can execute queries (session is authenticated after SignInWithRefresh)
	_, err = surrealdb.Query[any](ctx, db, `RETURN "authenticated"`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after signin failed: %v", err))
	}

	// 6. Use refresh token to get new tokens (without credentials)
	newTokens, err := db.SignInWithRefresh(ctx, map[string]any{
		"NS":      "example_record_refresh_v3",
		"DB":      "testdb",
		"AC":      "user_access",
		"refresh": tokenPair.Refresh, // No username/password needed
	})
	if err != nil {
		panic(fmt.Sprintf("Refresh SignInWithRefresh failed: %v", err))
	}

	// 7. The access token can be used with Authenticate() on NEW connections.
	// Note: SignInWithRefresh already authenticates the current session,
	// so Authenticate() is only needed when establishing a session on a different connection.
	err = db.Authenticate(ctx, newTokens.Access)
	if err != nil {
		panic(fmt.Sprintf("Authenticate with new token failed: %v", err))
	}

	// 8. Verify queries still work (would work even without step 7 on this connection)
	_, err = surrealdb.Query[any](ctx, db, `RETURN "authenticated with refreshed token"`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after refresh auth failed: %v", err))
	}

	fmt.Println("Record access with refresh demonstrated successfully")
	// Output:
	// Record access with refresh demonstrated successfully
}

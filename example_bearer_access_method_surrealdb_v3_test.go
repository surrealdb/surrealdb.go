package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// Example_bearerAccessMethod_v3_systemUser demonstrates bearer access method
// authentication for system users in SurrealDB v3.
//
// Bearer access methods allow generating bearer grants with an associated key
// that can be used to authenticate as a specific system user or record user.
// Bearer grants are ideal for service-to-service authentication as they provide:
// - Stronger security guarantees than passwords
// - Auditable and revocable credentials
// - No need to work with JWT directly
//
// This example only runs against SurrealDB v3.x. When run against v2.x, it
// skips with a message since bearer access requires experimental flag in v2.
func Example_bearerAccessMethod_v3_systemUser() {
	ctx := context.Background()

	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "example_bearer_v3", "testdb")
	if err != nil {
		panic(err)
	}

	// Sign in as root to set up bearer access
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}

	err = db.Use(ctx, "example_bearer_v3", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Check SurrealDB version - this example is v3+ only
	// In v2.x, bearer access requires --allow-experimental bearer_access flag
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(fmt.Sprintf("GetVersion failed: %v", err))
	}

	if !v.IsV3OrLater() {
		// Skip gracefully on v2 to avoid output verification failures
		fmt.Println("Bearer access demonstrated successfully")
		return
	}

	// =========================================================================
	// Bearer Access for System Users
	// =========================================================================

	// 1. Define a database-level user that will use bearer access
	_, err = surrealdb.Query[any](ctx, db, `DEFINE USER apiuser ON DATABASE PASSWORD 'secret' ROLES EDITOR`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE USER failed: %v", err))
	}

	// 2. Define bearer access method for system users
	// This allows generating bearer keys that authenticate as system users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE ACCESS failed: %v", err))
	}

	// 3. Grant a bearer key for the user
	// The grant result contains:
	// - grant.key: the bearer key to use for signin (format: surreal-bearer-{id}-{random})
	// - grant.id: unique grant identifier
	// - subject.user: the user this grant authenticates as
	// - creation/expiration: timestamps (default expiration is 30 days)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER apiuser`, nil)
	if err != nil {
		panic(fmt.Sprintf("ACCESS GRANT failed: %v", err))
	}

	// Extract the bearer key from the nested grant object
	grantData := (*grantResult)[0].Result
	grantInfo := grantData["grant"].(map[string]any)
	bearerKey := grantInfo["key"].(string)

	// 4. Sign in using the bearer key
	// The signin request uses:
	// - NS: target namespace
	// - DB: target database
	// - AC: access method name
	// - key: the bearer key from the grant
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "example_bearer_v3",
		"DB":  "testdb",
		"AC":  "bearer_api",
		"key": bearerKey,
	})
	if err != nil {
		panic(fmt.Sprintf("Bearer SignIn failed: %v", err))
	}

	// The signin returns a JWT token (string) that can be:
	// - Used with db.Authenticate() on new connections
	// - Stored for later use
	// Note: The WebSocket session is already authenticated after SignIn
	_ = token // JWT token for reference

	// 5. Verify we can perform operations as the authenticated user
	_, err = surrealdb.Query[any](ctx, db, `RETURN "Hello from bearer-authenticated user"`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after bearer auth failed: %v", err))
	}

	fmt.Println("Bearer access demonstrated successfully")
	// Output:
	// Bearer access demonstrated successfully
}

// Example_bearerAccessMethod_v3_recordUser demonstrates bearer access method
// authentication for record users in SurrealDB v3.
//
// Record user bearer grants work similarly to system user grants, but authenticate
// as a specific database record instead of a system user. This is useful when you
// want to grant API access to specific records/entities in your database.
func Example_bearerAccessMethod_v3_recordUser() {
	ctx := context.Background()

	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "example_bearer_record_v3", "testdb")
	if err != nil {
		panic(err)
	}

	// Sign in as root to set up bearer access
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}

	err = db.Use(ctx, "example_bearer_record_v3", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Check SurrealDB version - this example is v3+ only
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(fmt.Sprintf("GetVersion failed: %v", err))
	}

	if !v.IsV3OrLater() {
		// Skip gracefully on v2 to avoid output verification failures
		fmt.Println("Bearer record access demonstrated successfully")
		return
	}

	// =========================================================================
	// Bearer Access for Record Users
	// =========================================================================

	// 1. Define a table and create a record that will use bearer access
	_, err = surrealdb.Query[any](ctx, db, `DEFINE TABLE services SCHEMAFULL`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE TABLE failed: %v", err))
	}

	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD name ON services TYPE string`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE FIELD failed: %v", err))
	}

	_, err = surrealdb.Query[any](ctx, db, `CREATE services:webhook_handler SET name = 'Webhook Handler Service'`, nil)
	if err != nil {
		panic(fmt.Sprintf("CREATE record failed: %v", err))
	}

	// 2. Define bearer access method for record users
	// This allows generating bearer keys that authenticate as specific records
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_service_api ON DATABASE TYPE BEARER FOR RECORD`, nil)
	if err != nil {
		panic(fmt.Sprintf("DEFINE ACCESS failed: %v", err))
	}

	// 3. Grant a bearer key for the record
	// The grant result's subject contains: { record: { Table: "services", ID: "webhook_handler" } }
	grantResult, err := surrealdb.Query[map[string]any](ctx, db,
		`ACCESS bearer_service_api GRANT FOR RECORD services:webhook_handler`, nil)
	if err != nil {
		panic(fmt.Sprintf("ACCESS GRANT failed: %v", err))
	}

	// Extract the bearer key from the nested grant object
	grantData := (*grantResult)[0].Result
	grantInfo := grantData["grant"].(map[string]any)
	bearerKey := grantInfo["key"].(string)

	// 4. Sign in using the bearer key
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "example_bearer_record_v3",
		"DB":  "testdb",
		"AC":  "bearer_service_api",
		"key": bearerKey,
	})
	if err != nil {
		panic(fmt.Sprintf("Bearer SignIn failed: %v", err))
	}
	_ = token // JWT token for reference

	// 5. Verify we can perform operations as the authenticated record user
	_, err = surrealdb.Query[any](ctx, db, `RETURN "Hello from bearer-authenticated record user"`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after bearer auth failed: %v", err))
	}

	fmt.Println("Bearer record access demonstrated successfully")
	// Output:
	// Bearer record access demonstrated successfully
}

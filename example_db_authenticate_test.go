package surrealdb_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// nolint:gocyclo // Example covers end-to-end JWT setup; splitting would reduce readability for docs
func ExampleDB_Authenticate_jwt_databaseLevelUser() {
	ctx := context.Background()

	// Generate ECDSA key pair using Go standard library
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate private key: %v", err))
	}

	// Extract public key and encode it to PEM format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal public key: %v", err))
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Connect to SurrealDB and authenticate as root
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "exampledb_authenticate_jwt", "testdb")
	if err != nil {
		panic(err)
	}

	// Sign in as root to set up the JWT access method
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}

	err = db.Use(ctx, "exampledb_authenticate_jwt", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Remove any existing access method first
	_, err = surrealdb.Query[any](ctx, db, `REMOVE ACCESS IF EXISTS jwt_access ON DATABASE`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing JWT access: %v", err))
	}

	// Define a JWT access method with the public key
	defineAccessQuery := fmt.Sprintf(`
		DEFINE ACCESS jwt_access ON DATABASE TYPE JWT
		ALGORITHM ES256 KEY '%s'
	`, string(publicKeyPEM))

	_, err = surrealdb.Query[any](ctx, db, defineAccessQuery, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to define JWT access: %v", err))
	}

	// Create a signed JWT token using the private key
	// Only the required claims for database-level JWT access
	// See: https://surrealdb.com/docs/surrealql/statements/define/access/jwt#using-tokens
	claims := jwt.MapClaims{
		"exp": time.Now().Add(1 * time.Hour).Unix(), // Token expiration
		"ac":  "jwt_access",                         // Access method name
		"ns":  "exampledb_authenticate_jwt",         // Namespace
		"db":  "testdb",                             // Database
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to sign JWT token: %v", err))
	}

	// Close the root connection
	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	// Create a new connection and authenticate with the JWT token
	db, err = surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	// Authenticate using the JWT token via the Authenticate method
	// The JWT contains ns and db claims, so we don't need to call Use() first
	err = db.Authenticate(ctx, signedToken)
	if err != nil {
		panic(fmt.Sprintf("Authenticate with JWT failed: %v", err))
	}

	// Verify authentication by performing a query
	results, err := surrealdb.Query[any](ctx, db, `SELECT * FROM type::thing("user", "test")`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after JWT authentication failed: %v", err))
	}

	if results == nil || len(*results) == 0 {
		panic("Expected query results after JWT authentication")
	}

	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	fmt.Println("JWT-based authentication completed successfully")

	// Output:
	// JWT-based authentication completed successfully
}

func ExampleDB_Authenticate_jwt_hs512_databaseLevelUser() {
	ctx := context.Background()

	// Generate a symmetric key for HS512 (HMAC-SHA512)
	// Use a strong random string as the symmetric key
	symmetricKeyString := "sNSYneezcr8kqphfOC6NwwraUHJCVAt0XjsRSNmssBaBRh3WyMa9TRfq8ST7fsU2H2kGiOpU4GbAF1bCiXmM1b3JGgleBzz7rsrz6VvYEM4q3CLkcO8CMBIlhwhzWmy8" //nolint:goconst // duplicated across examples intentionally for self-contained docs

	// Connect to SurrealDB and authenticate as root
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "exampledb_authenticate_jwt_hs512", "testdb")
	if err != nil {
		panic(err)
	}

	// Sign in as root to set up the JWT access method
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}

	err = db.Use(ctx, "exampledb_authenticate_jwt_hs512", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Remove any existing access method first
	_, err = surrealdb.Query[any](ctx, db, `REMOVE ACCESS IF EXISTS jwt_hs512 ON DATABASE`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing JWT access: %v", err))
	}

	// Define a JWT access method with HS512 and the symmetric key
	// See: https://surrealdb.com/docs/surrealql/statements/define/access/jwt#database
	defineAccessQuery := fmt.Sprintf(`
		DEFINE ACCESS jwt_hs512 ON DATABASE TYPE JWT
		ALGORITHM HS512 KEY '%s'
	`, symmetricKeyString)

	_, err = surrealdb.Query[any](ctx, db, defineAccessQuery, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to define JWT access: %v", err))
	}

	// Create a signed JWT token using the symmetric key
	// Only the required claims for database-level JWT access
	claims := jwt.MapClaims{
		"exp": time.Now().Add(1 * time.Hour).Unix(), // Token expiration
		"ac":  "jwt_hs512",                          // Access method name
		"ns":  "exampledb_authenticate_jwt_hs512",   // Namespace
		"db":  "testdb",                             // Database
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signedToken, err := token.SignedString([]byte(symmetricKeyString))
	if err != nil {
		panic(fmt.Sprintf("Failed to sign JWT token: %v", err))
	}

	// Close the root connection
	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	// Create a new connection and authenticate with the JWT token
	db, err = surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	// Authenticate using the JWT token via the Authenticate method
	// The JWT contains ns and db claims, so we don't need to call Use() first
	err = db.Authenticate(ctx, signedToken)
	if err != nil {
		panic(fmt.Sprintf("Authenticate with JWT failed: %v", err))
	}

	// Verify authentication by performing a query
	results, err := surrealdb.Query[any](ctx, db, `SELECT * FROM type::thing("user", "test")`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after JWT authentication failed: %v", err))
	}

	if results == nil || len(*results) == 0 {
		panic("Expected query results after JWT authentication")
	}

	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	fmt.Println("JWT HS512 authentication completed successfully")

	// Output:
	// JWT HS512 authentication completed successfully
}

// nolint:gocyclo // Example shows full flow for namespace-level JWT auth
func ExampleDB_Authenticate_jwt_hs512_namespaceLevelUser() {
	ctx := context.Background()

	// Symmetric key for HS512 (HMAC-SHA512)
	symmetricKeyString := "sNSYneezcr8kqphfOC6NwwraUHJCVAt0XjsRSNmssBaBRh3WyMa9TRfq8ST7fsU2H2kGiOpU4GbAF1bCiXmM1b3JGgleBzz7rsrz6VvYEM4q3CLkcO8CMBIlhwhzWmy8" //nolint:goconst // duplicated across examples intentionally for self-contained docs

	// Names for this test
	ns := "exampledb_authenticate_jwt_hs512_ns"
	accessName := "jwt_hs512_ns"

	// Connect to SurrealDB and authenticate as root
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, ns, "testdb")
	if err != nil {
		panic(err)
	}

	// Sign in as root to set up the JWT access method
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}

	// Select namespace (database may be required by helper; safe to set anyway)
	err = db.Use(ctx, ns, "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Remove any existing access method first (namespace level)
	_, err = surrealdb.Query[any](ctx, db, `REMOVE ACCESS IF EXISTS jwt_hs512_ns ON NAMESPACE`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing JWT access: %v", err))
	}

	// Define a JWT access method for namespace level with HS512
	defineAccessQuery := fmt.Sprintf(`
		DEFINE ACCESS %s ON NAMESPACE TYPE JWT
		ALGORITHM HS512 KEY '%s'
	`, accessName, symmetricKeyString)

	_, err = surrealdb.Query[any](ctx, db, defineAccessQuery, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to define JWT access: %v", err))
	}

	// Create a signed JWT token using the symmetric key (namespace-level claims)
	claims := jwt.MapClaims{
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"ac":  accessName,
		"ns":  ns,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signedToken, err := token.SignedString([]byte(symmetricKeyString))
	if err != nil {
		panic(fmt.Sprintf("Failed to sign JWT token: %v", err))
	}

	// Close the root connection
	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	// Create a new connection and authenticate with the JWT token
	db, err = surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	// Authenticate using the JWT token via the Authenticate method
	err = db.Authenticate(ctx, signedToken)
	if err != nil {
		panic(fmt.Sprintf("Authenticate with JWT failed: %v", err))
	}

	// For namespace-level tokens, select a database to run queries
	err = db.Use(ctx, ns, "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed after JWT auth: %v", err))
	}

	// Verify authentication by performing a query
	results, err := surrealdb.Query[any](ctx, db, `SELECT * FROM type::thing("user", "test")`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after JWT authentication failed: %v", err))
	}
	if results == nil || len(*results) == 0 {
		panic("Expected query results after JWT authentication")
	}

	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	fmt.Println("JWT HS512 namespace-level authentication completed successfully")

	// Output:
	// JWT HS512 namespace-level authentication completed successfully
}

// nolint:gocyclo // Example shows full flow for root-level JWT auth
func ExampleDB_Authenticate_jwt_hs512_rootLevelUser() {
	ctx := context.Background()

	symmetricKeyString := "sNSYneezcr8kqphfOC6NwwraUHJCVAt0XjsRSNmssBaBRh3WyMa9TRfq8ST7fsU2H2kGiOpU4GbAF1bCiXmM1b3JGgleBzz7rsrz6VvYEM4q3CLkcO8CMBIlhwhzWmy8" //nolint:goconst // duplicated across examples intentionally for self-contained docs
	accessName := "jwt_hs512_root"
	ns := "exampledb_authenticate_jwt_hs512_root"

	// Admin connection
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}
	db, err = testenv.Init(db, ns, "testdb")
	if err != nil {
		panic(err)
	}
	if _, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"}); err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}
	err = db.Use(ctx, ns, "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}
	_, err = surrealdb.Query[any](ctx, db, `REMOVE ACCESS IF EXISTS jwt_hs512_root ON ROOT`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing JWT access: %v", err))
	}

	defineAccessQuery := fmt.Sprintf(`
		DEFINE ACCESS %s ON ROOT TYPE JWT
		ALGORITHM HS512 KEY '%s'
	`, accessName, symmetricKeyString)
	_, err = surrealdb.Query[any](ctx, db, defineAccessQuery, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to define JWT access: %v", err))
	}

	// Root-level token claims (no ns/db)
	claims := jwt.MapClaims{
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"ac":  accessName,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signedToken, err := token.SignedString([]byte(symmetricKeyString))
	if err != nil {
		panic(fmt.Sprintf("Failed to sign JWT token: %v", err))
	}

	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	// Authenticate with root-level token
	db, err = surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}
	err = db.Authenticate(ctx, signedToken)
	if err != nil {
		panic(fmt.Sprintf("Authenticate with JWT failed: %v", err))
	}
	// Choose ns/db for subsequent queries
	err = db.Use(ctx, ns, "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed after JWT auth: %v", err))
	}
	results, err := surrealdb.Query[any](ctx, db, `SELECT * FROM type::thing("user", "test")`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query after JWT authentication failed: %v", err))
	}
	if results == nil || len(*results) == 0 {
		panic("Expected query results after JWT authentication")
	}
	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	fmt.Println("JWT HS512 root-level authentication completed successfully")

	// Output:
	// JWT HS512 root-level authentication completed successfully
}

func ExampleDB_Authenticate_jwt_hs512_rootLevelUser_expired() {
	ctx := context.Background()

	symmetricKeyString := "sNSYneezcr8kqphfOC6NwwraUHJCVAt0XjsRSNmssBaBRh3WyMa9TRfq8ST7fsU2H2kGiOpU4GbAF1bCiXmM1b3JGgleBzz7rsrz6VvYEM4q3CLkcO8CMBIlhwhzWmy8" //nolint:goconst // duplicated across examples intentionally for self-contained docs
	accessName := "jwt_hs512_root_expired"

	// Admin connection
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}
	// Use a dedicated namespace for this test
	ns := "exampledb_authenticate_jwt_hs512_root_expired"
	db, err = testenv.Init(db, ns, "testdb")
	if err != nil {
		panic(err)
	}
	if _, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"}); err != nil {
		panic(fmt.Sprintf("SignIn as root failed: %v", err))
	}
	err = db.Use(ctx, ns, "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}
	_, err = surrealdb.Query[any](ctx, db, `REMOVE ACCESS IF EXISTS jwt_hs512_root_expired ON ROOT`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing JWT access: %v", err))
	}
	defineAccessQuery := fmt.Sprintf(`
		DEFINE ACCESS %s ON ROOT TYPE JWT
		ALGORITHM HS512 KEY '%s'
	`, accessName, symmetricKeyString)
	_, err = surrealdb.Query[any](ctx, db, defineAccessQuery, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to define JWT access: %v", err))
	}

	// Expired token (exp in the past)
	claims := jwt.MapClaims{
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"ac":  accessName,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signedToken, err := token.SignedString([]byte(symmetricKeyString))
	if err != nil {
		panic(fmt.Sprintf("Failed to sign JWT token: %v", err))
	}

	if closeErr := db.Close(ctx); closeErr != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", closeErr))
	}

	// Try authenticating with expired token - should fail
	db, err = surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(err)
	}
	err = db.Authenticate(ctx, signedToken)
	if err != nil {
		// Expected failure path
		fmt.Println("JWT HS512 root-level expired authentication failed as expected")
		return
	}

	// If we reached here, authentication incorrectly succeeded
	panic("Expected Authenticate to fail with expired token, but it succeeded")

	// Output:
	// JWT HS512 root-level expired authentication failed as expected
}

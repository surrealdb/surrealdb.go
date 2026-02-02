package surrealdb_test

import (
	"context"
	"fmt"
	"log"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// ExampleDB_Attach demonstrates creating and using an additional session.
// Sessions allow independent authentication, namespace selection, and variable scope.
// This feature requires SurrealDB v3+ and WebSocket connections.
func ExampleDB_Attach() {
	// Skip if not v3+ (this is for documentation purposes)
	ctx := context.Background()

	// Connect using WebSocket (sessions require WebSocket)
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	// Sign in as root on the main connection
	if _, err := db.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
		log.Fatal(err)
	}

	// Create an additional session
	session, err := db.Attach(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Detach(ctx)

	fmt.Printf("Session created with ID: %s\n", session.ID())

	// The session starts unauthenticated - sign in and select namespace/database
	if _, err := session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
		log.Fatal(err)
	}

	if err := session.Use(ctx, "test", "test"); err != nil {
		log.Fatal(err)
	}

	// Set a session-scoped variable
	if err := session.Let(ctx, "user_id", "user123"); err != nil {
		log.Fatal(err)
	}

	// Query using the session - the variable is available
	type Result struct {
		UserID string `json:"user_id"`
	}
	results, err := surrealdb.Query[Result](ctx, session, "RETURN $user_id", nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(*results) > 0 {
		fmt.Printf("Session variable $user_id: %s\n", (*results)[0].Result)
	}

	// Note: This example requires SurrealDB v3+ and will fail on earlier versions.
	// Output is not verified because session IDs are dynamic.
}

// ExampleSession_Begin demonstrates starting a transaction within a session.
// Transactions within sessions are isolated and can be committed or cancelled.
func ExampleSession_Begin() {
	ctx := context.Background()

	// Connect using WebSocket
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	// Sign in and set up
	if _, err := db.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
		log.Fatal(err)
	}
	if err := db.Use(ctx, "test", "test"); err != nil {
		log.Fatal(err)
	}

	// Create a session
	session, err := db.Attach(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Detach(ctx)

	// Authenticate and configure the session
	if _, err := session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
		log.Fatal(err)
	}
	if err := session.Use(ctx, "test", "test"); err != nil {
		log.Fatal(err)
	}

	// Start a transaction within the session
	tx, err := session.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Transaction started with ID: %s\n", tx.ID())
	fmt.Printf("Transaction is in session: %s\n", tx.SessionID())

	// Perform operations in the transaction
	// ... your operations here ...

	// Commit or cancel
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Transaction committed successfully")

	// Note: This example requires SurrealDB v3+ and will fail on earlier versions.
	// Output is not verified because transaction IDs are dynamic.
}

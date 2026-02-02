package surrealdb_test

import (
	"context"
	"fmt"
	"log"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// ExampleDB_Begin demonstrates starting an interactive transaction.
// Interactive transactions allow executing statements one at a time
// and conditionally committing or cancelling based on results.
// This feature requires SurrealDB v3+ and WebSocket connections.
func ExampleDB_Begin() {
	ctx := context.Background()

	// Connect using WebSocket (transactions require WebSocket)
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	// Sign in and select namespace/database
	if _, err := db.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
		log.Fatal(err)
	}
	if err := db.Use(ctx, "test", "test"); err != nil {
		log.Fatal(err)
	}

	// Start an interactive transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// Always clean up if not committed
	defer func() {
		if !tx.IsClosed() {
			_ = tx.Cancel(ctx)
		}
	}()

	fmt.Printf("Transaction started with ID: %s\n", tx.ID())

	// Perform operations within the transaction
	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Stock int     `json:"stock"`
	}

	// Create a product
	_, err = surrealdb.Query[[]Product](ctx, tx,
		"CREATE products:widget SET name = 'Widget', stock = 100", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Query within the same transaction - changes are visible
	results, err := surrealdb.Query[[]Product](ctx, tx,
		"SELECT * FROM products:widget", nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(*results) > 0 && len((*results)[0].Result) > 0 {
		fmt.Printf("Product in transaction: %s (stock: %d)\n",
			(*results)[0].Result[0].Name,
			(*results)[0].Result[0].Stock)
	}

	// Commit the transaction to persist changes
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Transaction committed")

	// Note: This example requires SurrealDB v3+ and will fail on earlier versions.
	// Output is not verified because transaction IDs are dynamic.
}

// ExampleTransaction_conditionalCommit demonstrates conditional commit/cancel.
// Based on query results, you can decide whether to commit or rollback.
func ExampleTransaction_conditionalCommit() {
	ctx := context.Background()

	// Connect using WebSocket
	db, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	// Sign in and configure
	if _, err := db.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
		log.Fatal(err)
	}
	if err := db.Use(ctx, "test", "test"); err != nil {
		log.Fatal(err)
	}

	// Start transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Simulate a business operation: deduct from inventory
	type Inventory struct {
		Stock int `json:"stock"`
	}

	// Check current stock
	results, err := surrealdb.Query[[]Inventory](ctx, tx,
		"SELECT stock FROM inventory:item1", nil)
	if err != nil {
		_ = tx.Cancel(ctx)
		log.Fatal(err)
	}

	var currentStock int
	if len(*results) > 0 && len((*results)[0].Result) > 0 {
		currentStock = (*results)[0].Result[0].Stock
	}

	requestedQuantity := 5

	// Conditional logic based on query results
	if currentStock >= requestedQuantity {
		// Update stock
		_, err = surrealdb.Query[any](ctx, tx,
			"UPDATE inventory:item1 SET stock -= $qty",
			map[string]any{"qty": requestedQuantity})
		if err != nil {
			_ = tx.Cancel(ctx)
			log.Fatal(err)
		}

		// Commit the transaction
		if err := tx.Commit(ctx); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Transaction committed: inventory updated")
	} else {
		// Not enough stock - cancel transaction
		if err := tx.Cancel(ctx); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Transaction cancelled: insufficient stock")
	}

	// Output depends on inventory state
}

// ExampleTransaction_isolation demonstrates transaction isolation.
// Changes made in a transaction are not visible to other connections
// until the transaction is committed.
func ExampleTransaction_isolation() {
	ctx := context.Background()

	// Create two connections
	db1, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db1.Close(ctx)

	db2, err := surrealdb.FromEndpointURLString(ctx, testenv.GetSurrealDBWSURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db2.Close(ctx)

	// Configure both connections
	for _, db := range []*surrealdb.DB{db1, db2} {
		if _, err := db.SignIn(ctx, map[string]any{"user": "root", "pass": "root"}); err != nil {
			log.Fatal(err)
		}
		if err := db.Use(ctx, "test", "test"); err != nil {
			log.Fatal(err)
		}
	}

	// Start transaction on db1
	tx, err := db1.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if !tx.IsClosed() {
			_ = tx.Cancel(ctx)
		}
	}()

	// Create record in transaction
	_, err = surrealdb.Query[any](ctx, tx,
		"CREATE items:isolated SET value = 'hidden'", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Query from db2 - should NOT see uncommitted data
	type Item struct {
		Value string `json:"value"`
	}
	results, err := surrealdb.Query[[]Item](ctx, db2,
		"SELECT * FROM items:isolated", nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(*results) > 0 && len((*results)[0].Result) == 0 {
		fmt.Println("Before commit: db2 cannot see uncommitted data")
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}

	// Now db2 should see the data
	results, err = surrealdb.Query[[]Item](ctx, db2,
		"SELECT * FROM items:isolated", nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(*results) > 0 && len((*results)[0].Result) > 0 {
		fmt.Println("After commit: db2 can see committed data")
	}

	// Note: This example requires SurrealDB v3+ and will fail on earlier versions.
}

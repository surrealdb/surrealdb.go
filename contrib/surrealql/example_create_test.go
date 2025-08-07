package surrealql_test

import (
	"context"
	"fmt"
	"log"
	"maps"
	"slices"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

func ExampleCreate() {
	// Create a new user
	query := surrealql.Create("users").
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Set("created_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Return("id, name, email")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: CREATE users CONTENT $content_1 RETURN id, name, email
	// Vars: map[content_1:map[created_at:2023-10-01 12:00:00 +0000 UTC email:john@example.com name:John Doe]]
}

func ExampleCreate_withThing() {
	// Create a new user with a specific ID
	query := surrealql.Create(surrealql.Thing("users", 123)).
		Set("name", "Alice").
		Set("email", "alice@example.com").
		Set("created_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Return("id, name, email")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	keys := slices.Collect(maps.Keys(vars))
	slices.Sort(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: CREATE $id_1 CONTENT $content_1 RETURN id, name, email
	// Var content_1: map[created_at:2023-10-01 12:00:00 +0000 UTC email:alice@example.com name:Alice]
	// Var id_1: {users 123}
}

// ExampleCreate_integration_f_recordID demonstrates creating a record with a specific RecordID using the query builder.
// It shows how to use the `surrealql.T` function to specify the RecordID.
func ExampleCreate_integration_thing_recordID() {
	// This example shows how to use the query builder with surrealdb.Query

	// Assume we have a *surrealdb.DB instance
	var db *surrealdb.DB

	db, err := testenv.New("test", "users")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Create a new user with a specific ID
	query := surrealql.Create(surrealql.Thing("users", 123)).
		Set("name", "Alice").
		Set("email", "alice@example.com").
		Set("created_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Return("id, name, email")

	sql, vars := query.Build()

	results, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	if err != nil {
		log.Fatal(err)
	}

	users := (*results)[0].Result

	fmt.Println("Results:")
	for i, user := range users {
		fmt.Printf("  User %d:\n", i+1)

		keys := slices.Collect(maps.Keys(user))
		slices.Sort(keys)
		for _, key := range keys {
			fmt.Printf("    %s: %v\n", key, user[key])
		}
	}

	// Output:
	// Results:
	//   User 1:
	//     email: alice@example.com
	//     id: {users 123}
	//     name: Alice
}

// ExampleCreate_integration_f_table demonstrates creating a record in a table using the query builder.
// It shows how to use the `surrealql.T` function to specify the table.
// The ID is specified via the `Set` method, so that the record is created with a specific ID.
func ExampleCreate_integration_table() {
	var db *surrealdb.DB

	db, err := testenv.New("test", "users")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Create a new user with a specific ID
	query := surrealql.Create(surrealql.Table("users")).
		Set("id", 123).
		Set("name", "Alice").
		Set("email", "alice@example.com").
		Set("created_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Return("id, name, email")

	sql, vars := query.Build()

	results, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	if err != nil {
		log.Fatal(err)
	}

	users := (*results)[0].Result

	fmt.Println("Results:")
	for i, user := range users {
		fmt.Printf("  User %d:\n", i+1)

		keys := slices.Collect(maps.Keys(user))
		slices.Sort(keys)
		for _, key := range keys {
			fmt.Printf("    %s: %v\n", key, user[key])
		}
	}

	// Output:
	// Results:
	//   User 1:
	//     email: alice@example.com
	//     id: {users 123}
	//     name: Alice
}

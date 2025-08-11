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
	dumpVars(vars)

	// Output:
	// SurrealQL: CREATE users SET name = $param_1, email = $param_2, created_at = $param_3 RETURN id, name, email
	// Vars:
	//   param_1: John Doe
	//   param_2: john@example.com
	//   param_3: 2023-10-01 12:00:00 +0000 UTC
}

func ExampleCreate_compoundOperations() {
	// CREATE with compound operations using the Set function
	sql, vars := surrealql.Create("stats:daily").
		Set("date", "2024-01-01").
		Set("page_views", 0).
		Set("unique_visitors += ?", 1). // Compound operation in CREATE
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// CREATE stats:daily SET date = $param_1, page_views = $param_2, unique_visitors += $param_3
	// Vars:
	//   param_1: 2024-01-01
	//   param_2: 0
	//   param_3: 1
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
	dumpVars(vars)

	// Output:
	// SurrealQL: CREATE $id_1 SET name = $param_1, email = $param_2, created_at = $param_3 RETURN id, name, email
	// Vars:
	//   id_1: users:123
	//   param_1: Alice
	//   param_2: alice@example.com
	//   param_3: 2023-10-01 12:00:00 +0000 UTC
}

// ExampleCreate_integration_f_recordID demonstrates creating a record with a specific RecordID using the query builder.
// It shows how to use the `surrealql.T` function to specify the RecordID.
func ExampleCreate_integration_thing_recordID() {
	// This example shows how to use the query builder with surrealdb.Query

	// Assume we have a *surrealdb.DB instance
	var db *surrealdb.DB

	db, err := testenv.New("surrealql", "test", "users")
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

	db, err := testenv.New("surrealql", "test", "users")
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

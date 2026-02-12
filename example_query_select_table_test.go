package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_selectOnTable() {
	db := testenv.MustNew("surrealdbexamples", "query_select_on_table", "persons")

	type Person struct {
		ID   *models.RecordID `json:"id,omitempty"`
		Name string           `json:"name"`
	}

	// Seed two records
	_, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`CREATE persons:alice CONTENT {name: "Alice"}; CREATE persons:bob CONTENT {name: "Bob"}`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	// Use models.Table as a query variable to select all records in a table
	// Note: Directly embedding table names in query strings is prone to injection attacks.
	results, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`SELECT * FROM $table ORDER BY name`,
		map[string]any{
			"table": models.Table("persons"),
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of query results: %d\n", len(*results))
	fmt.Printf("First query result's status: %s\n", (*results)[0].Status)
	for _, p := range (*results)[0].Result {
		fmt.Printf("Person: %s (ID: %s)\n", p.Name, p.ID)
	}

	// Output:
	// Number of query results: 1
	// First query result's status: OK
	// Person: Alice (ID: persons:alice)
	// Person: Bob (ID: persons:bob)
}

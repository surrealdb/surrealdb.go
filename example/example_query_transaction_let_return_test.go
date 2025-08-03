package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_transaction_let_return() {
	db := testenv.MustNew("query", "t")

	createQueryResults, err := surrealdb.Query[[]any](
		context.Background(),
		db,
		`BEGIN;
		 CREATE t:1 SET name = 'test';
		 LET $i = SELECT * FROM $id;
		 RETURN $i.name;
		 COMMIT
		`,
		map[string]any{
			"id": models.NewRecordID("t", 1),
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of query results: %d\n", len(*createQueryResults))
	fmt.Printf("First query result's status: %+s\n", (*createQueryResults)[0].Status)
	fmt.Printf("Names contained in the first query result: %+v\n", (*createQueryResults)[0].Result)

	//nolint:lll
	// Output:
	// Number of query results: 1
	// First query result's status: OK
	// Names contained in the first query result: [test]
}

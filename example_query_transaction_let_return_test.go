package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

func ExampleQuery_transaction_let_return() {
	config := testenv.MustNewConfig("surrealdbexamples", "query", "t")
	db := config.MustNew()
	ctx := context.Background()

	createQueryResults, err := surrealdb.Query[[]any](
		ctx,
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

	// A transaction returns results for all statements (5 results).
	// The RETURN result is at index 3 (after BEGIN, CREATE, LET)
	returnResult := (*createQueryResults)[3].Result
	fmt.Printf("First query result's status: %+s\n", (*createQueryResults)[0].Status)
	fmt.Printf("Names contained in the RETURN result: %+v\n", returnResult)

	// Output:
	// First query result's status: OK
	// Names contained in the RETURN result: [test]
}

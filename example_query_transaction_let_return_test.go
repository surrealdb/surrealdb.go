package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_transaction_let_return() {
	config := testenv.MustNewConfig("surrealdbexamples", "query", "t")
	db := config.MustNew()
	ctx := context.Background()

	// Detect version to handle result format differences
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(err)
	}

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

	// Transaction result format changed between v2 and v3:
	// - v2.x: Returns only the RETURN result (1 result)
	// - v3.x: Returns results for all statements (5 results)
	var returnResult any
	if v.IsV3OrLater() {
		// In v3, the RETURN result is at index 3 (after BEGIN, CREATE, LET)
		returnResult = (*createQueryResults)[3].Result
	} else {
		// In v2, only the RETURN result is returned
		returnResult = (*createQueryResults)[0].Result
	}
	fmt.Printf("First query result's status: %+s\n", (*createQueryResults)[0].Status)
	fmt.Printf("Names contained in the RETURN result: %+v\n", returnResult)

	// Output:
	// First query result's status: OK
	// Names contained in the RETURN result: [test]
}

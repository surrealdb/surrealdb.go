package main

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleQueryReturn demonstrates how to use the RETURN NONE clause in a query.
// See https://github.com/surrealdb/surrealdb.go/issues/203 for more context.
//
//nolint:funlen
func ExampleQuery_return() {
	db := testenv.MustNew("query", "persons")

	type NestedStruct struct {
		City string `json:"city"`
	}

	type Person struct {
		ID           *models.RecordID `json:"id,omitempty"`
		Name         string           `json:"name"`
		NestedMap    map[string]any   `json:"nested_map,omitempty"`
		NestedStruct `json:"nested_struct,omitempty"`
		CreatedAt    models.CustomDateTime  `json:"created_at,omitempty"`
		UpdatedAt    *models.CustomDateTime `json:"updated_at,omitempty"`
	}

	createdAt, err := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	if err != nil {
		panic(err)
	}

	insertQueryResults, err := surrealdb.Query[any](
		context.Background(),
		db,
		`INSERT INTO persons [$content] RETURN NONE`,
		map[string]any{
			"content": map[string]any{
				"id":   "yusuke",
				"name": "Yusuke",
				"nested_struct": NestedStruct{
					City: "Tokyo",
				},
				"created_at": models.CustomDateTime{
					Time: createdAt,
				},
			},
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of insert query results: %d\n", len(*insertQueryResults))
	fmt.Printf("First insert query result's status: %+s\n", (*insertQueryResults)[0].Status)
	fmt.Printf("Results contained in the first query result: %+v\n", (*insertQueryResults)[0].Result)

	selectQueryResults, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`SELECT * FROM $id`, map[string]any{
			"id": models.NewRecordID("persons", "yusuke"),
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of select query results: %d\n", len(*selectQueryResults))
	fmt.Printf("First select query result's status: %+s\n", (*selectQueryResults)[0].Status)
	fmt.Printf("Persons contained in the first select query result: %+v\n", (*selectQueryResults)[0].Result)

	//nolint:lll
	// Output:
	// Number of insert query results: 1
	// First insert query result's status: OK
	// Results contained in the first query result: []
	// Number of select query results: 1
	// First select query result's status: OK
	// Persons contained in the first select query result: [{ID:persons:yusuke Name:Yusuke NestedMap:map[] NestedStruct:{City:Tokyo} CreatedAt:{Time:2023-10-01 12:00:00 +0000 UTC} UpdatedAt:<nil>}]
}

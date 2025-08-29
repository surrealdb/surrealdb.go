package surrealdb_test

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery() {
	db := testenv.MustNew("surrealdbexamples", "query", "persons")

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

	createQueryResults, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`CREATE type::thing($tb, $id) CONTENT $content`,
		map[string]any{
			"tb": "persons",
			"id": "yusuke",
			"content": map[string]any{
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
	fmt.Printf("Number of query results: %d\n", len(*createQueryResults))
	fmt.Printf("First query result's status: %+s\n", (*createQueryResults)[0].Status)
	fmt.Printf("Persons contained in the first query result: %+v\n", (*createQueryResults)[0].Result)

	//nolint:lll
	// Output:
	// Number of query results: 1
	// First query result's status: OK
	// Persons contained in the first query result: [{ID:persons:yusuke Name:Yusuke NestedMap:map[] NestedStruct:{City:Tokyo} CreatedAt:{Time:2023-10-01 12:00:00 +0000 UTC} UpdatedAt:<nil>}]
}

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

// The Query function's result type parameter should be varied according to the query.
// For example, SELECT ONLY returns a single record, not an array of records, and therefore
// the result type parameter should be a single type, not a slice type.
//
//nolint:staticcheck // for demonstration purpose
func ExampleQuery_only() {
	db := testenv.MustNew("surrealdbexamples", "query_only", "persons")

	type Person struct {
		ID   *models.RecordID `json:"id,omitempty"`
		Name string           `json:"name"`
	}

	recordID := models.NewRecordID("persons", "yusuke")

	// Note the type parameter is []Person
	createQueryResults, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`CREATE $record_id CONTENT {name: "Yusuke"}`,
		map[string]any{
			"record_id": recordID,
		},
	)
	if err != nil {
		panic(err)
	}
	var persons []Person = (*createQueryResults)[0].Result
	fmt.Printf("Persons contained in the first query result: %+v\n", persons)

	// Note the type parameter is Person, not []Person,
	// due to the ONLY keyword
	queryOnlyResults, err := surrealdb.Query[Person](
		context.Background(),
		db,
		`SELECT * FROM ONLY $record_id`,
		map[string]any{
			"record_id": recordID,
		},
	)
	if err != nil {
		panic(err)
	}
	var person Person = (*queryOnlyResults)[0].Result
	fmt.Printf("Person contained in the query only result: %+v\n", person)

	// Output:
	// Persons contained in the first query result: [{ID:persons:yusuke Name:Yusuke}]
	// Person contained in the query only result: {ID:persons:yusuke Name:Yusuke}
}

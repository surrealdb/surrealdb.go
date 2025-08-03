package main

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen
func ExampleQuery_embedded_struct() {
	db := testenv.MustNew("query", "persons")

	type Base struct {
		ID *models.RecordID `json:"id,omitempty"`
	}

	type Profile struct {
		Base
		City string `json:"city"`
	}

	type Person struct {
		Base
		Name      string `json:"name"`
		Profile   Profile
		CreatedAt models.CustomDateTime  `json:"created_at,omitempty"`
		UpdatedAt *models.CustomDateTime `json:"updated_at,omitempty"`
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
				"created_at": models.CustomDateTime{
					Time: createdAt,
				},
				"profile": map[string]any{
					"id":   models.NewRecordID("profiles", "yusuke"),
					"city": "Tokyo",
				},
			},
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of query results: %d\n", len(*createQueryResults))
	fmt.Printf("First query result's status: %+s\n", (*createQueryResults)[0].Status)
	fmt.Printf("Persons contained in the first query result: %+v\n", (*createQueryResults)[0].Result)

	updatedAt, err := time.Parse(time.RFC3339, "2023-10-02T12:00:00Z")
	if err != nil {
		panic(err)
	}
	updateQueryResults, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`UPDATE $id CONTENT $content`,
		map[string]any{
			"id": models.NewRecordID("persons", "yusuke"),
			"content": map[string]any{
				"name":       "Yusuke Updated Last",
				"created_at": createdAt,
				"updated_at": updatedAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of update query results: %d\n", len(*updateQueryResults))
	fmt.Printf("Update query result's status: %+s\n", (*updateQueryResults)[0].Status)
	fmt.Printf("Persons contained in the update query result: %+v\n", (*updateQueryResults)[0].Result)

	selectQueryResults, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`SELECT * FROM $id`,
		map[string]any{
			"id": models.NewRecordID("persons", "yusuke"),
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of select query results: %d\n", len(*selectQueryResults))
	fmt.Printf("Select query result's status: %+s\n", (*selectQueryResults)[0].Status)
	fmt.Printf("Persons contained in the select query result: %+v\n", (*selectQueryResults)[0].Result)

	//nolint:lll
	// Output:
	// Number of query results: 1
	// First query result's status: OK
	// Persons contained in the first query result: [{Base:{ID:persons:yusuke} Name:Yusuke Profile:{Base:{ID:profiles:yusuke} City:Tokyo} CreatedAt:{Time:2023-10-01 12:00:00 +0000 UTC} UpdatedAt:<nil>}]
	// Number of update query results: 1
	// Update query result's status: OK
	// Persons contained in the update query result: [{Base:{ID:persons:yusuke} Name:Yusuke Updated Last Profile:{Base:{ID:<nil>} City:} CreatedAt:{Time:2023-10-01 12:00:00 +0000 UTC} UpdatedAt:2023-10-02T12:00:00Z}]
	// Number of select query results: 1
	// Select query result's status: OK
	// Persons contained in the select query result: [{Base:{ID:persons:yusuke} Name:Yusuke Updated Last Profile:{Base:{ID:<nil>} City:} CreatedAt:{Time:2023-10-01 12:00:00 +0000 UTC} UpdatedAt:2023-10-02T12:00:00Z}]
}

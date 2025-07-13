package main

import (
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen
func ExampleInsert() {
	db := newSurrealDBWSConnection("query", "persons")

	type Person struct {
		Name string `json:"name"`
		// Note that you must use CustomDateTime instead of time.Time.
		CreatedAt models.CustomDateTime  `json:"created_at,omitempty"`
		UpdatedAt *models.CustomDateTime `json:"updated_at,omitempty"`
	}

	createdAt, err := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	if err != nil {
		panic(err)
	}

	inserted, err := surrealdb.Insert[Person](
		db,
		"persons",
		map[string]any{
			"name":       "First",
			"created_at": createdAt,
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Insert result: %+s\n", *inserted)

	_, err = surrealdb.Insert[struct{}](
		db,
		"persons",
		map[string]any{
			"name":       "Second",
			"created_at": createdAt,
		},
	)
	if err != nil {
		panic(err)
	}

	_, err = surrealdb.Insert[struct{}](
		db,
		"persons",
		Person{
			Name: "Third",
			CreatedAt: models.CustomDateTime{
				Time: createdAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}

	fourthAsMap, err := surrealdb.Insert[map[string]any](
		db,
		"persons",
		Person{
			Name: "Fourth",
			CreatedAt: models.CustomDateTime{
				Time: createdAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}
	if _, ok := (*fourthAsMap)[0]["id"].(models.RecordID); ok {
		delete((*fourthAsMap)[0], "id")
	}
	fmt.Printf("Insert result: %+s\n", *fourthAsMap)

	selected, err := surrealdb.Select[[]Person](
		db,
		"persons",
	)
	if err != nil {
		panic(err)
	}
	for _, person := range *selected {
		fmt.Printf("Selected person: %+s\n", person)
	}

	//nolint:lll
	// Unordered output:
	// Insert result: [{First {2023-10-01 12:00:00 +0000 UTC} <nil>}]
	// Insert result: [map[created_at:{2023-10-01 12:00:00 +0000 UTC} name:Fourth]]
	// Selected person: {First {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Second {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Third {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Fourth {2023-10-01 12:00:00 +0000 UTC} <nil>}
}

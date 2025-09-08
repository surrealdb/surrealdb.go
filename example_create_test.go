package surrealdb_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofrs/uuid"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen
func ExampleCreate() {
	db := testenv.MustNew("surrealdbexamples", "example_create", "persons")

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

	// Unlike Insert which returns a pointer to the array of inserted records,
	// Create returns a pointer to the record itself.
	var inserted *Person
	inserted, err = surrealdb.Create[Person](
		context.Background(),
		db,
		"persons",
		map[string]any{
			"name":       "First",
			"created_at": createdAt,
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Create result: %v\n", *inserted)

	// You can throw away the result if you don't need it,
	// by specifying an empty struct as the type parameter.
	_, err = surrealdb.Create[struct{}](
		context.Background(),
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

	// You can also create a record by passing a struct directly.
	_, err = surrealdb.Create[struct{}](
		context.Background(),
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

	// You can also receive the result as a map[string]any.
	// It should be handy when you don't want to define a struct type,
	// in other words, when the schema is not known upfront.
	var fourthAsMap *map[string]any
	fourthAsMap, err = surrealdb.Create[map[string]any](
		context.Background(),
		db,
		"persons",
		map[string]any{
			"name": "Fourth",
			"created_at": models.CustomDateTime{
				Time: createdAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}
	if _, ok := (*fourthAsMap)["id"].(models.RecordID); ok {
		delete((*fourthAsMap), "id")
	}
	fmt.Printf("Create result: %v\n", *fourthAsMap)

	selected, err := surrealdb.Select[[]Person](
		context.Background(),
		db,
		"persons",
	)
	if err != nil {
		panic(err)
	}
	for _, person := range *selected {
		fmt.Printf("Selected person: %v\n", person)
	}

	//nolint:lll
	// Unordered output:
	// Create result: {First {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Create result: map[created_at:{2023-10-01 12:00:00 +0000 UTC} name:Fourth]
	// Selected person: {First {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Second {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Third {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Fourth {2023-10-01 12:00:00 +0000 UTC} <nil>}
}

func ExampleCreate_server_unmarshal_error() {
	db := testenv.MustNew("surrealdbexamples", "query", "person")

	type Person struct {
		ID   models.RecordID `json:"id,omitempty"`
		Name string          `json:"name"`
	}

	_, err := surrealdb.Create[Person](
		context.Background(),
		db,
		"persons",
		Person{
			Name: "Test",
		},
	)
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}

	// Output:
	// Expected error: cannot marshal RecordID with empty table or ID: want <table>:<identifier> but got :<nil>
}

func ExampleCreate_recordID_withUUID() {
	db := testenv.MustNew("surrealdbexamples", "query", "person")

	type Person struct {
		ID   *models.RecordID `json:"id,omitempty"`
		Text string           `json:"text"`
	}

	// Create a UUIDv7 using gofrs/uuid
	u, _ := uuid.NewV7()
	u2 := models.UUID{UUID: u}

	// Create the record with UUIDv7 ID
	record := Person{
		Text: "Hello, SurrealDB with UUIDv7!",
	}

	created, err := surrealdb.Create[Person](
		context.Background(),
		db,
		models.NewRecordID("person", u2),
		record,
	)
	if err != nil {
		log.Fatal("Failed to create record:", err)
	}

	// The UUID in the ID field will vary, so we just check the table and text
	fmt.Printf("Created record with table '%s' and text: %s\n", created.ID.Table, created.Text)

	// Output:
	// Created record with table 'person' and text: Hello, SurrealDB with UUIDv7!
}

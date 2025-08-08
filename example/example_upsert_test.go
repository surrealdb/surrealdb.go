package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen
func ExampleUpsert() {
	db := testenv.MustNewDeprecated("query", "persons")

	type Person struct {
		ID   *models.RecordID `json:"id,omitempty"`
		Name string           `json:"name"`
		// Note that you must use CustomDateTime instead of time.Time.
		// See
		CreatedAt models.CustomDateTime  `json:"created_at,omitempty"`
		UpdatedAt *models.CustomDateTime `json:"updated_at,omitempty"`
	}

	createdAt, err := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	if err != nil {
		panic(err)
	}

	inserted, err := surrealdb.Upsert[Person](
		context.Background(),
		db,
		models.NewRecordID("persons", "yusuke"),
		map[string]any{
			"name":       "Yusuke",
			"created_at": createdAt,
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Insert via upsert result: %v\n", *inserted)

	updated, err := surrealdb.Upsert[Person](
		context.Background(),
		db,
		models.NewRecordID("persons", "yusuke"),
		map[string]any{
			"name": "Yusuke Updated",
			// because the upsert RPC is like UPSERT ~ CONTENT rather than UPSERT ~ MERGE,
			// the created_at field becomes None, which results in the returned created_at field being zero value.
			"updated_at": createdAt,
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Update via upsert result: %v\n", *updated)

	udpatedAt, err := time.Parse(time.RFC3339, "2023-10-02T12:00:00Z")
	if err != nil {
		panic(err)
	}
	updatedFurther, err := surrealdb.Upsert[Person](
		context.Background(),
		db,
		models.NewRecordID("persons", "yusuke"),
		map[string]any{
			"name":       "Yusuke Updated Further",
			"created_at": createdAt,
			"updated_at": models.CustomDateTime{
				Time: udpatedAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Update further via upsert result: %v\n", *updatedFurther)

	_, err = surrealdb.Upsert[struct{}](
		context.Background(),
		db,
		models.NewRecordID("persons", "yusuke"),
		map[string]any{
			"name": "Yusuke Updated Last",
		},
	)
	if err != nil {
		panic(err)
	}

	selected, err := surrealdb.Select[Person](
		context.Background(),
		db,
		models.NewRecordID("persons", "yusuke"),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected person: %v\n", *selected)

	//nolint:lll
	// Output:
	// Insert via upsert result: {persons:yusuke Yusuke {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Update via upsert result: {persons:yusuke Yusuke Updated {0001-01-01 00:00:00 +0000 UTC} 2023-10-01T12:00:00Z}
	// Update further via upsert result: {persons:yusuke Yusuke Updated Further {2023-10-01 12:00:00 +0000 UTC} 2023-10-02T12:00:00Z}
	// Selected person: {persons:yusuke Yusuke Updated Last {0001-01-01 00:00:00 +0000 UTC} <nil>}
}

func ExampleUpsert_unmarshal_error() {
	db := testenv.MustNewDeprecated("query", "person")

	type Person struct {
		Name string `json:"name"`
	}

	// This will fail because the record ID is not valid.
	_, err := surrealdb.Upsert[Person](
		context.Background(),
		db,
		models.Table("person"),
		map[string]any{
			// We are trying to set the name field to a number,
			// which is OK from the database's perspective,
			// because the table is schemaless for this example.
			//
			// However, we are trying to unmarshal the result into a struct
			// that expects the name field to be a string,
			// which will fail when the result is unmarshaled.
			"name": 123,
		},
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("Error is RPCError: %v\n", errors.Is(err, &surrealdb.RPCError{}))
	}

	// Output:
	// Error: Send: error unmarshaling result: cbor: cannot unmarshal array into Go value of type main.Person (cannot decode CBOR array to struct without toarray option)
	// Error is RPCError: false
}

func ExampleUpsert_rpc_error() {
	db := testenv.MustNewDeprecated("query", "person")

	type Person struct {
		Name string `json:"name"`
	}

	// For this example, we will define a SCHEMAFUL table
	// with a name field that is a string.
	// Trying to set the name field to a number
	// will result in an error from the database.

	if _, err := surrealdb.Query[any](
		context.Background(),
		db,
		`DEFINE TABLE person SCHEMAFUL;
		 DEFINE FIELD name ON person TYPE string;`,
		nil,
	); err != nil {
		panic(err)
	}

	// This will fail because the record ID is not valid.
	_, err := surrealdb.Upsert[Person](
		context.Background(),
		db,
		models.Table("person"),
		map[string]any{
			"id": models.NewRecordID("person", "a"),
			// Unlike ExampleUpsert_unmarshal_error,
			// this will fail on the database side
			// because the name field is defined as a string,
			// and we are trying to set it to a number.
			"name": 123,
		},
	)
	if err != nil {
		switch err.Error() {
		// As of v3.0.0-alpha.7
		case "There was a problem with the database: Couldn't coerce value for field `name` of `person:a`: Expected `string` but found `123`":
			fmt.Println("Encountered expected error for either v3.0.0-alpha.7 or v2.3.7")
			// As of v2.3.7
		case "There was a problem with the database: Found 123 for field `name`, with record `person:a`, but expected a string":
			fmt.Println("Encountered expected error for either v3.0.0-alpha.7 or v2.3.7")
		default:
			fmt.Printf("Unknown Error: %v\n", err)
		}
		fmt.Printf("Error is RPCError: %v\n", errors.Is(err, &surrealdb.RPCError{}))
	}

	// Output:
	// Encountered expected error for either v3.0.0-alpha.7 or v2.3.7
	// Error is RPCError: true
}

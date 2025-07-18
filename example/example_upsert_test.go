package main

import (
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen
func ExampleUpsert() {
	db := newSurrealDBWSConnection("query", "persons")

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
		db,
		models.NewRecordID("persons", "yusuke"),
		map[string]any{
			"name":       "Yusuke",
			"created_at": createdAt,
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Insert via upsert result: %+s\n", *inserted)

	updated, err := surrealdb.Upsert[Person](
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
	fmt.Printf("Update via upsert result: %+s\n", *updated)

	udpatedAt, err := time.Parse(time.RFC3339, "2023-10-02T12:00:00Z")
	if err != nil {
		panic(err)
	}
	updatedFurther, err := surrealdb.Upsert[Person](
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
	fmt.Printf("Update further via upsert result: %+s\n", *updatedFurther)

	_, err = surrealdb.Upsert[struct{}](
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
		db,
		models.NewRecordID("persons", "yusuke"),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected person: %+s\n", *selected)

	//nolint:lll
	// Output:
	// Insert via upsert result: {persons:yusuke Yusuke {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Update via upsert result: {persons:yusuke Yusuke Updated {0001-01-01 00:00:00 +0000 UTC} 2023-10-01T12:00:00Z}
	// Update further via upsert result: {persons:yusuke Yusuke Updated Further {2023-10-01 12:00:00 +0000 UTC} 2023-10-02T12:00:00Z}
	// Selected person: {persons:yusuke Yusuke Updated Last {0001-01-01 00:00:00 +0000 UTC} <nil>}
}

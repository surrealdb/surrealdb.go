package main

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen // ExampleUpdate demonstrates how to update records in SurrealDB.
func ExampleUpdate() {
	db := testenv.MustNewDeprecated("update", "persons")

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

	recordID := models.NewRecordID("persons", "yusuke")
	created, err := surrealdb.Create[Person](context.Background(), db, recordID, map[string]any{
		"name": "Yusuke",
		"nested_struct": NestedStruct{
			City: "Tokyo",
		},
		"created_at": models.CustomDateTime{
			Time: createdAt,
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created persons: %+v\n", *created)

	updatedAt, err := time.Parse(time.RFC3339, "2023-10-02T12:00:00Z")
	if err != nil {
		panic(err)
	}

	updated, err := surrealdb.Update[Person](context.Background(), db, recordID, map[string]any{
		"name": "Yusuke",
		"nested_map": map[string]any{
			"key1": "value1",
		},
		"nested_struct": NestedStruct{
			City: "Kagawa",
		},
		"updated_at": models.CustomDateTime{
			Time: updatedAt,
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Updated persons: %+v\n", *updated)

	//nolint:lll
	// Output:
	// Created persons: {ID:persons:yusuke Name:Yusuke NestedMap:map[] NestedStruct:{City:Tokyo} CreatedAt:{Time:2023-10-01 12:00:00 +0000 UTC} UpdatedAt:<nil>}
	// Updated persons: {ID:persons:yusuke Name:Yusuke NestedMap:map[key1:value1] NestedStruct:{City:Kagawa} CreatedAt:{Time:0001-01-01 00:00:00 +0000 UTC} UpdatedAt:2023-10-02T12:00:00Z}
}

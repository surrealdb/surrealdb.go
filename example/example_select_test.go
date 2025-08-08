package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleSelect() {
	db := testenv.MustNewDeprecated("update", "person")

	type Person struct {
		ID models.RecordID `json:"id,omitempty"`
	}

	a := Person{ID: models.NewRecordID("person", "a")}
	b := Person{ID: models.NewRecordID("person", "b")}

	for _, p := range []Person{a, b} {
		created, err := surrealdb.Create[Person](
			context.Background(),
			db,
			p.ID,
			map[string]any{},
		)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Created person: %+v\n", *created)
	}

	selectedOneUsingSelect, err := surrealdb.Select[Person](
		context.Background(),
		db,
		a.ID,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("selectedOneUsingSelect: %+v\n", *selectedOneUsingSelect)

	selectedMultiUsingSelect, err := surrealdb.Select[[]Person](
		context.Background(),
		db,
		"person",
	)
	if err != nil {
		panic(err)
	}
	for _, p := range *selectedMultiUsingSelect {
		fmt.Printf("selectedMultiUsingSelect: %+v\n", p)
	}

	// Output:
	// Created person: {ID:{Table:person ID:a}}
	// Created person: {ID:{Table:person ID:b}}
	// selectedOneUsingSelect: {ID:{Table:person ID:a}}
	// selectedMultiUsingSelect: {ID:{Table:person ID:a}}
	// selectedMultiUsingSelect: {ID:{Table:person ID:b}}
}

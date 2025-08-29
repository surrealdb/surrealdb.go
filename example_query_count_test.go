package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_count_groupAll() {
	db := testenv.MustNew("surrealdbexamples", "querytest", "product")

	type Product struct {
		ID       models.RecordID `json:"id,omitempty"`
		Name     string          `json:"name,omitempty"`
		Category string          `json:"category,omitempty"`
	}

	a := Product{
		ID:       models.NewRecordID("product", "a"),
		Name:     "A",
		Category: "One",
	}
	b := Product{
		ID:       models.NewRecordID("product", "b"),
		Name:     "B",
		Category: "One",
	}
	c := Product{
		ID:       models.NewRecordID("product", "c"),
		Name:     "C",
		Category: "Two",
	}

	for _, p := range []Product{a, b, c} {
		created, err := surrealdb.Create[Product](
			context.Background(),
			db,
			p.ID,
			p,
		)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Created product: %+v\n", *created)
	}

	type CountResult struct {
		C int `json:"c,omitempty"`
	}

	res, err := surrealdb.Query[[]CountResult](
		context.Background(),
		db,
		"SELECT COUNT() as c FROM product GROUP ALL",
		map[string]any{},
	)
	if err != nil {
		panic(err)
	}

	countResult := (*res)[0].Result[0]

	fmt.Printf("Count: %d\n", countResult.C)

	// Output:
	// Created product: {ID:{Table:product ID:a} Name:A Category:One}
	// Created product: {ID:{Table:product ID:b} Name:B Category:One}
	// Created product: {ID:{Table:product ID:c} Name:C Category:Two}
	// Count: 3
}

func ExampleQuery_count_groupBy() {
	db := testenv.MustNew("surrealdbexamples", "querytest", "product")

	type Product struct {
		ID       models.RecordID `json:"id,omitempty"`
		Name     string          `json:"name,omitempty"`
		Category string          `json:"category,omitempty"`
	}

	a := Product{
		ID:       models.NewRecordID("product", "a"),
		Name:     "A",
		Category: "One",
	}
	b := Product{
		ID:       models.NewRecordID("product", "b"),
		Name:     "B",
		Category: "One",
	}
	c := Product{
		ID:       models.NewRecordID("product", "c"),
		Name:     "C",
		Category: "Two",
	}

	for _, p := range []Product{a, b, c} {
		created, err := surrealdb.Create[Product](
			context.Background(),
			db,
			p.ID,
			p,
		)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Created product: %+v\n", *created)
	}

	type ProductCategorySummary struct {
		Category string `json:"category,omitempty"`
		Count    int    `json:"count,omitempty"`
	}

	res, err := surrealdb.Query[[]ProductCategorySummary](
		context.Background(),
		db,
		// Note that there's no `COUNT(*)` in SurrealDB.
		// When counting, you use either `COUNT()` or `COUNT(field)`,
		// with either GROUP BY or GROUP ALL.
		"SELECT category, COUNT() AS count FROM product GROUP BY category",
		map[string]any{},
	)
	if err != nil {
		panic(err)
	}

	summaries := (*res)[0].Result

	for i, summary := range summaries {
		fmt.Printf("Category %d: %s, Count: %d\n", i+1, summary.Category, summary.Count)
	}

	// Output:
	// Created product: {ID:{Table:product ID:a} Name:A Category:One}
	// Created product: {ID:{Table:product ID:b} Name:B Category:One}
	// Created product: {ID:{Table:product ID:c} Name:C Category:Two}
	// Category 1: One, Count: 2
	// Category 2: Two, Count: 1
}

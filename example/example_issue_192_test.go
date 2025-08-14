package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// See https://github.com/surrealdb/surrealdb.go/issues/292
func ExampleQuery_issue192() {
	db := testenv.MustNew("surrealdbexamples", "query_issue192", "t")

	_, err := surrealdb.Query[any](
		context.Background(),
		db,
		`DEFINE TABLE IF NOT EXISTS t SCHEMAFULL;
DEFINE FIELD IF NOT EXISTS modified_at2 ON TABLE t TYPE option<datetime>;
CREATE t:s`,
		map[string]any{"name": "John Doe"},
	)
	if err != nil {
		panic(err)
	}

	type ReturnData1 struct {
		ID         *models.RecordID      `json:"id,omitempty"`
		ModifiedAt models.CustomDateTime `json:"modified_at,omitempty"`
	}

	data, err := surrealdb.Query[[]ReturnData1](
		context.Background(),
		db,
		"SELECT id, modified_at FROM t",
		nil)
	if err != nil {
		panic(err)
	}

	got := (*data)[0].Result[0]

	fmt.Printf("ID: %s\n", got.ID)
	fmt.Printf("ModifiedAt: %v\n", got.ModifiedAt)
	fmt.Printf("ModifiedAt.IsZero(): %v\n", got.ModifiedAt.IsZero())

	type ReturnData2 struct {
		ID         *models.RecordID       `json:"id,omitempty"`
		ModifiedAt *models.CustomDateTime `json:"modified_at,omitempty"`
	}

	data2, err := surrealdb.Query[[]ReturnData2](
		context.Background(),
		db,
		"SELECT id, modified_at FROM t",
		nil)
	if err != nil {
		panic(err)
	}

	got2 := (*data2)[0].Result[0]

	fmt.Printf("ID: %s\n", got2.ID)
	// With fxamacker/cbor: returns zero-value struct (not nil)
	// With surrealcbor: returns nil
	// Both should print the same format for consistency
	if got2.ModifiedAt == nil || got2.ModifiedAt.IsZero() {
		fmt.Printf("ModifiedAt: <nil or zero>\n")
	}
	fmt.Printf("ModifiedAt.IsZero(): %v\n", got2.ModifiedAt.IsZero())

	// Output:
	// ID: t:s
	// ModifiedAt: {0001-01-01 00:00:00 +0000 UTC}
	// ModifiedAt.IsZero(): true
	// ID: t:s
	// ModifiedAt: <nil or zero>
	// ModifiedAt.IsZero(): true
}

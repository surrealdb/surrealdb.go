package surrealdb_test

import (
	"context"
	"errors"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// ExampleUpsert_server_error demonstrates extracting a *ServerError from
// an RPC error using errors.As.
//
// The error is still an *RPCError (backward compatible), and
// errors.As(err, &se) works because RPCError.Unwrap() returns a *ServerError.
func ExampleUpsert_server_error() {
	db := testenv.MustNew("surrealdbexamples", "server_error", "person")

	type Person struct {
		Name string `json:"name"`
	}

	if _, err := surrealdb.Query[any](
		context.Background(),
		db,
		`DEFINE TABLE person SCHEMAFUL;
		 DEFINE FIELD name ON person TYPE string;`,
		nil,
	); err != nil {
		panic(err)
	}

	_, err := surrealdb.Upsert[Person](
		context.Background(),
		db,
		models.Table("person"),
		map[string]any{
			"id":   models.NewRecordID("person", "a"),
			"name": 123,
		},
	)
	if err != nil {
		// Backward compat: RPCError is still matchable
		fmt.Printf("Error is RPCError: %v\n", errors.Is(err, &surrealdb.RPCError{}))

		// Extract ServerError for structured info
		fmt.Printf("Error is ServerError: %v\n", errors.Is(err, surrealdb.ServerError{}))
	}

	// Output:
	// Error is RPCError: true
	// Error is ServerError: true
}

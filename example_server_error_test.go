package surrealdb_test

import (
	"context"
	"errors"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleUpsert_server_error demonstrates extracting a *ServerError from
// an RPC error on SurrealDB v3 using errors.As.
//
// On SurrealDB v2, the error is still an *RPCError (backward compatible),
// but errors.As(err, &se) also works because RPCError.Unwrap() returns
// a *ServerError. On v2 servers, se.Kind will be empty.
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
		// v2 backward compat: RPCError is still matchable
		fmt.Printf("Error is RPCError: %v\n", errors.Is(err, &surrealdb.RPCError{}))

		// v3 migration path: extract ServerError for structured info
		fmt.Printf("Error is ServerError: %v\n", errors.Is(err, surrealdb.ServerError{}))
	}

	// Output:
	// Error is RPCError: true
	// Error is ServerError: true
}

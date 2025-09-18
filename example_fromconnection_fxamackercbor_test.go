package surrealdb_test

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// FromConnection can take any connection.Connection implementation with
// a custom connection.Config that can be used to specify a CBOR marshaler and unmarshaler.
// This example demonstrates how to explicitly use the legacy fxamacker/cbor implementation
// instead of the default surrealcbor implementation.
func ExampleFromConnection_alternativeCBORImpl_fxamackerCBOR() {
	conf := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	// To explicitly use the legacy fxamacker/cbor implementation,
	// override the default surrealcbor with fxamacker-based marshalers.
	// Note: fxamacker/cbor is deprecated in favor of surrealcbor.
	conf.Marshaler = &models.CborMarshaler{}     //nolint:staticcheck // Intentional use of deprecated type for example
	conf.Unmarshaler = &models.CborUnmarshaler{} //nolint:staticcheck // Intentional use of deprecated type for example

	conn := gws.New(conf)
	db, err := surrealdb.FromConnection(context.Background(), conn)
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "surrealdbexamples", "fxamackercbor", "user")
	if err != nil {
		panic(err)
	}

	// Define a sample struct
	type User struct {
		ID    *models.RecordID `json:"id,omitempty"`
		Name  string           `json:"name"`
		Email string           `json:"email"`
		// Note that with fxamacker/cbor you need to use models.CustomDateTime
		// instead of time.Time for proper datetime handling
		CreatedAt models.CustomDateTime `json:"created_at"`
	}

	// Create a user
	createdAt, _ := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	user := User{
		Name:      "Bob",
		Email:     "bob@example.com",
		CreatedAt: models.CustomDateTime{Time: createdAt},
	}

	// Insert the user
	created, err := surrealdb.Insert[User](context.Background(), db, "user", user)
	if err != nil {
		panic(err)
	}

	if created != nil && len(*created) > 0 {
		fmt.Printf("Created user: %s with email: %s\n", (*created)[0].Name, (*created)[0].Email)
	}

	// Output:
	// Created user: Bob with email: bob@example.com
}

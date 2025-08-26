package main

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// Example_surrealCBOR demonstrates how to use SurrealCBOR with SurrealDB Go SDK
// for efficient binary serialization.
func ExampleFromConnection_surrealCBOR() {
	conf := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	// To enable surrealcbor, instantiate the codec
	// and set it as the marshaler and unmarshaler.
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec
	conn := gws.New(conf)
	db, err := surrealdb.FromConnection(context.Background(), conn)
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "surrealdbexamples", "surrealcbor", "user")
	if err != nil {
		panic(err)
	}

	// Define a sample struct
	type User struct {
		ID    *models.RecordID `json:"id,omitempty"`
		Name  string           `json:"name"`
		Email string           `json:"email"`
		// Note that this had to be `CreatedAt models.CustomDateTime`
		// with the previous fxamacker/cbor-based implementation.
		CreatedAt time.Time `json:"created_at"`
	}

	// Create a user
	createdAt, _ := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	user := User{
		Name:      "Alice",
		Email:     "alice@example.com",
		CreatedAt: createdAt,
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
	// Created user: Alice with email: alice@example.com
}

package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_nonExistentRecord_fxamackercbor demonstrates how fxamacker/cbor
// handles non-existent records - it returns a struct with non-nil CustomNil{} for missing pointer fields
func ExampleSelect_nonExistentRecord_fxamackercbor() {
	c := testenv.MustNewConfig("example", "select", "user")
	c.CBORImpl = testenv.CBORImplFxamackerCBOR

	db := c.MustNew()

	type User struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Username string           `json:"username"`
		Email    string           `json:"email"`
	}

	// Try to select a record that doesn't exist
	user, err := surrealdb.Select[User](context.Background(), db, models.NewRecordID("user", "does_not_exist"))
	if err != nil {
		panic(err)
	}

	// With fxamacker/cbor, non-existent records return a struct where:
	// - Pointer fields that would be NONE become nil (behavior changed after v1.0.0)
	// - This example shows the current behavior with fxamacker
	fmt.Printf("User found: %t\n", user != nil)
	fmt.Printf("User.ID is nil: %t\n", user.ID == nil)
	fmt.Printf("User.ID type: %T\n", user.ID)
	fmt.Printf("User.Username: %q\n", user.Username)
	fmt.Printf("User.Email: %q\n", user.Email)

	// Output:
	// User found: true
	// User.ID is nil: true
	// User.ID type: *models.RecordID
	// User.Username: ""
	// User.Email: ""
}

// ExampleSelect_nonExistentRecord_surrealcbor demonstrates how surrealcbor
// handles non-existent records - it returns nil
func ExampleSelect_nonExistentRecord_surrealcbor() {
	c := testenv.MustNewConfig("example", "select", "user")
	c.CBORImpl = testenv.CBORImplSurrealCBOR

	db := c.MustNew()

	type User struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Username string           `json:"username"`
		Email    string           `json:"email"`
	}

	// Try to select a record that doesn't exist
	user, err := surrealdb.Select[User](context.Background(), db, models.NewRecordID("user", "does_not_exist"))
	if err != nil {
		panic(err)
	}

	// With surrealcbor, non-existent records return nil
	// - This is why tests like s.Require().Nil(user) pass
	fmt.Printf("User found: %t\n", user != nil)

	// Output:
	// User found: false
}

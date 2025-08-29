package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Send can be used to any SurrealDB RPC method including "select".
func ExampleSend_select() {
	db := testenv.MustNew("surrealdbexamples", "update", "person")

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

	var selectedUsingSendSelect connection.RPCResponse[Person]
	err := surrealdb.Send(
		context.Background(),
		db,
		&selectedUsingSendSelect,
		"select",
		a.ID,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("selectedUsingSendSelect: %+v\n", *selectedUsingSendSelect.Result)

	var selectedMultiUsingSendSelect connection.RPCResponse[[]Person]
	err = surrealdb.Send(
		context.Background(),
		db,
		&selectedMultiUsingSendSelect,
		"select",
		"person",
	)
	if err != nil {
		panic(err)
	}
	for _, p := range *selectedMultiUsingSendSelect.Result {
		fmt.Printf("selectedMultiUsingSendSelect: %+v\n", p)
	}

	var selectedOneUsingCustomSelect *Person
	selectedOneUsingCustomSelect, err = customSelect[Person](db, a.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("selectedOneUsingCustomSelect: %+v\n", *selectedOneUsingCustomSelect)

	var selectedMultiUsingCustomSelect *[]Person
	selectedMultiUsingCustomSelect, err = customSelect[[]Person](db, "person")
	if err != nil {
		panic(err)
	}
	for _, p := range *selectedMultiUsingCustomSelect {
		fmt.Printf("selectedMultiUsingCustomSelect: %+v\n", p)
	}

	// Output:
	// Created person: {ID:{Table:person ID:a}}
	// Created person: {ID:{Table:person ID:b}}
	// selectedUsingSendSelect: {ID:{Table:person ID:a}}
	// selectedMultiUsingSendSelect: {ID:{Table:person ID:a}}
	// selectedMultiUsingSendSelect: {ID:{Table:person ID:b}}
	// selectedOneUsingCustomSelect: {ID:{Table:person ID:a}}
	// selectedMultiUsingCustomSelect: {ID:{Table:person ID:a}}
	// selectedMultiUsingCustomSelect: {ID:{Table:person ID:b}}
}

func customSelect[TResult any, TWhat surrealdb.TableOrRecord](db *surrealdb.DB, what TWhat) (*TResult, error) {
	var res connection.RPCResponse[TResult]

	if err := surrealdb.Send(context.Background(), db, &res, "select", what); err != nil {
		return nil, err
	}

	return res.Result, nil
}

package main

import (
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

func ExampleQuery_transaction_return() {
	db := newSurrealDBWSConnection("query", "person")

	var err error

	var a *[]surrealdb.QueryResult[bool]
	a, err = surrealdb.Query[bool](
		db,
		`BEGIN; CREATE person:1; CREATE person:2; RETURN true; COMMIT;`,
		nil,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Status: %v\n", (*a)[0].Status)
	fmt.Printf("Result: %v\n", (*a)[0].Result)

	// Output:
	// Status: OK
	// Result: true
}

func ExampleQuery_transaction_throw() {
	db := newSurrealDBWSConnection("query", "person")

	var err error

	// Making b parameterized with `bool` type
	// would make it fail with `cannot unmarshal UTF-8 text string into Go struct field`
	// like reported in https://github.com/surrealdb/surrealdb.go/issues/175
	var b *[]surrealdb.QueryResult[any]
	b, err = surrealdb.Query[any](
		db,
		`BEGIN; THROW "test"; RETURN 1; COMMIT;`,
		nil,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Status: %v\n", (*b)[0].Status)
	fmt.Printf("Result: %v\n", (*b)[0].Result)

	// Output:
	// Status: ERR
	// Result: An error occurred: test
}

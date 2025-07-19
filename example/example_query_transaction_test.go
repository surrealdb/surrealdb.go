package main

import (
	"errors"
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

	var (
		queryResults *[]surrealdb.QueryResult[*int]
		err          error
	)

	// Up until v0.4.3, making QueryResult[T] parameterized with anything other than `any`
	// or `string` failed with:
	//   cannot unmarshal UTF-8 text string into Go struct field
	// in case the query was executed on the database but failed with an error.
	//
	// It was due to a mismatch between the expected type and the actual type-
	// The actual query result was a string, which provides the error message sent
	// from the database, regardless of the type parameter specified to the Query function.
	//
	// Since v0.4.4, the QueryResult was enhanced to set the Error field
	// to a QueryError if the query failed, allowing the caller to handle the error.
	// In that case, the Result field will be empty(or nil if it is a pointer type),
	// and the Status field will be set to "ERR".
	//
	// It's also worth noting that the returned error from the Query function
	// will be nil if the query was executed successfully, in which case all the results
	// have no Error field set.
	//
	// If the query failed, the returned error will be a `joinError` created by the `errors.Join` function,
	// which contains all the errors that occurred during the query execution.
	// The caller can check the Error field of each QueryResult to see if the query failed,
	// or check the returned error from the Query function to see if the query failed.
	queryResults, err = surrealdb.Query[*int](
		db,
		`BEGIN; THROW "test"; RETURN 1; COMMIT;`,
		nil,
	)
	fmt.Printf("# of results: %d\n", len(*queryResults))
	fmt.Println("=== Func error ===")
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Error is RPCError: %v\n", errors.Is(err, &surrealdb.RPCError{}))
	fmt.Printf("Error is QueryError: %v\n", errors.Is(err, &surrealdb.QueryError{}))
	for i, r := range *queryResults {
		fmt.Printf("=== QueryResult[%d] ===\n", i)
		fmt.Printf("Status: %v\n", r.Status)
		fmt.Printf("Result: %v\n", r.Result)
		fmt.Printf("Error: %v\n", r.Error)
		fmt.Printf("Error is RPCError: %v\n", errors.Is(r.Error, &surrealdb.RPCError{}))
		fmt.Printf("Error is QueryError: %v\n", errors.Is(r.Error, &surrealdb.QueryError{}))
	}

	// Output:
	// # of results: 2
	// === Func error ===
	// Error: An error occurred: test
	// The query was not executed due to a failed transaction
	// Error is RPCError: false
	// Error is QueryError: true
	// === QueryResult[0] ===
	// Status: ERR
	// Result: <nil>
	// Error: An error occurred: test
	// Error is RPCError: false
	// Error is QueryError: true
	// === QueryResult[1] ===
	// Status: ERR
	// Result: <nil>
	// Error: The query was not executed due to a failed transaction
	// Error is RPCError: false
	// Error is QueryError: true
}

package main

import (
	"context"
	"errors"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_transaction_return() {
	db := testenv.MustNew("surrealdbexamples", "query", "person")

	var err error

	var a *[]surrealdb.QueryResult[bool]
	a, err = surrealdb.Query[bool](
		context.Background(),
		db,
		`BEGIN; CREATE person:1; CREATE person:2; RETURN true; COMMIT;`,
		map[string]any{},
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
	db := testenv.MustNew("surrealdbexamples", "query", "person")

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
		context.Background(),
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

// See https://github.com/surrealdb/surrealdb.go/issues/177
func ExampleQuery_transaction_issue_177_return_before_commit() {
	db := testenv.MustNew("surrealdbexamples", "query", "t")

	var err error

	// Note that you are returning before committing the transaction.
	// In this case, you get the uncommitted result of the CREATE,
	// which lacks the ID field becase we aren't sure if the ID is committed or not
	// at that point.
	// SurrealDB may be enhanced to handle this, but for now,
	// you should commit the transaction before returning the result.
	// See the ExampleQuery_transaction_issue_177_commit function for the correct way to do this.
	queryResults, err := surrealdb.Query[any](context.Background(), db,
		`BEGIN;
		CREATE t:s SET name = 'test';
		LET $i = SELECT * FROM $id;
		RETURN $i;
		COMMIT;`,
		map[string]any{
			"id": models.RecordID{Table: "t", ID: "s"},
		})
	if err != nil {
		panic(err)
	}

	if len(*queryResults) != 1 {
		panic(fmt.Errorf("expected 1 query result, got %d", len(*queryResults)))
	}

	rs := (*queryResults)[0].Result.([]any)
	r := rs[0].(map[string]any)

	fmt.Printf("Status: %v\n", (*queryResults)[0].Status)
	fmt.Printf("r.name: %v\n", r["name"])
	if id := r["id"]; id != nil && id != (models.RecordID{Table: "t", ID: "s"}) {
		panic(fmt.Errorf("expected id to be empty for SurrealDB v3.0.0-alpha.7, or 's' for v2.3.7, got %v", id))
	}

	// Output:
	// Status: OK
	// r.name: test
}

// See https://github.com/surrealdb/surrealdb.go/issues/177
func ExampleQuery_transaction_issue_177_commit() {
	db := testenv.MustNew("surrealdbexamples", "query", "t")

	var err error

	queryResults, err := surrealdb.Query[any](context.Background(), db,
		`BEGIN;
		CREATE t:s SET name = 'test1';
		CREATE t:t SET name = 'test2';
		SELECT * FROM $id;
		COMMIT;`,
		map[string]any{
			"id": models.RecordID{Table: "t", ID: "s"},
		})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Status: %v\n", (*queryResults)[0].Status)

	if len(*queryResults) != 3 {
		panic(fmt.Errorf("expected 3 query results, got %d", len(*queryResults)))
	}

	var records []map[string]any
	for i, result := range *queryResults {
		if result.Status != "OK" {
			panic(fmt.Errorf("expected OK status for query result %d, got %s", i, result.Status))
		}
		if result.Result == nil {
			panic(fmt.Errorf("expected non-nil result for query result %d", i))
		}
		if record, ok := result.Result.([]any); ok && len(record) > 0 {
			records = append(records, record[0].(map[string]any))
		} else {
			panic(fmt.Errorf("expected result to be a slice of maps, got %T", result.Result))
		}
	}

	fmt.Printf("result[0].id: %v\n", records[0]["id"])
	fmt.Printf("result[0].name: %v\n", records[0]["name"])
	fmt.Printf("result[1].id: %v\n", records[1]["id"])
	fmt.Printf("result[1].name: %v\n", records[1]["name"])
	if id := records[2]["id"]; id != nil && id != (models.RecordID{Table: "t", ID: "s"}) {
		panic(fmt.Errorf("expected id to be empty for SurrealDB v3.0.0-alpha.7, or 's' for v2.3.7, got %v", id))
	}
	fmt.Printf("result[2].name: %v\n", records[2]["name"])

	// Output:
	// Status: OK
	// result[0].id: {t s}
	// result[0].name: test1
	// result[1].id: {t t}
	// result[1].name: test2
	// result[2].name: test1
}

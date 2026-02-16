package surrealdb_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_transaction_return() {
	config := testenv.MustNewConfig("surrealdbexamples", "query", "person")
	db := config.MustNew()
	ctx := context.Background()

	// Detect version to handle result format differences
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(err)
	}

	// Transaction result format changed between v2 and v3:
	// - v2.x: Returns only the RETURN result (1 result)
	// - v3.x: Returns results for all statements (5 results: BEGIN, CREATE, CREATE, RETURN, COMMIT)
	// For v3.x, use []any to avoid decode error when the type varies per result
	results, err := surrealdb.Query[any](
		ctx,
		db,
		`BEGIN; CREATE person:1; CREATE person:2; RETURN true; COMMIT;`,
		map[string]any{},
	)
	if err != nil {
		panic(err)
	}

	var resultBool bool
	if v.IsV3OrLater() {
		// In v3, the RETURN result is at index 3 (after BEGIN, CREATE, CREATE)
		resultBool = (*results)[3].Result.(bool)
	} else {
		// In v2, only the RETURN result is returned
		resultBool = (*results)[0].Result.(bool)
	}
	fmt.Printf("Status: %v\n", (*results)[0].Status)
	fmt.Printf("Result: %v\n", resultBool)

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

	// Normalize error messages for version compatibility
	// v2.x: "failed transaction"
	// v3.x: uses British spelling in error messages
	normalizeTransactionError := func(err error) string {
		if err == nil {
			return "<nil>"
		}
		s := err.Error()
		s = strings.ReplaceAll(s, "cancelled transaction", "failed transaction") //nolint:misspell
		s = strings.ReplaceAll(s, "canceled transaction", "failed transaction")
		return s
	}

	// Filter to only show ERR results (v3 adds OK results for BEGIN)
	var errResults []surrealdb.QueryResult[*int]
	for _, r := range *queryResults {
		if r.Status == "ERR" {
			errResults = append(errResults, r)
		}
	}

	fmt.Printf("# of ERR results: %d\n", len(errResults))
	fmt.Println("=== Func error ===")
	fmt.Printf("Error: %v\n", normalizeTransactionError(err))
	fmt.Printf("Error is ServerError: %v\n", errors.Is(err, &surrealdb.ServerError{}))
	for i, r := range errResults {
		fmt.Printf("=== QueryResult[%d] ===\n", i)
		fmt.Printf("Status: %v\n", r.Status)
		fmt.Printf("Result: %v\n", r.Result)
		fmt.Printf("Error: %v\n", normalizeTransactionError(r.Error))
		fmt.Printf("Error is ServerError: %v\n", errors.Is(r.Error, &surrealdb.ServerError{}))
	}

	// Output:
	// # of ERR results: 2
	// === Func error ===
	// Error: An error occurred: test
	// The query was not executed due to a failed transaction
	// Error is ServerError: true
	// === QueryResult[0] ===
	// Status: ERR
	// Result: <nil>
	// Error: An error occurred: test
	// Error is ServerError: true
	// === QueryResult[1] ===
	// Status: ERR
	// Result: <nil>
	// Error: The query was not executed due to a failed transaction
	// Error is ServerError: true
}

// See https://github.com/surrealdb/surrealdb.go/issues/177
func ExampleQuery_transaction_issue_177_return_before_commit() {
	config := testenv.MustNewConfig("surrealdbexamples", "query", "t")
	db := config.MustNew()
	ctx := context.Background()

	// Detect version to handle result format differences
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(err)
	}

	// Note that you are returning before committing the transaction.
	// In this case, you get the uncommitted result of the CREATE,
	// which lacks the ID field becase we aren't sure if the ID is committed or not
	// at that point.
	// SurrealDB may be enhanced to handle this, but for now,
	// you should commit the transaction before returning the result.
	// See the ExampleQuery_transaction_issue_177_commit function for the correct way to do this.
	queryResults, err := surrealdb.Query[any](ctx, db,
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

	// Transaction result format changed between v2 and v3:
	// - v2.x: Returns only the RETURN result (1 result)
	// - v3.x: Returns results for all statements (5 results: BEGIN, CREATE, LET, RETURN, COMMIT)
	var returnResult surrealdb.QueryResult[any]
	if v.IsV3OrLater() {
		// In v3, the RETURN result is at index 3 (after BEGIN, CREATE, LET)
		returnResult = (*queryResults)[3]
	} else {
		// In v2, only the RETURN result is returned
		returnResult = (*queryResults)[0]
	}

	rs := returnResult.Result.([]any)
	r := rs[0].(map[string]any)

	fmt.Printf("Status: %v\n", returnResult.Status)
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
	config := testenv.MustNewConfig("surrealdbexamples", "query", "t")
	db := config.MustNew()
	ctx := context.Background()

	// Detect version to handle result format differences
	v, err := testenv.GetVersion(ctx, db)
	if err != nil {
		panic(err)
	}

	queryResults, err := surrealdb.Query[any](ctx, db,
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

	// Transaction result format changed between v2 and v3:
	// - v2.x: Returns only statement results (3 results: CREATE, CREATE, SELECT)
	// - v3.x: Returns results for all statements (5 results: BEGIN, CREATE, CREATE, SELECT, COMMIT)
	// Extract only the statement results (CREATE, CREATE, SELECT)
	var statementResults []surrealdb.QueryResult[any]
	if v.IsV3OrLater() {
		// In v3, skip BEGIN (index 0) and COMMIT (last index)
		statementResults = (*queryResults)[1:4]
	} else {
		// In v2, all results are statement results
		statementResults = *queryResults
	}

	if len(statementResults) != 3 {
		panic(fmt.Errorf("expected 3 statement results, got %d", len(statementResults)))
	}

	var records []map[string]any
	for i, result := range statementResults {
		if result.Status != "OK" {
			panic(fmt.Errorf("expected OK status for statement result %d, got %s", i, result.Status))
		}
		if result.Result == nil {
			panic(fmt.Errorf("expected non-nil result for statement result %d", i))
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

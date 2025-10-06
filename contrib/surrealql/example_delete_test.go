package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleDelete() {
	// Delete expired sessions
	query := surrealql.Delete("sessions").
		Where("expires_at < ?", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: DELETE sessions WHERE expires_at < $param_1
	// Vars: map[param_1:2023-10-01 12:00:00 +0000 UTC]
}

func ExampleDelete_withReturnNone() {
	// Delete expired sessions without returning results
	query := surrealql.Delete("sessions").
		Where("expires_at < ?", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		ReturnNone()

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: DELETE sessions WHERE expires_at < $param_1 RETURN NONE
	// Vars: map[param_1:2023-10-01 12:00:00 +0000 UTC]
}

func ExampleDeleteOnly_withReturnBefore() {
	// Delete a specific record and return its state before deletion
	query := surrealql.DeleteOnly("users:123").
		ReturnBefore()

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: DELETE ONLY users:123 RETURN BEFORE
	// Vars: map[]
}

func ExampleDeleteOnly_withReturnAfter() {
	// Delete a specific record and return its state after deletion
	query := surrealql.DeleteOnly("users:123").
		ReturnAfter()

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: DELETE ONLY users:123 RETURN AFTER
	// Vars: map[]
}

func ExampleDelete_withWhereAndReturnDiff() {
	// Delete inactive users and return the difference
	query := surrealql.Delete("users").
		Where("active = ?", false).
		ReturnDiff()

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: DELETE users WHERE active = $param_1 RETURN DIFF
	// Vars: map[param_1:false]
}

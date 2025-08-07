package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

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

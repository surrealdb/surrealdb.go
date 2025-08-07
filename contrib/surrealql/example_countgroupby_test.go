package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleCountGroupBy() {
	// Count active users by role
	query := surrealql.CountGroupBy("role").
		FromTable("users").
		Where("active = ?", true).
		OrderByDesc("count")

	sql, _ := query.Build()
	fmt.Println("SurrealQL:", sql)
	// Output:
	// SurrealQL: SELECT role, count() AS count FROM users WHERE active = $param_1 GROUP BY role ORDER BY count DESC
}

package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleFunCall_ArgFromField() {
	// Example of using Fn with ArgFromField
	query := surrealql.Fn("math::sum").ArgFromField("amount")

	sql, _ := query.Build()
	fmt.Println("SurrealQL:", sql)
	// Output:
	// SurrealQL: math::sum(amount)
}

func ExampleFunCall_ArgFromValue() {
	// Example of using Fn with ArgFromValue
	query := surrealql.Fn("math::mean").ArgFromValue(42)

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: math::mean($fn_math_mean_0)
	// Var fn_math_mean_0: 42
}

func ExampleFunCall_ArgFromQuery() {
	// Example of using Fn with ArgFromQuery
	subQuery := surrealql.SelectValue("amount").FromTable("transactions").Where("status = ?", "completed")
	query := surrealql.Fn("math::sum").ArgFromQuery(subQuery)

	sql, _ := query.Build()
	fmt.Println("SurrealQL:", sql)
	// Output:
	// SurrealQL: math::sum(SELECT VALUE amount FROM transactions WHERE status = $param_1)
}

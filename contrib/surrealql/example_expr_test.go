package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

// dumpVarsInline prints all variables in ascending order by key on a single line
func dumpVarsInline(vars map[string]any) {
	if len(vars) == 0 {
		fmt.Println("Vars: (empty)")
		return
	}

	keys := slices.Collect(maps.Keys(vars))
	sort.Strings(keys)

	fmt.Print("Vars:")
	for _, key := range keys {
		fmt.Printf(" %s=%v", key, vars[key])
	}
	fmt.Println()
}

// ExampleExpr demonstrates using Field with aliasing
func ExampleExpr() {
	query := surrealql.Select("products").
		Fields(
			surrealql.Expr("count(*)").As("total"),
			surrealql.Expr("math::max(price)").As("max_price"),
			surrealql.Expr("math::min(price)").As("min_price"),
		)

	sql, _ := query.Build()
	fmt.Println(sql)
	// Output:
	// SELECT count(*) AS total, math::max(price) AS max_price, math::min(price) AS min_price FROM products
}

// ExampleExpr_withPlaceholders demonstrates Expr with placeholders and aliasing
func ExampleExpr_withPlaceholders() {
	query := surrealql.Select("dummy").Fields(
		surrealql.Expr("? * ? + ?", 10, 20, 5).As("calculation"),
		surrealql.Expr("math::mean([?,?,?])", 1, 2, 3).As("average"),
	)

	sql, vars := query.Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT $param_1 * $param_2 + $param_3 AS calculation, math::mean([$fn_math_mean_1,$fn_math_mean_2,$fn_math_mean_3]) AS average FROM dummy
	// Vars:
	//   fn_math_mean_1: 1
	//   fn_math_mean_2: 2
	//   fn_math_mean_3: 3
	//   param_1: 10
	//   param_2: 20
	//   param_3: 5
}

// ExampleExpr_withoutAlias demonstrates using Expr when you don't need aliasing
func ExampleExpr_withoutAlias() {
	// Expr is semantically clearer when you don't need aliasing
	query := surrealql.Select("users").Fields(
		surrealql.Expr("id"),
		surrealql.Expr("name"),
		surrealql.Expr("created_at"),
	)

	sql, _ := query.Build()
	fmt.Println(sql)
	// Output:
	// SELECT id, name, created_at FROM users
}

// ExampleExpr_vsExpr demonstrates the relationship between Field and Expr
func ExampleExpr_vsExpr() {
	// Both Field and Expr can be used in Select
	// Field is more semantic when you want to use As()
	// Expr is more semantic when you don't need aliasing

	query1 := surrealql.Select("users").Fields(
		surrealql.Expr("count(*)").As("total"), // Field with alias
		surrealql.Expr("name"),                 // Expr without alias
	)

	sql1, _ := query1.Build()
	fmt.Println("Mixed usage:")
	fmt.Println(sql1)

	// You can also use Expr with As() since it returns the same type
	query2 := surrealql.Select("users").Fields(
		surrealql.Expr("count(*)").As("total"), // Expr can also use As()
		surrealql.Expr("name"),
	)

	sql2, _ := query2.Build()
	fmt.Println("\nExpr with As():")
	fmt.Println(sql2)

	// Output:
	// Mixed usage:
	// SELECT count(*) AS total, name FROM users
	//
	// Expr with As():
	// SELECT count(*) AS total, name FROM users
}

// ExampleExpr_simpleFunction demonstrates using Expr for simple function calls
func ExampleExpr_simpleFunction() {
	query := surrealql.Select("products").Fields(
		surrealql.Expr("count(orders)").As("total_orders"),
		surrealql.Expr("math::max(price)").As("max_price"),
	)

	sql, _ := query.Build()
	fmt.Println(sql)
	// Output:
	// SELECT count(orders) AS total_orders, math::max(price) AS max_price FROM products
}

// ExampleExpr_withValues demonstrates using Expr with value placeholders
func ExampleExpr_withValues() {
	query := surrealql.Select("dummy").Fields(
		surrealql.Expr("math::mean([?,?,?])", 1, 2, 3).As("average"),
	)

	sql, vars := query.Build()
	fmt.Println(sql)
	dumpVarsInline(vars)
	// Output:
	// SELECT math::mean([$fn_math_mean_1,$fn_math_mean_2,$fn_math_mean_3]) AS average FROM dummy
	// Vars: fn_math_mean_1=1 fn_math_mean_2=2 fn_math_mean_3=3
}

// ExampleExpr_withVariable demonstrates using Expr with variable references
func ExampleExpr_withVariable() {
	query := surrealql.Select("test").Fields(
		surrealql.Expr("math::mean([1,?,3])", surrealql.Var("two")).As("result"),
	)

	sql, _ := query.Build()
	fmt.Println(sql)
	// Output:
	// SELECT math::mean([1,$two,3]) AS result FROM test
}

// ExampleExpr_withSubquery demonstrates using Expr with a subquery
func ExampleExpr_withSubquery() {
	subQuery := surrealql.Select("orders").Fields("price")

	query := surrealql.Select("dummy").Fields(
		surrealql.Expr("math::sum(?)", subQuery).As("total"),
	)

	sql, _ := query.Build()
	fmt.Println(sql)
	// Output:
	// SELECT math::sum((SELECT price FROM orders)) AS total FROM dummy
}

// ExampleExpr_expression demonstrates using Expr for general expressions
func ExampleExpr_expression() {
	query := surrealql.Select("test").Fields(
		surrealql.Expr("? + ? * 2", surrealql.Var("base"), 10).As("calculated"),
	)

	sql, vars := query.Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT $base + $param_1 * 2 AS calculated FROM test
	// Vars:
	//   param_1: 10
}

// ExampleExpr_selectFuncResultWithAlias demonstrates how to use Expr
// as select fields.
func ExampleExpr_selectFuncResultWithAlias() {
	subQuery := surrealql.Select("orders").Fields("total")

	// New way with Expr:
	query := surrealql.Select("dummy").Fields(
		surrealql.Expr("math::sum(?)", subQuery).As("total"),
	)

	sql, _ := query.Build()
	fmt.Println(sql)

	// Output:
	// SELECT math::sum((SELECT total FROM orders)) AS total FROM dummy
}

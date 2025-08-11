package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_modelsTable demonstrates using models.Table for type-safe table selection
func ExampleSelect_modelsTable() {
	// models.Table provides type safety and proper CBOR encoding for table names
	table := models.Table("users")
	sql, vars := surrealql.Select(table).Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output: SELECT * FROM $from_table_1
	// Vars:
	//   from_table_1: users
}

// ExampleSelect_modelsTableWithConditions demonstrates using models.Table with WHERE conditions
func ExampleSelect_modelsTableWithConditions() {
	// Combine models.Table with other query operations
	table := models.Table("products")
	sql, vars := surrealql.Select(table).
		FieldName("name").
		FieldName("price").
		Where("category = ? AND price > ?", "electronics", 100).
		OrderBy("price").
		Limit(10).
		Build()

	fmt.Println(sql)
	dumpVars(vars)

	// Output:
	// SELECT name, price FROM $from_table_1 WHERE category = $param_1 AND price > $param_2 ORDER BY price LIMIT 10
	// Vars:
	//   from_table_1: products
	//   param_1: electronics
	//   param_2: 100
}

// ExampleSelect_modelsTableDynamic demonstrates dynamic table selection with models.Table
func ExampleSelect_modelsTableDynamic() {
	// Useful when table name comes from configuration or user input
	// models.Table ensures proper type handling in SurrealDB
	getTableName := func() string {
		return "user_sessions"
	}

	table := models.Table(getTableName())
	sql, vars := surrealql.Select(table).
		Field("count() AS total").
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT count() AS total FROM $from_table_1
	// Vars:
	//   from_table_1: user_sessions
}

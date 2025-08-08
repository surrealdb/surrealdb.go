package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectFrom_modelsTable demonstrates using models.Table for type-safe table selection
func ExampleSelectFrom_modelsTable() {
	// models.Table provides type safety and proper CBOR encoding for table names
	table := models.Table("users")
	sql, vars := surrealql.SelectFrom(table).Build()

	fmt.Println(sql)
	fmt.Printf("vars: table_1=%v (type: %T)\n", vars["table_1"], vars["table_1"])
	// Output: SELECT * FROM $table_1
	// vars: table_1=users (type: models.Table)
}

// ExampleSelectFrom_modelsTableWithConditions demonstrates using models.Table with WHERE conditions
func ExampleSelectFrom_modelsTableWithConditions() {
	// Combine models.Table with other query operations
	table := models.Table("products")
	sql, vars := surrealql.SelectFrom(table).
		FieldName("name").
		FieldName("price").
		Where("category = ? AND price > ?", "electronics", 100).
		OrderBy("price").
		Limit(10).
		Build()

	fmt.Println(sql)
	fmt.Printf("Table: %v, Category: %v, MinPrice: %v\n",
		vars["table_1"], vars["param_1"], vars["param_2"])
	// Output: SELECT name, price FROM $table_1 WHERE category = $param_1 AND price > $param_2 ORDER BY price LIMIT 10
	// Table: products, Category: electronics, MinPrice: 100
}

// ExampleSelectFrom_modelsTableDynamic demonstrates dynamic table selection with models.Table
func ExampleSelectFrom_modelsTableDynamic() {
	// Useful when table name comes from configuration or user input
	// models.Table ensures proper type handling in SurrealDB
	getTableName := func() string {
		return "user_sessions"
	}

	table := models.Table(getTableName())
	sql, vars := surrealql.SelectFrom(table).
		FieldRaw("count() AS total").
		Build()

	fmt.Println(sql)
	fmt.Printf("Counting records in table: %v\n", vars["table_1"])
	// Output: SELECT count() AS total FROM $table_1
	// Counting records in table: user_sessions
}

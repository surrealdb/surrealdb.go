package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_table demonstrates selecting all from a table
func ExampleSelect_table() {
	sql, _ := surrealql.Select("users").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM users
}

// ExampleSelect_specificRecord demonstrates selecting from a specific record
func ExampleSelect_specificRecord() {
	sql, _ := surrealql.Select("users:123").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM users:123
}

// ExampleSelect_withFields demonstrates adding specific fields to Select
func ExampleSelect_withFields() {
	sql, _ := surrealql.Select("users").
		FieldName("name").
		FieldName("email").
		Build()
	fmt.Println(sql)
	// Output: SELECT name, email FROM users
}

// ExampleSelect_withExpression demonstrates using expressions in field selection
func ExampleSelect_withExpression() {
	sql, _ := surrealql.Select("foo:5").
		Field(`text + "b" AS aa`).
		Build()
	fmt.Println(sql)
	// Output: SELECT text + "b" AS aa FROM foo:5
}

// ExampleSelect_array demonstrates selecting from an array literal
func ExampleSelect_array() {
	arr := []any{1, 2, 3}
	sql, vars := surrealql.Select(surrealql.Expr("?", arr)).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM $from_param_1
	// vars: map[from_param_1:[1 2 3]]
}

// ExampleSelect_object demonstrates selecting from an object literal
func ExampleSelect_object() {
	obj := map[string]any{"a": 1, "b": 2}
	sql, vars := surrealql.Select(surrealql.Expr("?", obj)).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM $from_param_1
	// vars: map[from_param_1:map[a:1 b:2]]
}

// ExampleSelect_arrayOfObjects demonstrates selecting from an array of objects
func ExampleSelect_arrayOfObjects() {
	arr := []any{
		map[string]any{"a": 1},
		map[string]any{"a": 2},
	}
	sql, vars := surrealql.Select(surrealql.Expr("?", arr)).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM $from_param_1
	// vars: map[from_param_1:[map[a:1] map[a:2]]]
}

// ExampleSelect_subquery demonstrates using a subquery as the source
func ExampleSelect_subquery() {
	subquery := surrealql.Select("users").Fields("name").Where("age > ?", 18)
	sql, vars := surrealql.Select(subquery).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM (SELECT name FROM users WHERE age > $from_param_1)
	// Vars:
	//   from_param_1: 18
}

// ExampleSelect_target demonstrates using a target struct
func ExampleSelect_target() {
	target := surrealql.Thing("users", 123)
	sql, vars := surrealql.Select(target).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: users:123
}

// ExampleSelect_recordID demonstrates using a RecordID
func ExampleSelect_recordID() {
	recordID := models.NewRecordID("users", 456)
	sql, vars := surrealql.Select(recordID).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {users 456}
}

// ExampleSelect_complexExpression demonstrates complex field expressions
func ExampleSelect_complexExpression() {
	sql, _ := surrealql.Select("products").
		Field("name").
		Field("price * 1.1 AS price_with_tax").
		Field("count() AS total").
		Where("category = ?", "electronics").
		GroupBy("category").
		Build()
	fmt.Println(sql)
	// Output: SELECT name, price * 1.1 AS price_with_tax, count() AS total FROM products WHERE category = $param_1 GROUP BY category
}

// ExampleSelect_rawExpression demonstrates using a raw expression as the source
func ExampleSelect_rawExpression() {
	// You can pass any valid SurrealQL expression as a string
	sql, _ := surrealql.Select("(SELECT * FROM users WHERE active = true)").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM (SELECT * FROM users WHERE active = true)
}

// ExampleSelect_graphTraversal demonstrates graph traversal as source
func ExampleSelect_graphTraversal() {
	// SurrealDB supports graph traversal in FROM clause
	sql, _ := surrealql.Select("users:john->knows->users").
		FieldName("name").
		Build()
	fmt.Println(sql)
	// Output: SELECT name FROM users:john->knows->users
}

// ExampleSelect_withPlaceholder demonstrates using a placeholder for parameterized FROM
func ExampleSelect_withPlaceholder() {
	// Use ? placeholder at the start - this creates a parameter
	recordID := models.NewRecordID("users", "john")
	sql, vars := surrealql.Select(surrealql.Expr("?->knows->users", recordID)).Build()
	fmt.Println(sql)
	fmt.Printf("vars: from_param_1 type: %T\n", vars["from_param_1"])
	// Output: SELECT * FROM $from_param_1->knows->users
	// vars: from_param_1 type: models.RecordID
}

// ExampleSelect_multiplePlaceholders demonstrates using multiple placeholders
func ExampleSelect_multiplePlaceholders() {
	// Use multiple placeholders in a graph traversal
	// Note: All placeholders become parameters
	fromRecord := models.NewRecordID("users", "john")
	toTable := "users"
	sql, vars := surrealql.Select(
		surrealql.Expr("?->knows->?", fromRecord, toTable),
	).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_param_1->knows->$from_param_2
	// Vars:
	//   from_param_1: {users john}
	//   from_param_2: users
}

// ExampleSelect_placeholderWithTable demonstrates table placeholder
func ExampleSelect_placeholderWithTable() {
	// Dynamically specify table name
	sql, vars := surrealql.Select("?", "products").
		Where("price > ?", 100).
		Build()
	fmt.Println(sql)
	fmt.Printf("vars: from_param_1: %v, param_1: %v\n", vars["from_param_1"], vars["param_1"])
	// Output: SELECT * FROM $from_param_1 WHERE price > $param_1
	// vars: from_param_1: products, param_1: 100
}

package surrealql_test

import (
	"fmt"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectFrom_table demonstrates selecting all from a table
func ExampleSelectFrom_table() {
	sql, _ := surrealql.SelectFrom("users").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM users
}

// ExampleSelectFrom_specificRecord demonstrates selecting from a specific record
func ExampleSelectFrom_specificRecord() {
	sql, _ := surrealql.SelectFrom("users:123").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM users:123
}

// ExampleSelectFrom_withFields demonstrates adding specific fields to SelectFrom
func ExampleSelectFrom_withFields() {
	sql, _ := surrealql.SelectFrom("users").
		FieldName("name").
		FieldName("email").
		Build()
	fmt.Println(sql)
	// Output: SELECT name, email FROM users
}

// ExampleSelectFrom_withExpression demonstrates using expressions in field selection
func ExampleSelectFrom_withExpression() {
	sql, _ := surrealql.SelectFrom("foo:5").
		FieldRaw(`text + "b" AS aa`).
		Build()
	fmt.Println(sql)
	// Output: SELECT text + "b" AS aa FROM foo:5
}

// ExampleSelectFrom_array demonstrates selecting from an array literal
func ExampleSelectFrom_array() {
	arr := []any{1, 2, 3}
	sql, vars := surrealql.SelectFrom("?", arr).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM $from_param_1
	// vars: map[from_param_1:[1 2 3]]
}

// ExampleSelectFrom_object demonstrates selecting from an object literal
func ExampleSelectFrom_object() {
	obj := map[string]any{"a": 1, "b": 2}
	sql, vars := surrealql.SelectFrom("?", obj).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM $from_param_1
	// vars: map[from_param_1:map[a:1 b:2]]
}

// ExampleSelectFrom_arrayOfObjects demonstrates selecting from an array of objects
func ExampleSelectFrom_arrayOfObjects() {
	arr := []any{
		map[string]any{"a": 1},
		map[string]any{"a": 2},
	}
	sql, vars := surrealql.SelectFrom("?", arr).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM $from_param_1
	// vars: map[from_param_1:[map[a:1] map[a:2]]]
}

// ExampleSelectFrom_subquery demonstrates using a subquery as the source
func ExampleSelectFrom_subquery() {
	subquery := surrealql.Select("name").FromTable("users").Where("age > ?", 18)
	sql, vars := surrealql.SelectFrom(subquery).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM (SELECT name FROM users WHERE age > $param_1)
	// vars: map[param_1:18]
}

// ExampleSelectFrom_target demonstrates using a target struct
func ExampleSelectFrom_target() {
	target := surrealql.Thing("users", 123)
	sql, vars := surrealql.SelectFrom(target).Build()
	fmt.Println(sql)
	// Print the RecordID type name instead of the full value for consistent output
	fmt.Printf("vars keys: %v, id_1 type: %T\n", getMapKeys(vars), vars["id_1"])
	// Output:
	// SELECT * FROM $id_1
	// vars keys: [id_1], id_1 type: models.RecordID
}

// ExampleSelectFrom_recordID demonstrates using a RecordID
func ExampleSelectFrom_recordID() {
	recordID := models.NewRecordID("users", 456)
	sql, vars := surrealql.SelectFrom(recordID).Build()
	fmt.Println(sql)
	// Print the RecordID type name instead of the full value for consistent output
	fmt.Printf("vars keys: %v, record_id_1 type: %T\n", getMapKeys(vars), vars["record_id_1"])
	// Output:
	// SELECT * FROM $record_id_1
	// vars keys: [record_id_1], record_id_1 type: models.RecordID
}

// ExampleSelectFrom_complexExpression demonstrates complex field expressions
func ExampleSelectFrom_complexExpression() {
	sql, _ := surrealql.SelectFrom("products").
		FieldRaw("name").
		FieldRaw("price * 1.1 AS price_with_tax").
		FieldRaw("count() AS total").
		Where("category = ?", "electronics").
		GroupBy("category").
		Build()
	fmt.Println(sql)
	// Output: SELECT name, price * 1.1 AS price_with_tax, count() AS total FROM products WHERE category = $param_1 GROUP BY category
}

// ExampleSelectFrom_rawExpression demonstrates using a raw expression as the source
func ExampleSelectFrom_rawExpression() {
	// You can pass any valid SurrealQL expression as a string
	sql, _ := surrealql.SelectFrom("(SELECT * FROM users WHERE active = true)").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM (SELECT * FROM users WHERE active = true)
}

// ExampleSelectFrom_graphTraversal demonstrates graph traversal as source
func ExampleSelectFrom_graphTraversal() {
	// SurrealDB supports graph traversal in FROM clause
	sql, _ := surrealql.SelectFrom("users:john->knows->users").
		FieldName("name").
		Build()
	fmt.Println(sql)
	// Output: SELECT name FROM users:john->knows->users
}

// ExampleSelectFrom_withPlaceholder demonstrates using a placeholder for parameterized FROM
func ExampleSelectFrom_withPlaceholder() {
	// Use ? placeholder at the start - this creates a parameter
	recordID := models.NewRecordID("users", "john")
	sql, vars := surrealql.SelectFrom("?->knows->users", recordID).Build()
	fmt.Println(sql)
	fmt.Printf("vars: from_param_1 type: %T\n", vars["from_param_1"])
	// Output: SELECT * FROM $from_param_1->knows->users
	// vars: from_param_1 type: models.RecordID
}

// ExampleSelectFrom_multiplePlaceholders demonstrates using multiple placeholders
func ExampleSelectFrom_multiplePlaceholders() {
	// Use multiple placeholders in a graph traversal
	// Note: All placeholders become parameters
	fromRecord := models.NewRecordID("users", "john")
	toTable := "users"
	sql, vars := surrealql.SelectFrom("?->knows->?", fromRecord, toTable).
		FieldName("name").
		Build()
	fmt.Println(sql)
	fmt.Printf("vars count: %d, from_param_1 type: %T\n", len(vars), vars["from_param_1"])
	// Output: SELECT name FROM $from_param_1->knows->$from_param_2
	// vars count: 2, from_param_1 type: models.RecordID
}

// ExampleSelectFrom_placeholderWithTable demonstrates table placeholder
func ExampleSelectFrom_placeholderWithTable() {
	// Dynamically specify table name
	sql, vars := surrealql.SelectFrom("?", "products").
		Where("price > ?", 100).
		Build()
	fmt.Println(sql)
	fmt.Printf("vars: from_param_1: %v, param_1: %v\n", vars["from_param_1"], vars["param_1"])
	// Output: SELECT * FROM $from_param_1 WHERE price > $param_1
	// vars: from_param_1: products, param_1: 100
}

// Helper function to get sorted map keys for consistent output
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

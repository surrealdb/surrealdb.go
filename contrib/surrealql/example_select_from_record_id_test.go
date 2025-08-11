package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_modelsRecordID demonstrates using models.RecordID for specific record selection
func ExampleSelect_modelsRecordID() {
	// models.RecordID provides type safety and proper CBOR encoding for record IDs
	recordID := models.NewRecordID("users", "john")
	sql, vars := surrealql.Select(recordID).Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {users john}
}

// ExampleSelect_modelsRecordIDWithFields demonstrates selecting specific fields from a record
func ExampleSelect_modelsRecordIDWithFields() {
	// Select specific fields from a record using models.RecordID
	recordID := models.NewRecordID("products", 12345)
	sql, vars := surrealql.Select(&recordID).
		FieldName("name").
		FieldName("price").
		FieldName("stock").
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT name, price, stock FROM $from_id_1
	// Vars:
	//   from_id_1: products:12345
}

// ExampleSelect_modelsRecordIDWithConditions demonstrates using models.RecordID with WHERE
func ExampleSelect_modelsRecordIDWithConditions() {
	// Even when selecting from a specific record, you can add conditions
	// This is useful for conditional field selection or validation
	recordID := models.NewRecordID("orders", "order_789")
	sql, vars := surrealql.Select(recordID).
		FieldName("items").
		FieldName("total").
		Where("status = ?", "completed").
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT items, total FROM $from_id_1 WHERE status = $param_1
	// Vars:
	//   from_id_1: {orders order_789}
	//   param_1: completed
}

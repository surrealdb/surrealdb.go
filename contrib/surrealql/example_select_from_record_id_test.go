package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectFrom_modelsRecordID demonstrates using models.RecordID for specific record selection
func ExampleSelectFrom_modelsRecordID() {
	// models.RecordID provides type safety and proper CBOR encoding for record IDs
	recordID := models.NewRecordID("users", "john")
	sql, vars := surrealql.SelectFrom(recordID).Build()

	fmt.Println(sql)
	recordIDVar := vars["record_id_1"].(models.RecordID)
	fmt.Printf("vars: record_id_1=%s:%v (type: %T)\n", recordIDVar.Table, recordIDVar.ID, vars["record_id_1"])
	// Output: SELECT * FROM $record_id_1
	// vars: record_id_1=users:john (type: models.RecordID)
}

// ExampleSelectFrom_modelsRecordIDWithFields demonstrates selecting specific fields from a record
func ExampleSelectFrom_modelsRecordIDWithFields() {
	// Select specific fields from a record using models.RecordID
	recordID := models.NewRecordID("products", 12345)
	sql, vars := surrealql.SelectFrom(&recordID).
		FieldName("name").
		FieldName("price").
		FieldName("stock").
		Build()

	fmt.Println(sql)
	recordIDVar := vars["record_id_1"].(models.RecordID)
	fmt.Printf("Record: %s:%v\n", recordIDVar.Table, recordIDVar.ID)
	// Output: SELECT name, price, stock FROM $record_id_1
	// Record: products:12345
}

// ExampleSelectFrom_modelsRecordIDWithConditions demonstrates using models.RecordID with WHERE
func ExampleSelectFrom_modelsRecordIDWithConditions() {
	// Even when selecting from a specific record, you can add conditions
	// This is useful for conditional field selection or validation
	recordID := models.NewRecordID("orders", "order_789")
	sql, vars := surrealql.SelectFrom(recordID).
		FieldName("items").
		FieldName("total").
		Where("status = ?", "completed").
		Build()

	fmt.Println(sql)
	recordIDVar := vars["record_id_1"].(models.RecordID)
	fmt.Printf("Order: %s:%v, Status: %v\n", recordIDVar.Table, recordIDVar.ID, vars["param_1"])
	// Output: SELECT items, total FROM $record_id_1 WHERE status = $param_1
	// Order: orders:order_789, Status: completed
}

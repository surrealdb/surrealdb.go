package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectFrom_tableWithSpecialChars demonstrates using SelectFrom with models.Table for safe table selection
func ExampleSelectFrom_tableWithSpecialChars() {
	// When you have a table name that contains special characters
	// or might be interpreted as an expression, use SelectFrom with models.Table
	sql, vars := surrealql.SelectFrom(models.Table("user-data")).
		FieldName("id").
		FieldName("name").
		Where("active = ?", true).
		Build()

	fmt.Println(sql)
	fmt.Printf("vars: table_1=%v, param_1=%v\n", vars["table_1"], vars["param_1"])
	// Output: SELECT id, name FROM $table_1 WHERE active = $param_1
	// vars: table_1=user-data, param_1=true
}

// ExampleSelectFrom_tableReservedWord demonstrates handling reserved words as table names
func ExampleSelectFrom_tableReservedWord() {
	// Reserved SurrealQL keywords are safely handled using models.Table
	sql, vars := surrealql.SelectFrom(models.Table("select")).
		FieldName("id").
		Build()

	fmt.Println(sql)
	fmt.Printf("vars: table_1=%v\n", vars["table_1"])
	// Output: SELECT id FROM $table_1
	// vars: table_1=select
}

// ExampleSelectFrom_dynamicTableName demonstrates safe handling of dynamic table names
func ExampleSelectFrom_dynamicTableName() {
	// In a multi-tenant system, you might construct table names dynamically
	// SelectFrom with models.Table provides safe parameterization
	tenant := "acme-corp"
	tableName := fmt.Sprintf("events-%s", tenant)

	sql, vars := surrealql.SelectFrom(models.Table(tableName)).
		FieldRaw("count() AS total").
		Build()

	fmt.Println(sql)
	fmt.Printf("vars: table_1=%v\n", vars["table_1"])
	// Output: SELECT count() AS total FROM $table_1
	// vars: table_1=events-acme-corp
}

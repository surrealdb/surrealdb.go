package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_tableWithSpecialChars demonstrates using Select with models.Table for safe table selection
func ExampleSelect_tableWithSpecialChars() {
	// When you have a table name that contains special characters
	// or might be interpreted as an expression, use Select with models.Table
	sql, vars := surrealql.Select(models.Table("user-data")).
		FieldName("id").
		FieldName("name").
		Where("active = ?", true).
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT id, name FROM $from_table_1 WHERE active = $param_1
	// Vars:
	//   from_table_1: user-data
	//   param_1: true
}

// ExampleSelect_tableReservedWord demonstrates handling reserved words as table names
func ExampleSelect_tableReservedWord() {
	// Reserved SurrealQL keywords are safely handled using models.Table
	sql, vars := surrealql.Select(models.Table("select")).
		FieldName("id").
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT id FROM $from_table_1
	// Vars:
	//   from_table_1: select
}

// ExampleSelect_dynamicTableName demonstrates safe handling of dynamic table names
func ExampleSelect_dynamicTableName() {
	// In a multi-tenant system, you might construct table names dynamically
	// Select with models.Table provides safe parameterization
	tenant := "acme-corp"
	tableName := fmt.Sprintf("events-%s", tenant)

	sql, vars := surrealql.Select(models.Table(tableName)).
		Field("count() AS total").
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT count() AS total FROM $from_table_1
	// Vars:
	//   from_table_1: events-acme-corp
}

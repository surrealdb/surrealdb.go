package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectOnly_specificRecord demonstrates selecting from a specific record
func ExampleSelectOnly_specificRecord() {
	sql, _ := surrealql.SelectOnly("users:123").Build()
	fmt.Println(sql)
	// Output: SELECT * FROM ONLY users:123
}

// ExampleSelectOnly_object demonstrates selecting from an object literal
func ExampleSelectOnly_object() {
	obj := map[string]any{"a": 1, "b": 2}
	sql, vars := surrealql.SelectOnly(surrealql.Expr("?", obj)).Build()
	fmt.Println(sql)
	fmt.Printf("vars: %v\n", vars)
	// Output: SELECT * FROM ONLY $from_param_1
	// vars: map[from_param_1:map[a:1 b:2]]
}

// ExampleSelectOnly_thing demonstrates using the Thing helper
func ExampleSelectOnly_target() {
	target := surrealql.Thing("users", 123)
	sql, vars := surrealql.SelectOnly(target).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM ONLY $from_id_1
	// Vars:
	//   from_id_1: users:123
}

// ExampleSelectOnly_recordID demonstrates using a RecordID
func ExampleSelectOnly_recordID() {
	recordID := models.NewRecordID("users", 456)
	sql, vars := surrealql.SelectOnly(recordID).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM ONLY $from_id_1
	// Vars:
	//   from_id_1: {users 456}
}

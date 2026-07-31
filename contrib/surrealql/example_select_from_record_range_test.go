package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_modelsRecordRange shows how to select a range of records.
func ExampleSelect_modelsRecordRange() {
	// Same idea as: SELECT * FROM person:1..=1000
	rr := models.NewRecordRange("person", models.Included(1), models.Included(1000))
	sql, vars := surrealql.Select(rr).Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_range_1
	// Vars:
	//   from_id_range_1: person:1..=1000
}

// ExampleSelect_recordRange_compositeID shows a range over array-style record ids.
// Same idea as: SELECT * FROM temp:['London',NONE]..=['London',..]
func ExampleSelect_recordRange_compositeID() {
	sql, vars := surrealql.Select(surrealql.RecordRange("temp",
		models.IncludedMany[any]("London", models.None),
		models.IncludedMany[any]("London", models.OpenRange()),
	)).Build()

	fmt.Println(sql)
	fmt.Printf("vars count: %d\n", len(vars))
	fmt.Printf("has id_range: %v\n", vars["from_id_range_1"] != nil)
	// Output:
	// SELECT * FROM $from_id_range_1
	// vars count: 1
	// has id_range: true
}

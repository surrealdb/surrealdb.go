package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleInsertRelation() {
	// Insert a relation
	relationData := surrealql.NewRelationData().
		SetIn("person:1").
		SetID("follows").
		SetOut("person:2").
		Set("since", "2023-01-01").
		Build()

	q := surrealql.Insert("likes").Relation().Value(relationData)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// INSERT RELATION INTO likes $insert_data_1
}

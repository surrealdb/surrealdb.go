package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleInsertRelation() {
	// Insert a relation
	relationData := surrealdb.Relationship{
		In:  models.NewRecordID("person", 1),
		Out: models.NewRecordID("person", 2),
		Data: map[string]any{
			"since": "2023-01-01",
		},
	}

	q := surrealql.Insert("likes").Relation().Value(relationData)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// INSERT RELATION INTO likes $insert_data_1
}

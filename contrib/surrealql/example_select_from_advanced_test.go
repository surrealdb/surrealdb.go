package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectFrom_edgecase_invalidAdvancedGraphTraversal demonstrates a complex and invalid graph traversal with placeholders
// As surrealql does not parse `->` expressions on its own, it produces an invalid query.
// In SurrealQL, you can write `FROM $from_param_1->follows->users->created->posts` which is valid.
// However, `FROM $from_param_1->$from_param_2` is invalid- you cannot place random variables in the middle of a path.
func ExampleSelectFrom_edgecase_invalidAdvancedGraphTraversal() {
	// Imagine building a dynamic social network query where the starting user
	// and relationship type are parameters
	startUser := models.NewRecordID("users", "alice")
	relationshipType := "follows"
	targetTable := "posts"

	// Build a query to find all posts from users that Alice follows
	sql, vars := surrealql.SelectFrom("?->?->users->created->?",
		startUser,
		relationshipType,
		targetTable).
		FieldName("title").
		FieldName("content").
		FieldName("created_at").
		Where("published = ?", true).
		OrderByDesc("created_at").
		Limit(10).
		Build()

	fmt.Println(sql)
	fmt.Printf("vars count: %d\n", len(vars))
	// Output: SELECT title, content, created_at FROM $from_param_1->$from_param_2->users->created->$from_param_3 WHERE published = $param_1 ORDER BY created_at DESC LIMIT 10
	// vars count: 4
}

// ExampleSelectFrom_dynamicTableSelection demonstrates dynamic table selection
func ExampleSelectFrom_dynamicTableSelection() {
	// In a multi-tenant application, you might need to dynamically select
	// from different tables based on the tenant
	tenant := "acme_corp"
	tableName := fmt.Sprintf("events_%s", tenant)

	sql, vars := surrealql.SelectFrom("?", tableName).
		FieldRaw("count() AS total_events").
		FieldRaw("math::mean(duration) AS avg_duration").
		Where("timestamp > ? AND timestamp < ?", "2024-01-01", "2024-12-31").
		GroupBy("event_type").
		Build()

	fmt.Println(sql)
	// Table name is now a parameter
	fmt.Printf("table param: %v\n", vars["from_param_1"])
	fmt.Printf("timestamp params: %v, %v\n", vars["param_1"], vars["param_2"])
	// Output: SELECT count() AS total_events, math::mean(duration) AS avg_duration FROM $from_param_1 WHERE timestamp > $param_1 AND timestamp < $param_2 GROUP BY event_type
	// table param: events_acme_corp
	// timestamp params: 2024-01-01, 2024-12-31
}

// ExampleSelectFrom_complexPlaceholderCombination demonstrates combining different types
func ExampleSelectFrom_complexPlaceholderCombination() {
	// Complex scenario: Start from a record, traverse through a dynamic relationship,
	// and end at a dynamic target
	user := models.NewRecordID("users", 123)
	edge := "purchased"

	// Find all products purchased by user 123
	sql, vars := surrealql.SelectFrom("?->?->products", user, edge).
		FieldName("name").
		FieldName("price").
		FieldRaw("price * 0.1 AS tax").
		Build()

	fmt.Println(sql)
	// All placeholders become parameters
	fmt.Printf("vars: from_param_1 type: %T, from_param_2: %v\n", vars["from_param_1"], vars["from_param_2"])
	// Output: SELECT name, price, price * 0.1 AS tax FROM $from_param_1->$from_param_2->products
	// vars: from_param_1 type: models.RecordID, from_param_2: purchased
}

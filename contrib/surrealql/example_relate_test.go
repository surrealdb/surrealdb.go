package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleRelate() {
	// Create a "likes" relation between user and post
	query := surrealql.Relate("users:123", "likes", "posts:456").
		Set("liked_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Set("reaction", "heart")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: RELATE users:123->likes->posts:456 SET liked_at = $liked_at_1, reaction = $reaction_1
	// Vars: map[liked_at_1:2023-10-01 12:00:00 +0000 UTC reaction_1:heart]
}

func ExampleRelate_compoundOperations() {
	// RELATE with compound operations using the Set function
	sql, vars := surrealql.Relate("users:123", "views", "posts:456").
		Set("count += ?", 1).             // Increment view count
		Set("last_viewed", "2024-01-01"). // Simple assignment
		Set("duration_seconds += ?", 30). // Add to duration
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// RELATE users:123->views->posts:456 SET last_viewed = $last_viewed_1, count += $param_1, duration_seconds += $param_2
	// Variables: map[last_viewed_1:2024-01-01 param_1:1 param_2:30]
}

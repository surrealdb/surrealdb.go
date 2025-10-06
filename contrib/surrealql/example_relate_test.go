package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleRelate() {
	// Create a "likes" relation between user and post
	query := surrealql.Relate("users:123", "likes", "posts:456").
		Set("liked_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Set("reaction", "heart")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: RELATE users:123->likes->posts:456 SET liked_at = $param_1, reaction = $param_2
	// Vars:
	//   param_1: 2023-10-01 12:00:00 +0000 UTC
	//   param_2: heart
}

func ExampleRelateOnly() {
	// Create a "follows" relation between user and another user, ensuring the single relation itself,
	// rather than an array containing the only relation, is returned.
	query := surrealql.RelateOnly("users:123", "likes", "posts:456").
		Set("liked_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Set("reaction", "heart")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: RELATE ONLY users:123->likes->posts:456 SET liked_at = $param_1, reaction = $param_2
	// Vars:
	//   param_1: 2023-10-01 12:00:00 +0000 UTC
	//   param_2: heart
}

func ExampleRelate_compoundOperations() {
	// RELATE with compound operations using the Set function
	sql, vars := surrealql.Relate("users:123", "views", "posts:456").
		Set("count += ?", 1).             // Increment view count
		Set("last_viewed", "2024-01-01"). // Simple assignment
		Set("duration_seconds += ?", 30). // Add to duration
		Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// RELATE users:123->views->posts:456 SET count += $param_1, last_viewed = $param_2, duration_seconds += $param_3
	// Vars:
	//   param_1: 1
	//   param_2: 2024-01-01
	//   param_3: 30
}

func ExampleRelate_recordID() {
	from := models.NewRecordID("users", 123)
	to := models.NewRecordID("posts", 456)
	query := surrealql.Relate(from, "likes", to).
		Set("liked_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		Set("reaction", "heart")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: RELATE $id_1->likes->$id_2 SET liked_at = $param_1, reaction = $param_2
	// Vars:
	//   id_1: {users 123}
	//   id_2: {posts 456}
	//   param_1: 2023-10-01 12:00:00 +0000 UTC
	//   param_2: heart
}

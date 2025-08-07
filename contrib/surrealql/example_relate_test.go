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
	// SurrealQL: RELATE users:123->likes->posts:456 CONTENT $content_1
	// Vars: map[content_1:map[liked_at:2023-10-01 12:00:00 +0000 UTC reaction:heart]]
}

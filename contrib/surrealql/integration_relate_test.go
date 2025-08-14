package surrealql_test

import (
	"context"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegrationRelate(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "users", "posts", "likes")

	ctx := context.Background()

	// Create a user and a post
	type User struct {
		ID   *models.RecordID `json:"id,omitempty"`
		Name string           `json:"name"`
	}

	type Post struct {
		ID    *models.RecordID `json:"id,omitempty"`
		Title string           `json:"title"`
	}

	user, err := surrealdb.Create[User](ctx, db, "users", User{Name: "John"})
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	post, err := surrealdb.Create[Post](ctx, db, "posts", Post{Title: "Hello World"})
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	// Create a relation
	t.Run("CreateRelation", func(t *testing.T) {
		query := surrealql.Relate(user.ID.String(), "likes", post.ID.String()).
			Set("rating", 5).
			Set("created_at", models.CustomDateTime{Time: time.Now()})

		sql, vars := query.Build()

		type Like struct {
			ID        models.RecordID       `json:"id,omitempty"`
			In        models.RecordID       `json:"in"`
			Out       models.RecordID       `json:"out"`
			Rating    int                   `json:"rating"`
			CreatedAt models.CustomDateTime `json:"created_at"`
		}

		results, err := surrealdb.Query[[]Like](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Relate failed: %v", err)
		}

		likes := (*results)[0].Result
		if len(likes) != 1 {
			t.Fatalf("Expected 1 relation created, got %d", len(likes))
		}

		if likes[0].Rating != 5 {
			t.Errorf("Expected rating 5, got %d", likes[0].Rating)
		}

		// Verify the relation exists
		selectQuery := surrealql.Select("likes")
		sql, vars = selectQuery.Build()

		results, err = surrealdb.Query[[]Like](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		likes = (*results)[0].Result
		if len(likes) != 1 {
			t.Errorf("Expected 1 like relation, got %d", len(likes))
		}
	})
}

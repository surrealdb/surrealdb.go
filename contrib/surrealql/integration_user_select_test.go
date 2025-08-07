package surrealql_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// User struct for SELECT tests
type User struct {
	ID     *models.RecordID `json:"id,omitempty"`
	Name   string           `json:"name"`
	Email  string           `json:"email"`
	Active bool             `json:"active"`
	Age    int              `json:"age"`
}

// setupUserData creates test user data for SELECT tests
func setupUserData(t *testing.T, ctx context.Context, db *surrealdb.DB, table string) {
	testUsers := []User{
		{Name: "Alice", Email: "alice@example.com", Active: true, Age: 25},
		{Name: "Bob", Email: "bob@example.com", Active: true, Age: 30},
		{Name: "Charlie", Email: "charlie@example.com", Active: false, Age: 35},
		{Name: "Diana", Email: "diana@example.com", Active: true, Age: 28},
	}

	for _, user := range testUsers {
		_, err := surrealdb.Create[User](ctx, db, table, user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}
}

func TestIntegrationSelect_All(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "users_all")
	ctx := context.Background()

	// Setup test data
	setupUserData(t, ctx, db, "users_all")

	query := surrealql.Select("*").FromTable("users_all")
	sql, vars := query.Build()

	results, err := surrealdb.Query[[]User](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(*results) != 1 {
		t.Fatalf("Expected 1 result set, got %d", len(*results))
	}

	if (*results)[0].Status != surrealql.StatusOK {
		t.Fatalf("Query status not OK: %s", (*results)[0].Status)
	}

	users := (*results)[0].Result
	if len(users) != 4 {
		t.Errorf("Expected 4 users, got %d", len(users))
	}
}

func TestIntegrationSelect_WhereEq(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "users_whereeq")
	ctx := context.Background()

	// Setup test data
	setupUserData(t, ctx, db, "users_whereeq")

	query := surrealql.Select("id", "name", "email").
		FromTable("users_whereeq").
		WhereEq("active", true).
		OrderBy("name")

	sql, vars := query.Build()

	results, err := surrealdb.Query[[]User](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	users := (*results)[0].Result
	if len(users) != 3 {
		t.Errorf("Expected 3 active users, got %d", len(users))
	}

	// Check order
	if len(users) > 0 && users[0].Name != "Alice" {
		t.Errorf("Expected first user to be Alice, got %s", users[0].Name)
	}
}

func TestIntegrationSelect_WhereWithParams(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "users_params")
	ctx := context.Background()

	// Setup test data
	setupUserData(t, ctx, db, "users_params")

	query := surrealql.Select("*").
		FromTable("users_params").
		Where("age > ? AND active = ?", 26, true).
		OrderByDesc("age")

	sql, vars := query.Build()

	results, err := surrealdb.Query[[]User](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	users := (*results)[0].Result
	if len(users) != 2 { // Bob (30) and Diana (28)
		t.Errorf("Expected 2 users, got %d", len(users))
	}

	if len(users) > 0 && users[0].Name != "Bob" {
		t.Errorf("Expected first user to be Bob, got %s", users[0].Name)
	}
}

func TestIntegrationSelect_WithPagination(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "users_page")
	ctx := context.Background()

	// Setup test data
	setupUserData(t, ctx, db, "users_page")

	query := surrealql.Select("*").
		FromTable("users_page").
		OrderBy("name").
		Limit(2).
		Start(1)

	sql, vars := query.Build()

	results, err := surrealdb.Query[[]User](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	users := (*results)[0].Result
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}

	if len(users) > 0 && users[0].Name != "Bob" {
		t.Errorf("Expected first user to be Bob, got %s", users[0].Name)
	}
}

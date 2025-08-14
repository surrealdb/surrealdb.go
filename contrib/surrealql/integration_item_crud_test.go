package surrealql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Item struct for CRUD tests
type Item struct {
	ID          *models.RecordID      `json:"id,omitempty"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Price       float64               `json:"price"`
	Active      bool                  `json:"active"`
	UpdatedAt   models.CustomDateTime `json:"updated_at"`
}

func TestIntegrationCreateThenUpdate(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "items")
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		query := surrealql.Create("items").
			Set("name", "Test Item").
			Set("description", "A test item").
			Set("price", 99.99).
			Set("active", true).
			Set("updated_at", models.CustomDateTime{Time: time.Now()})

		sql, vars := query.Build()

		results, err := surrealdb.Query[[]Item](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		items := (*results)[0].Result
		if len(items) != 1 {
			t.Fatalf("Expected 1 item created, got %d", len(items))
		}

		if items[0].Name != "Test Item" {
			t.Errorf("Expected name 'Test Item', got %s", items[0].Name)
		}
	})

	t.Run("Update", func(t *testing.T) {
		// First create some items
		for i := 1; i <= 3; i++ {
			_, err := surrealdb.Create[Item](ctx, db, "items", Item{
				Name:      fmt.Sprintf("Item %c", 'A'+i-1),
				Price:     float64(i * 10),
				Active:    true,
				UpdatedAt: models.CustomDateTime{Time: time.Now()},
			})
			if err != nil {
				t.Fatalf("Failed to create item: %v", err)
			}
		}

		// Update items with price > 15
		query := surrealql.Update("items").
			Set("active", false).
			Set("updated_at", models.CustomDateTime{Time: time.Now()}).
			Where("price > ?", 15.0)

		sql, vars := query.Build()

		_, err := surrealdb.Query[[]Item](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// First check all items
		allQuery := surrealql.Select("items")
		allQL, allParams := allQuery.Build()
		allResults, _ := surrealdb.Query[[]Item](ctx, db, allQL, allParams)
		if len(*allResults) > 0 {
			t.Logf("All items after update: %+v", (*allResults)[0].Result)
		}

		// Verify update
		selectQuery := surrealql.Select("items").WhereEq("active", false)
		sql, vars = selectQuery.Build()

		results, err := surrealdb.Query[[]Item](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		items := (*results)[0].Result
		// Test Item (99.99), Item B (20), and Item C (30) all have price > 15
		if len(items) != 3 {
			t.Errorf("Expected 3 inactive items, got %d", len(items))
		}
	})
}

func TestIntegrationCreateThenDelete(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "items_delete")
	ctx := context.Background()

	// Setup: Create items with different active states
	t.Run("Setup", func(t *testing.T) {
		// Create active items
		for i := 1; i <= 2; i++ {
			_, err := surrealdb.Create[Item](ctx, db, "items_delete", Item{
				Name:      fmt.Sprintf("Active Item %d", i),
				Price:     float64(i * 10),
				Active:    true,
				UpdatedAt: models.CustomDateTime{Time: time.Now()},
			})
			if err != nil {
				t.Fatalf("Failed to create active item: %v", err)
			}
		}

		// Create inactive items
		for i := 1; i <= 3; i++ {
			_, err := surrealdb.Create[Item](ctx, db, "items_delete", Item{
				Name:      fmt.Sprintf("Inactive Item %d", i),
				Price:     float64(i * 20),
				Active:    false,
				UpdatedAt: models.CustomDateTime{Time: time.Now()},
			})
			if err != nil {
				t.Fatalf("Failed to create inactive item: %v", err)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Delete inactive items
		query := surrealql.Delete("items_delete").
			Where("active = ?", false)

		sql, vars := query.Build()

		_, err := surrealdb.Query[[]Item](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify deletion
		selectQuery := surrealql.Select("items_delete")
		sql, vars = selectQuery.Build()

		results, err := surrealdb.Query[[]Item](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		items := (*results)[0].Result
		// After DELETE of inactive items:
		// Only 2 active items should remain
		if len(items) != 2 {
			t.Errorf("Expected 2 items remaining, got %d", len(items))
			for i, item := range items {
				t.Logf("Item %d: %+v", i, item)
			}
		}

		for _, item := range items {
			if !item.Active {
				t.Errorf("Expected all remaining items to be active")
			}
		}
	})
}

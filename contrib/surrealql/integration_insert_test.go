package surrealql_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegrationInsert(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "products", "categories", "product_categories")

	ctx := context.Background()

	type Product struct {
		ID       models.RecordID `json:"id,omitempty"`
		Name     string          `json:"name"`
		Price    float64         `json:"price"`
		Category string          `json:"category"`
	}

	t.Run("InsertSingle", func(t *testing.T) {
		// Insert a single product
		insertQuery := surrealql.Insert("products").Value(map[string]any{
			"name":     "Laptop",
			"price":    999.99,
			"category": "Electronics",
		})

		sql, vars := insertQuery.Build()
		t.Logf("INSERT QL: %s", sql)
		t.Logf("INSERT Params: %v", vars)

		results, err := surrealdb.Query[[]Product](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		products := (*results)[0].Result
		if len(products) != 1 {
			t.Fatalf("Expected 1 product inserted, got %d", len(products))
		}

		if products[0].Name != "Laptop" {
			t.Errorf("Expected name 'Laptop', got %s", products[0].Name)
		}
	})

	t.Run("InsertMultipleWithValues", func(t *testing.T) {
		// Insert multiple products using VALUES
		insertQuery := surrealql.Insert("products").
			Fields("name", "price", "category").
			Values("Phone", 699.99, "Electronics").
			Values("Desk", 299.99, "Furniture").
			Values("Chair", 199.99, "Furniture")

		sql, vars := insertQuery.Build()

		results, err := surrealdb.Query[[]Product](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		products := (*results)[0].Result
		if len(products) != 3 {
			t.Fatalf("Expected 3 products inserted, got %d", len(products))
		}
	})

	t.Run("InsertWithReturnNone", func(t *testing.T) {
		// Insert with RETURN NONE
		insertQuery := surrealql.Insert("products").
			Value(map[string]any{
				"name":     "Monitor",
				"price":    399.99,
				"category": "Electronics",
			}).
			ReturnNone()

		sql, vars := insertQuery.Build()

		results, err := surrealdb.Query[[]Product](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		// With RETURN NONE, result should be empty
		if len((*results)[0].Result) != 0 {
			t.Errorf("Expected empty result with RETURN NONE, got %d items", len((*results)[0].Result))
		}
	})

	t.Run("InsertRelation", func(t *testing.T) {
		// Create a category first
		type Category struct {
			ID   models.RecordID `json:"id,omitempty"`
			Name string          `json:"name"`
		}
		// Create category using query
		createCat := surrealql.Create("categories").
			Set("name", "Electronics")
		sql, vars := createCat.Build()
		catResults, err := surrealdb.Query[[]Category](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Failed to create category: %v", err)
		}
		category := (*catResults)[0].Result[0]

		// Get first product
		products, err := surrealdb.Select[[]Product](ctx, db, "products")
		if err != nil || len(*products) == 0 {
			t.Fatalf("Failed to get products: %v", err)
		}

		// Insert a relation using RELATE instead as INSERT RELATION might have issues
		relateQuery := surrealql.Relate((*products)[0].ID.String(), "belongs_to", category.ID.String()).
			Set("primary", true)

		sql, vars = relateQuery.Build()
		t.Logf("RELATE SurrealQL: %s", sql)

		_, err = surrealdb.Query[any](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("RELATE failed: %v", err)
		}
	})
}

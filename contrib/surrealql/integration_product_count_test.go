package surrealql_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Product struct for COUNT tests
type Product struct {
	ID       *models.RecordID `json:"id,omitempty"`
	Name     string           `json:"name"`
	Category string           `json:"category"`
	Price    float64          `json:"price"`
	InStock  bool             `json:"in_stock"`
}

// setupProductData creates test product data for COUNT tests
func setupProductData(t *testing.T, ctx context.Context, db *surrealdb.DB, table string) {
	testProducts := []Product{
		{Name: "Laptop", Category: "Electronics", Price: 999.99, InStock: true},
		{Name: "Mouse", Category: "Electronics", Price: 29.99, InStock: true},
		{Name: "Keyboard", Category: "Electronics", Price: 79.99, InStock: false},
		{Name: "Desk", Category: "Furniture", Price: 299.99, InStock: true},
		{Name: "Chair", Category: "Furniture", Price: 199.99, InStock: true},
		{Name: "Lamp", Category: "Furniture", Price: 49.99, InStock: false},
	}

	for _, product := range testProducts {
		_, err := surrealdb.Create[Product](ctx, db, table, product)
		if err != nil {
			t.Fatalf("Failed to create product: %v", err)
		}
	}
}

func TestIntegrationCount_All(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "products_all")
	ctx := context.Background()

	// Setup test data
	setupProductData(t, ctx, db, "products_all")

	// First check if products exist
	checkQuery := surrealql.Select("products_all")
	checkQL, checkParams := checkQuery.Build()
	checkResults, err := surrealdb.Query[[]Product](ctx, db, checkQL, checkParams)
	if err != nil {
		t.Fatalf("Failed to check products: %v", err)
	}
	if len(*checkResults) > 0 {
		t.Logf("Found %d products in database", len((*checkResults)[0].Result))
	}

	// Try raw query first
	rawResults, err := surrealdb.Query[[]map[string]any](ctx, db, "SELECT count() FROM products_all GROUP ALL", nil)
	if err != nil {
		t.Fatalf("Raw query failed: %v", err)
	}
	t.Logf("Raw query with GROUP ALL results: %+v", rawResults)

	query := surrealql.Select("products_all").Fields("count()").GroupAll()
	sql, vars := query.Build()
	t.Logf("COUNT SurrealQL: %s", sql)
	t.Logf("COUNT Params: %v", vars)

	type CountResult struct {
		Count int `json:"count"`
	}

	results, err := surrealdb.Query[[]CountResult](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	t.Logf("Full results: %+v", results)
	if len(*results) > 0 {
		t.Logf("First result: %+v", (*results)[0])
		t.Logf("Result data: %+v", (*results)[0].Result)
	}

	countResults := (*results)[0].Result
	if len(countResults) == 0 {
		t.Errorf("No count results returned")
	} else if countResults[0].Count != 6 {
		t.Errorf("Expected count 6, got %d", countResults[0].Count)
	}
}

func TestIntegrationCount_WithWhere(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "products_where")
	ctx := context.Background()

	// Setup test data
	setupProductData(t, ctx, db, "products_where")

	query := surrealql.Select("products_where").
		Fields("count()").
		WhereEq("in_stock", true).
		GroupAll()

	sql, vars := query.Build()

	type CountResult struct {
		Count int `json:"count"`
	}

	results, err := surrealdb.Query[[]CountResult](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	countResults := (*results)[0].Result
	if len(countResults) > 0 && countResults[0].Count != 4 {
		t.Errorf("Expected count 4, got %d", countResults[0].Count)
	}
}

func TestIntegrationCount_GroupBy(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "products_group")
	ctx := context.Background()

	// Setup test data
	setupProductData(t, ctx, db, "products_group")

	query := surrealql.Select("products_group").
		Fields("category, count()").
		GroupBy("category").
		OrderByDesc("count")

	sql, vars := query.Build()

	type CategoryCount struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	}

	results, err := surrealdb.Query[[]CategoryCount](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	categories := (*results)[0].Result
	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}

	// Both categories should have 3 products each
	for _, cat := range categories {
		if cat.Count != 3 {
			t.Errorf("Expected count 3 for %s, got %d", cat.Category, cat.Count)
		}
	}
}

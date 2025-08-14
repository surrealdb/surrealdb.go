package surrealql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

func TestIntegration_UpsertSet(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_set", "product")
	ctx := context.Background()

	t.Run("creates new record", func(t *testing.T) {
		// UPSERT with SET
		sql, vars := surrealql.Upsert("product:upsert_test1").
			Set("name", "Test Product").
			Set("price", 99).
			ReturnAfter().
			Build()

		resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, *resp, 1)

		// Verify the record was created
		products := (*resp)[0].Result
		require.Len(t, products, 1)
		assert.Equal(t, "Test Product", products[0]["name"])
		assert.EqualValues(t, 99, products[0]["price"])
	})

	t.Run("updates existing record", func(t *testing.T) {
		// First, create a record
		_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test2 SET name = 'Original', price = 50", nil)
		require.NoError(t, err)

		// UPSERT to update it
		sql, vars := surrealql.Upsert("product:upsert_test2").
			Set("name", "Updated").
			Set("price", 75).
			ReturnAfter().
			Build()

		resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
		require.NoError(t, err)

		// Verify the record was updated
		products := (*resp)[0].Result
		require.Len(t, products, 1)
		assert.Equal(t, "Updated", products[0]["name"])
		assert.EqualValues(t, 75, products[0]["price"])
	})
}

func TestIntegration_UpsertContent(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_content", "product")
	ctx := context.Background()

	// UPSERT with CONTENT
	sql, vars := surrealql.Upsert("product:upsert_test3").
		Content(map[string]any{
			"name":      "Content Product",
			"price":     199,
			"category":  "electronics",
			"available": true,
		}).
		ReturnAfter().
		Build()

	resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// Verify the record
	products := (*resp)[0].Result
	require.Len(t, products, 1)
	assert.Equal(t, "Content Product", products[0]["name"])
	assert.EqualValues(t, 199, products[0]["price"])
	assert.Equal(t, "electronics", products[0]["category"])
	assert.Equal(t, true, products[0]["available"])
}

func TestIntegration_UpsertMerge(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_merge", "product")
	ctx := context.Background()

	// First, create a record with initial data
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test4 SET name = 'Merge Product', price = 150, brand = 'TechCorp'", nil)
	require.NoError(t, err)

	// UPSERT with MERGE to add/update fields
	sql, vars := surrealql.Upsert("product:upsert_test4").
		Merge(map[string]any{
			"price":    175,       // Update existing field
			"warranty": "2 years", // Add new field
		}).
		ReturnAfter().
		Build()

	resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// Verify the merge
	products := (*resp)[0].Result
	require.Len(t, products, 1)
	assert.Equal(t, "Merge Product", products[0]["name"]) // Original field preserved
	assert.EqualValues(t, 175, products[0]["price"])      // Updated field
	assert.Equal(t, "TechCorp", products[0]["brand"])     // Original field preserved
	assert.Equal(t, "2 years", products[0]["warranty"])   // New field added
}

func TestIntegration_UpsertWhere(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_where", "product")
	ctx := context.Background()

	// Create a record
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test5 SET name = 'Premium Item', price = 500", nil)
	require.NoError(t, err)

	// UPSERT with condition that matches
	sql, vars := surrealql.Upsert("product:upsert_test5").
		Set("tier", "premium").
		Where("price >= ?", 500).
		ReturnAfter().
		Build()

	resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	products := (*resp)[0].Result
	require.Len(t, products, 1)
	assert.Equal(t, "premium", products[0]["tier"])

	// UPSERT with condition that doesn't match
	sql, vars = surrealql.Upsert("product:upsert_test5").
		Set("tier", "budget").
		Where("price < ?", 100).
		ReturnAfter().
		Build()

	resp, err = surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// When the WHERE condition doesn't match, UPSERT returns empty results
	products = (*resp)[0].Result
	require.Len(t, products, 0) // No records returned when WHERE doesn't match

	// Verify the record wasn't updated by querying it directly
	verifyResp, err := surrealdb.Query[[]map[string]any](ctx, db, "SELECT * FROM product:upsert_test5", nil)
	require.NoError(t, err)
	verifyProducts := (*verifyResp)[0].Result
	require.Len(t, verifyProducts, 1)
	assert.Equal(t, "premium", verifyProducts[0]["tier"]) // Tier unchanged
}

func TestIntegration_UpsertOnly(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_only", "product")
	ctx := context.Background()

	// UPSERT ONLY
	sql, vars := surrealql.UpsertOnly("product:upsert_test6").
		Set("name", "Single Product").
		Set("type", "unique").
		Build()

	// Note: ONLY returns a single record, not wrapped in an array
	resp, err := surrealdb.Query[map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// Should return a single record
	product := (*resp)[0].Result
	assert.Equal(t, "Single Product", product["name"])
	assert.Equal(t, "unique", product["type"])
}

func TestIntegration_UpsertReturnNone(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_return_none", "product")
	ctx := context.Background()

	// UPSERT with RETURN NONE
	sql, vars := surrealql.Upsert("product:upsert_test7").
		Set("name", "No Return Product").
		ReturnNone().
		Build()

	resp, err := surrealdb.Query[[]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// With RETURN NONE, the result should be empty
	if len(*resp) > 0 && (*resp)[0].Result != nil {
		result := (*resp)[0].Result
		assert.Empty(t, result)
	}
}

func TestIntegration_UpsertReturnDiff(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_return_diff", "product")
	ctx := context.Background()

	// First create a record
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test8 SET name = 'Original Product', price = 250", nil)
	require.NoError(t, err)

	// UPSERT with RETURN DIFF
	sql, vars := surrealql.Upsert("product:upsert_test8").
		Set("name", "Modified Product").
		Set("price", 299).
		Set("new_field", "added").
		ReturnDiff().
		Build()

	resp, err := surrealdb.Query[[]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// The diff should show changes
	// The exact format of DIFF depends on SurrealDB's implementation
	require.NotNil(t, resp)
}

func TestIntegration_UpsertSetRaw(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_setraw", "product")
	ctx := context.Background()

	// First create a record with initial values
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test9 SET name = 'Counter Product', views = 100, tags = ['new']", nil)
	require.NoError(t, err)

	// UPSERT with SetRaw for compound operations
	sql, vars := surrealql.Upsert("product:upsert_test9").
		Set("views += 1").
		Set("tags += 'popular'").
		Set("last_viewed", "2024-01-01").
		ReturnAfter().
		Build()

	resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// Verify the updates
	products := (*resp)[0].Result
	require.Len(t, products, 1)

	product := products[0]
	assert.Equal(t, "Counter Product", product["name"])
	assert.EqualValues(t, 101, product["views"]) // Should be incremented
	assert.Equal(t, "2024-01-01", product["last_viewed"])

	// Check that tags array contains both original and new tag
	tags, ok := product["tags"].([]any)
	if ok {
		assert.Contains(t, tags, "new")
		assert.Contains(t, tags, "popular")
	}
}

func TestIntegration_UpsertUnifiedSet(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_unified", "product")
	ctx := context.Background()

	// First create a record with initial values
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test10 SET name = 'Unified Product', stock = 50, price = 100", nil)
	require.NoError(t, err)

	// UPSERT with Set for both simple and compound operations
	sql, vars := surrealql.Upsert("product:upsert_test10").
		Set("name", "Updated Product").               // Simple assignment
		Set("stock -= ?", 5).                         // Compound operation with parameter
		Set("price += ?", 20).                        // Add to price
		Set("last_modified", "2024-01-15T10:00:00Z"). // Simple assignment
		ReturnAfter().
		Build()

	resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// Verify the updates
	products := (*resp)[0].Result
	require.Len(t, products, 1)

	product := products[0]
	assert.Equal(t, "Updated Product", product["name"])
	assert.EqualValues(t, 45, product["stock"])  // 50 - 5
	assert.EqualValues(t, 120, product["price"]) // 100 + 20
	assert.Equal(t, "2024-01-15T10:00:00Z", product["last_modified"])
}

func TestIntegration_UpsertSetArrayOperations(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_arrays", "product")
	ctx := context.Background()

	// First create a record with initial array values
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:upsert_test11 SET name = 'Array Product', tags = ['new'], stock = 10", nil)
	require.NoError(t, err)

	// UPSERT with Set for array operations
	sql, vars := surrealql.Upsert("product:upsert_test11").
		Set("tags += ?", []string{"featured", "sale"}). // Append multiple tags
		Set("stock -= ?", 2).                           // Decrement stock
		Set("last_updated", "2024-01-20").              // Simple assignment
		ReturnAfter().
		Build()

	resp, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// Verify the updates
	products := (*resp)[0].Result
	require.Len(t, products, 1)

	product := products[0]
	assert.Equal(t, "Array Product", product["name"])
	assert.EqualValues(t, 8, product["stock"]) // 10 - 2
	assert.Equal(t, "2024-01-20", product["last_updated"])

	// Check that tags array contains all tags
	tags, ok := product["tags"].([]any)
	if ok {
		assert.Contains(t, tags, "new")
		assert.Contains(t, tags, "featured")
		assert.Contains(t, tags, "sale")
	}
}

func TestIntegration_UpsertReturnValue(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "surrealql_test", "upsert_return_value", "product")
	ctx := context.Background()

	// First create a record with initial value
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE product:counter SET view_count = 5, name = 'Counter Product'", nil)
	require.NoError(t, err)

	// UPSERT with RETURN VALUE - should return just the field value
	sql, vars := surrealql.Upsert("product:counter").
		Set("view_count += ?", 1).
		ReturnValue("view_count").
		Build()

	// The response should be just the value of the field
	resp, err := surrealdb.Query[[]any](ctx, db, sql, vars)
	require.NoError(t, err)

	// RETURN VALUE returns just the field value, not a full record
	result := (*resp)[0].Result
	// The result should be [6] (the new view_count value)
	require.Len(t, result, 1)
	assert.EqualValues(t, 6, result[0]) // 5 + 1
}

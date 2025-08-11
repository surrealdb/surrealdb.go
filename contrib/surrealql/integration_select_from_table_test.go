package surrealql_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegrationSelect_fromTable(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "select_from_table_test")
	ctx := context.Background()

	// Create test data in a table with special characters
	tableName := "user_data_test" // Using a unique table name

	// Clean up any existing data first
	cleanupQuery := fmt.Sprintf("DELETE %s", tableName)
	_, _ = surrealdb.Query[any](ctx, db, cleanupQuery, nil)

	// Insert test data using raw query
	insertQuery := fmt.Sprintf("INSERT INTO %s [{ name: 'Alice', active: true }, { name: 'Bob', active: false }, { name: 'Charlie', active: true }]", tableName)
	_, err := surrealdb.Query[any](ctx, db, insertQuery, nil)
	assert.NoError(t, err)

	t.Run("Select with models.Table and special characters", func(t *testing.T) {
		// Build query using Select with models.Table
		sql, vars := surrealql.Select(models.Table(tableName)).
			Where("active = ?", true).
			OrderBy("name").
			Build()

		// Execute the query
		type UserData struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}

		results, err := surrealdb.Query[[]UserData](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify results
		if len(*results) > 0 && (*results)[0].Result != nil {
			users := (*results)[0].Result
			assert.Len(t, users, 2, "Should find 2 active users")
			if len(users) == 2 {
				assert.Equal(t, "Alice", users[0].Name)
				assert.Equal(t, "Charlie", users[1].Name)
			}
		}
	})

	t.Run("Select with models.Table all records", func(t *testing.T) {
		// Build query to select all records
		sql, vars := surrealql.Select(models.Table(tableName)).Build()

		t.Logf("Select all query SQL: %s", sql)
		t.Logf("Select all query vars: %+v", vars)

		// Execute the query
		type UserData struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}

		results, err := surrealdb.Query[[]UserData](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify we get all 3 records
		if len(*results) > 0 && (*results)[0].Result != nil {
			assert.Len(t, (*results)[0].Result, 3, "Should find all 3 users")
		}
	})

	// Test with dynamic table name
	t.Run("Select with models.Table dynamic name", func(t *testing.T) {
		// This simulates a scenario where table name comes from user input or config
		dynamicTable := tableName // In real scenario, this could come from elsewhere

		sql, vars := surrealql.Select(models.Table(dynamicTable)).
			FieldName("name").
			Build()

		// Execute the query
		type NameResult struct {
			Name string `json:"name"`
		}

		results, err := surrealdb.Query[[]NameResult](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify we got results
		if len(*results) > 0 && (*results)[0].Result != nil {
			assert.Len(t, (*results)[0].Result, 3, "Should find all 3 users")
		}
	})

	t.Run("Select with models.Table", func(t *testing.T) {
		// Test using models.Table directly with Select
		table := models.Table(tableName)

		sql, vars := surrealql.Select(table).
			Where("active = ?", true).
			OrderBy("name").
			Build()

		// Execute the query
		type UserData struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}

		results, err := surrealdb.Query[[]UserData](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify results
		if len(*results) > 0 && (*results)[0].Result != nil {
			users := (*results)[0].Result
			assert.Len(t, users, 2, "Should find 2 active users")
			if len(users) == 2 {
				assert.Equal(t, "Alice", users[0].Name)
				assert.Equal(t, "Charlie", users[1].Name)
			}
		}
	})

	t.Run("Select models.Table with aggregation", func(t *testing.T) {
		// Test using models.Table with aggregation
		table := models.Table(tableName)

		// Count active vs inactive users
		sql, vars := surrealql.Select(table).
			Field("active").
			Field("count() AS total").
			GroupBy("active").
			OrderBy("active").
			Build()

		// Execute the query
		type CountResult struct {
			Active bool `json:"active"`
			Total  int  `json:"total"`
		}

		results, err := surrealdb.Query[[]CountResult](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Log for debugging
		if results != nil && len(*results) > 0 {
			t.Logf("Aggregation results: %+v", (*results)[0].Result)
		}
	})
}

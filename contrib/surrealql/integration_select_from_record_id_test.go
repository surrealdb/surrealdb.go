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

func TestIntegrationSelect_fromRecordID(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "select_from_record_id_test")
	ctx := context.Background()

	tableName := "record_test_data"

	// Clean up any existing data first
	cleanupQuery := fmt.Sprintf("DELETE %s", tableName)
	_, _ = surrealdb.Query[any](ctx, db, cleanupQuery, nil)

	t.Run("Select with models.RecordID", func(t *testing.T) {
		// First insert a specific record
		insertQuery := fmt.Sprintf("CREATE %s:test_user SET name = 'Test User', active = true", tableName)
		_, err := surrealdb.Query[any](ctx, db, insertQuery, nil)
		assert.NoError(t, err)

		// Test using models.RecordID directly with Select
		recordID := models.NewRecordID(tableName, "test_user")

		sql, vars := surrealql.Select(recordID).
			FieldName("name").
			FieldName("active").
			Build()

		t.Logf("RecordID query SQL: %s", sql)
		t.Logf("RecordID query vars: %+v", vars)

		// Execute the query
		type UserData struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}

		results, err := surrealdb.Query[[]UserData](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify we get the specific record
		if len(*results) > 0 && (*results)[0].Result != nil {
			users := (*results)[0].Result
			assert.Len(t, users, 1, "Should find exactly 1 user")
			if len(users) == 1 {
				assert.Equal(t, "Test User", users[0].Name)
				assert.Equal(t, true, users[0].Active)
			}
		}
	})

	t.Run("Select with models.RecordID pointer", func(t *testing.T) {
		// First insert a specific record
		insertQuery := fmt.Sprintf("CREATE %s:ptr_user SET name = 'Pointer User', active = false", tableName)
		_, err := surrealdb.Query[any](ctx, db, insertQuery, nil)
		assert.NoError(t, err)

		// Test using pointer to models.RecordID with Select
		recordID := models.NewRecordID(tableName, "ptr_user")

		sql, vars := surrealql.Select(&recordID).Build()

		t.Logf("RecordID pointer query SQL: %s", sql)
		t.Logf("RecordID pointer query vars: %+v", vars)

		// Execute the query
		type UserData struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}

		results, err := surrealdb.Query[[]UserData](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify we get the specific record
		if len(*results) > 0 && (*results)[0].Result != nil {
			users := (*results)[0].Result
			assert.Len(t, users, 1, "Should find exactly 1 user")
			if len(users) == 1 {
				assert.Equal(t, "Pointer User", users[0].Name)
				assert.Equal(t, false, users[0].Active)
			}
		}
	})

	t.Run("Select RecordID with WHERE conditions", func(t *testing.T) {
		// Insert multiple records with same ID pattern
		insertQuery1 := fmt.Sprintf("CREATE %s:order_1 SET status = 'pending', total = 100", tableName)
		insertQuery2 := fmt.Sprintf("CREATE %s:order_2 SET status = 'completed', total = 200", tableName)
		_, err := surrealdb.Query[any](ctx, db, insertQuery1, nil)
		assert.NoError(t, err)
		_, err = surrealdb.Query[any](ctx, db, insertQuery2, nil)
		assert.NoError(t, err)

		// Select specific record with additional WHERE clause
		recordID := models.NewRecordID(tableName, "order_2")

		sql, vars := surrealql.Select(recordID).
			FieldName("status").
			FieldName("total").
			Where("status = ?", "completed").
			Build()

		// Execute the query
		type OrderData struct {
			Status string `json:"status"`
			Total  int    `json:"total"`
		}

		results, err := surrealdb.Query[[]OrderData](ctx, db, sql, vars)
		assert.NoError(t, err)
		assert.NotNil(t, results)

		// Verify we get the correct record
		if len(*results) > 0 && (*results)[0].Result != nil {
			orders := (*results)[0].Result
			assert.Len(t, orders, 1, "Should find exactly 1 order")
			if len(orders) == 1 {
				assert.Equal(t, "completed", orders[0].Status)
				assert.Equal(t, 200, orders[0].Total)
			}
		}
	})
}

package surrealql_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// TestIntegrationUpdateReturnNone tests UPDATE with RETURN NONE clause
func TestIntegrationUpdateReturnNone(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := testenv.MustNewDeprecated("surrealql_test", "update_table")
	ctx := context.Background()

	// Setup: Create test records
	for i := 1; i <= 3; i++ {
		createQuery := surrealql.Create("update_table").
			Set("name", fmt.Sprintf("Item %d", i)).
			Set("value", i*10).
			Set("active", true)

		sql, vars := createQuery.Build()
		_, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Failed to create test record %d: %v", i, err)
		}
	}

	// Create an inactive record
	createQuery := surrealql.Create("update_table").
		Set("name", "Inactive Item").
		Set("value", 100).
		Set("active", false)

	sql, vars := createQuery.Build()
	_, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Failed to create inactive record: %v", err)
	}

	// Test UPDATE with RETURN NONE
	updateQuery := surrealql.Update("update_table").
		Set("updated", true).
		Where("active = ?", true).
		ReturnNone()

	sql, vars = updateQuery.Build()
	t.Logf("UPDATE SurrealQL: %s", sql)
	t.Logf("UPDATE Params: %v", vars)

	results, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}

	// With RETURN NONE, result should be empty
	if len((*results)[0].Result) != 0 {
		t.Errorf("Expected empty result with RETURN NONE, got %d items", len((*results)[0].Result))
	}

	// Verify the update worked
	selectQuery := surrealql.Select("*").FromTable("update_table").WhereEq("updated", true)
	sql, vars = selectQuery.Build()

	verifyResults, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}

	updatedRecords := (*verifyResults)[0].Result
	// We should have 3 updated records (the 3 active ones)
	if len(updatedRecords) != 3 {
		t.Errorf("Expected 3 updated records, got %d", len(updatedRecords))
	}

	// Verify inactive record was not updated
	selectInactiveQuery := surrealql.Select("*").FromTable("update_table").WhereEq("active", false)
	sql, vars = selectInactiveQuery.Build()

	inactiveResults, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("SELECT inactive failed: %v", err)
	}

	inactiveRecords := (*inactiveResults)[0].Result
	if len(inactiveRecords) == 1 {
		// Check that the inactive record doesn't have the "updated" field set to true
		if updated, exists := inactiveRecords[0]["updated"]; exists && updated == true {
			t.Errorf("Inactive record should not have been updated")
		}
	} else {
		t.Errorf("Expected 1 inactive record, got %d", len(inactiveRecords))
	}
}

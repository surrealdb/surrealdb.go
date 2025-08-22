package surrealdump

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

// GetCurrentVersionstamp gets the current versionstamp by writing to a temp table.
//
// This function is designed to handle the requirements for you,
// which prevents you from falling into the common mistakes,
// both explained in the below.
//
// Requirements:
//  1. The database connection MUST have a namespace and database selected via db.Use()
//     before creating the Dumper instance.
//  2. This function creates a temporary table with change feed enabled and IMMEDIATELY
//     writes a record to it. The change feed ONLY generates versionstamps AFTER it is
//     enabled AND when changes are made to the table.
//  3. Without the actual write operation (CREATE record), no versionstamp will be available.
//
// Common mistakes:
//   - Do NOT assume versionstamps exist just because change feeds are enabled
//   - Do NOT assume versionstamps are generated without writing data to the table
//   - ALWAYS ensure db.Use() was called before using the dumper
//   - Be aware that specific databases (namespace.database combinations) can become
//     corrupted after heavy use in test environments, where DEFINE TABLE CHANGEFEED
//     succeeds but doesn't actually enable change feeds. Use unique database names
//     in tests to avoid this SurrealDB issue.
//
// This technique works because versionstamps are monotonic and unique within a database.
//
//nolint:gocyclo,funlen // Complex versionstamp detection logic with multiple fallback paths
func (d *Dumper) GetCurrentVersionstamp(ctx context.Context) (uint64, error) {
	// First, verify we're in the right namespace/database by trying to query INFO
	// This helps catch issues where db.Use() wasn't called
	if _, err := surrealdb.Query[any](ctx, d.db, "INFO FOR DB", nil); err != nil {
		return 0, fmt.Errorf("cannot query database info - ensure db.Use() was called with namespace '%s' and database '%s': %w",
			d.namespace, d.database, err)
	}

	tempTable := fmt.Sprintf("_dump_temp_%d_%d", time.Now().UnixNano(), time.Now().Unix())

	// Create temp table with change feed enabled
	//
	// Note that just enabling change feed does NOT generate a versionstamp yet.
	//
	// We must check the Error field in the result to catch DEFINE TABLE errors.
	defineTableQuery, defineVars := surrealql.DefineTable(tempTable).Changefeed("1h").Build()
	defineTableResult, err := surrealdb.Query[[]map[string]any](ctx, d.db, defineTableQuery, defineVars)
	if err != nil {
		return 0, fmt.Errorf("failed to create temp table with change feed (check db.Use was called): %w", err)
	}
	// Check if the query returned an error in the result
	if len(*defineTableResult) > 0 && (*defineTableResult)[0].Error != nil {
		return 0, fmt.Errorf("failed to create temp table with change feed: %s", (*defineTableResult)[0].Error.Error())
	}
	// Verify we got a successful result
	if len(*defineTableResult) == 0 {
		return 0, fmt.Errorf("DEFINE TABLE returned empty result")
	}

	// Clean up temp table when done
	defer func() {
		deleteQuery := fmt.Sprintf("REMOVE TABLE %s", tempTable)
		_, _ = surrealdb.Query[any](ctx, d.db, deleteQuery, nil)
	}()

	// Insert a record to generate a versionstamp
	//
	// Note that change feeds ONLY generate versionstamps when changes are made to the table.
	// Without this insert, there will be NO versionstamp available.
	createQuery, createVars := surrealql.Create(surrealql.Thing(tempTable, "marker")).Set("timestamp = time::now()").Build()
	createResult, err := surrealdb.Query[[]map[string]any](ctx, d.db, createQuery, createVars)
	if err != nil {
		return 0, fmt.Errorf("failed to insert marker record (this is required to generate versionstamp): %w", err)
	}
	// Verify the insert succeeded
	if len(*createResult) == 0 || len((*createResult)[0].Result) == 0 {
		// Check if there's an error in the result
		if len(*createResult) > 0 && (*createResult)[0].Error != nil {
			return 0, fmt.Errorf("failed to insert marker record: %s", (*createResult)[0].Error.Error())
		}
		return 0, fmt.Errorf("insert succeeded but returned no result - database might not be properly initialized")
	}

	// Check if we can query the table to verify the record exists
	selectQuery, selectVars := surrealql.Select(tempTable).Build()
	selectResult, _ := surrealdb.Query[[]map[string]any](ctx, d.db, selectQuery, selectVars)
	if selectResult != nil && len(*selectResult) > 0 {
		recordCount := len((*selectResult)[0].Result)
		if recordCount == 0 {
			return 0, fmt.Errorf("table %s exists but has no records after insert", tempTable)
		}
	}

	// Query the change feed to get the versionstamp
	//
	// SurrealDB generates versionstamps immediately when a change is made to a table with change feed enabled.
	// However, it does not return the versionstamp as a part of the query result.
	// That's why we need to query the change feed separately for obtaining the versionstamp here.
	//
	// We retry a few times only to handle potential network/query timing issues, not because versionstamps are delayed.
	// Versionstamps are generated immediately and never delayed.
	var lastErr error
	for i := 0; i < 10; i++ {
		if i > 0 {
			// Only sleep on retries, not on first attempt
			time.Sleep(50 * time.Millisecond)
		}

		changesQuery, changesVars := surrealql.ShowChangesForTable(tempTable).SinceVersionstamp(0).Build()

		// The result from SHOW CHANGES is an array of change entries
		result, err := surrealdb.Query[[]map[string]any](ctx, d.db, changesQuery, changesVars)
		if err != nil {
			lastErr = fmt.Errorf("SHOW CHANGES query error for table %s: %w", tempTable, err)
			continue
		}

		if len(*result) > 0 && len((*result)[0].Result) > 0 {
			change := (*result)[0].Result[0]
			if vs, ok := change["versionstamp"].(uint64); ok {
				return vs, nil
			}
			// If versionstamp field exists but has unexpected type, debug it.
			// This is purely for debugging purpose, because this should never happen.
			// If happened, it indicates a bug in the SDK, the dumper or SurrealDB.
			if _, hasVs := change["versionstamp"]; hasVs {
				lastErr = fmt.Errorf("versionstamp has unexpected type: %T", change["versionstamp"])
			}
		} else {
			// This shouldn't happen and indicates a bug in the SDK, the dumper or SurrealDB.
			if len(*result) > 0 {
				lastErr = fmt.Errorf("got result but no changes (len=%d)", len((*result)[0].Result))
			} else {
				lastErr = fmt.Errorf("got empty result array")
			}
		}
	}

	if lastErr != nil {
		return 0, fmt.Errorf("no versionstamp found after retries (last error: %w)", lastErr)
	}
	return 0, fmt.Errorf("no versionstamp found after retries")
}

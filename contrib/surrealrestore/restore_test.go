package surrealrestore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
	"github.com/surrealdb/surrealdb.go/contrib/surrealrestore"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

//nolint:gocyclo // Test requires complex validation of different dump formats
func TestRestorerFull(t *testing.T) {
	ctx := context.Background()

	// Setup connection for source
	conf := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec
	conf.Logger = nil

	conn := gws.New(conf)
	sourceConn, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer sourceConn.Close(ctx)

	// Create source database with test data
	sourceDB, err := testenv.Init(sourceConn, "simple_test", "source", "test_table")
	if err != nil {
		t.Fatalf("Failed to init source db: %v", err)
	}

	// Insert test data
	type TestRecord struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	records := []TestRecord{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
		{Name: "Charlie", Age: 35},
	}

	for _, r := range records {
		_, insertErr := surrealdb.Insert[TestRecord](ctx, sourceDB, "test_table", r)
		if insertErr != nil {
			t.Fatalf("Failed to insert record: %v", insertErr)
		}
	}

	// Create dump (the current namespace/database was already set by testenv.Init)
	tempDir := t.TempDir()
	dumpPath := filepath.Join(tempDir, "dump.cbor")
	dumper := surrealdump.New(sourceDB, "simple_test", "source")
	if dumpErr := dumper.Full(ctx, dumpPath); dumpErr != nil {
		t.Fatalf("Dump failed: %v", dumpErr)
	}

	// Check dump size for logging
	if fileInfo, statErr := os.Stat(dumpPath); statErr == nil {
		t.Logf("Dump size: %d bytes", fileInfo.Size())
	}

	// Create a new connection for target to ensure complete isolation
	conf2 := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	codec2 := surrealcbor.New()
	conf2.Marshaler = codec2
	conf2.Unmarshaler = codec2
	conf2.Logger = nil

	conn2 := gws.New(conf2)
	targetConn, err := surrealdb.FromConnection(ctx, conn2)
	if err != nil {
		t.Fatalf("Failed to connect target: %v", err)
	}
	defer targetConn.Close(ctx)

	// Initialize target database (clean)
	targetDB, err := testenv.Init(targetConn, "simple_restore_target", "restored")
	if err != nil {
		t.Fatalf("Failed to init target db: %v", err)
	}

	// Before restore, check if simple_test.source exists on target connection
	if useErr := targetDB.Use(ctx, "simple_test", "source"); useErr == nil {
		// The namespace/database already exists, clean it
		preCheck, _ := surrealdb.Query[[]TestRecord](ctx, targetDB,
			"SELECT * FROM test_table", nil)
		if preCheck != nil && len(*preCheck) > 0 && len((*preCheck)[0].Result) > 0 {
			t.Logf("WARNING: Found %d existing records in target before restore!", len((*preCheck)[0].Result))
			// Clean the table
			_, _ = surrealdb.Query[any](ctx, targetDB, "DELETE test_table", nil)
		}
	}

	// Switch back to target namespace for restore
	if switchErr := targetDB.Use(ctx, "simple_restore_target", "restored"); switchErr != nil {
		t.Fatalf("Failed to switch back to target: %v", switchErr)
	}

	// Restore using the simplified API
	restorer := surrealrestore.New(targetDB)
	restorer.Verbose = true

	if restoreErr := restorer.Full(ctx, dumpPath); restoreErr != nil {
		t.Fatalf("Restore failed: %v", restoreErr)
	}

	stats := restorer.Stats()
	t.Logf("Restored %d records in %d tables", stats.RecordsRestored, stats.TablesRestored)

	// The restore creates the namespace/database from the dump, so we need to switch to that
	if useRestoreErr := targetDB.Use(ctx, "simple_test", "source"); useRestoreErr != nil {
		t.Fatalf("Failed to use restored db: %v", useRestoreErr)
	}

	// First, let's see what's actually in the test_table
	checkResult, err := surrealdb.Query[[]TestRecord](ctx, targetDB,
		"SELECT * FROM test_table", nil)
	if err != nil {
		t.Fatalf("Failed to check records: %v", err)
	}

	t.Logf("Raw check - found %d result sets", len(*checkResult))
	for i, resultSet := range *checkResult {
		t.Logf("  Result set %d has %d records", i, len(resultSet.Result))
	}

	// Count records
	type CountResult struct {
		Count int `json:"count"`
	}

	result, err := surrealdb.Query[[]CountResult](ctx, targetDB,
		"SELECT count() as count FROM test_table GROUP ALL", nil)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if len(*result) > 0 && len((*result)[0].Result) > 0 {
		count := (*result)[0].Result[0].Count
		if count != 3 {
			t.Errorf("Expected 3 records, got %d", count)
		} else {
			t.Logf("Successfully restored %d records", count)
		}
	}

	// Select all records
	allRecords, err := surrealdb.Query[[]TestRecord](ctx, targetDB,
		"SELECT * FROM test_table", nil)
	if err != nil {
		t.Fatalf("Failed to select records: %v", err)
	}

	if len(*allRecords) > 0 {
		t.Logf("Found %d records", len((*allRecords)[0].Result))
		for _, r := range (*allRecords)[0].Result {
			t.Logf("  - %s (age %d)", r.Name, r.Age)
		}
	}
}

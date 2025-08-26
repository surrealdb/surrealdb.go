package surrealdump_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// TestComprehensiveSurrealdumpAPIs tests all surrealdump APIs comprehensively:
// 1. New() - Creates dumper instance (for GetCurrentVersionstamp)
// 2. Do() - Creates full dump with manifest using the external API
// 3. Do() - Creates incremental dumps with manifests using the external API
// 4. ReadManifest() - Reads and validates manifest files
// 5. ReadManifest() - Reads and validates manifests
// 6. ScanChains() - Scans directory and builds valid chains
// 7. Chain.Validate() - Validates chain consistency
// 8. Chain.GetPointInTimeOptions() - Lists available restore points
// 9. Chain.GetDumpsForVersionstamp() - Gets dumps needed for specific point
// 10. GetCurrentVersionstamp() - Gets current datastart versionstamp
// 11. CanApplyIncremental() - Validates incremental dump applicability
// 12. Verifies dumps without manifests are ignored
//
//nolint:gocyclo // Comprehensive integration test requires complex workflow validation
func TestComprehensiveSurrealdumpAPIs(t *testing.T) {
	ctx := context.Background()

	// Setup connection
	conf := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec
	conf.Logger = nil

	conn := gws.New(conf)
	db, err := surrealdb.FromConnection(ctx, conn)
	require.NoError(t, err, "Failed to connect")
	defer db.Close(ctx)

	// We observed that SurrealDB running locally with in-memory datastore stops
	// serving change feed sometimes.
	// More concretely, after repeated table creation and removal in the same database, we found that:
	// - DEFINE TABLE ... CHANGEFEED succeeds but doesn't create working change feeds
	// - SHOW CHANGES FOR TABLE always returns 0 changes, even after inserts
	// - The behavior persists even after REMOVE TABLE and recreating tables
	// - The behavior appears to be at the database level and persists until SurrealDB is restarted
	// To avoid this issue in tests, we use unique database names for each test run.
	ns := "test_comprehensive"
	testdb := fmt.Sprintf("comprehensive_db_%d", time.Now().UnixNano())

	// Initialize database
	_, initErr := testenv.Init(db, ns, testdb)
	require.NoError(t, initErr, "Failed to init test environment")

	// Create temporary directory for dumps
	tempDir := t.TempDir()

	// Shared state across sub-tests
	var dumper *surrealdump.Dumper
	var fullDumpPath string
	var fullManifest *surrealdump.Manifest
	var inc1DumpPath string
	var inc1Manifest *surrealdump.Manifest
	var inc2DumpPath string
	var inc2Manifest *surrealdump.Manifest
	var chains []*surrealdump.Chain

	t.Run("New_Creates_Dumper_Instance", func(t *testing.T) {
		dumper = surrealdump.New(db, ns, testdb, "products", "users", "orders")
		require.NotNil(t, dumper, "New() returned nil dumper")
	})

	t.Run("Setup_Enable_Change_Feeds", func(t *testing.T) {
		_, feedErr := surrealdb.Query[any](ctx, db, `
			DEFINE TABLE products CHANGEFEED 1h;
			DEFINE TABLE users CHANGEFEED 1h;
			DEFINE TABLE orders CHANGEFEED 1h;
		`, nil)
		require.NoError(t, feedErr, "Failed to enable change feeds")
	})

	t.Run("Setup_Create_Initial_Data", func(t *testing.T) {
		// Define Product struct for API insertion
		type Product struct {
			ID    string  `json:"id,omitempty"`
			Name  string  `json:"name"`
			Price float64 `json:"price"`
			Stock int     `json:"stock"`
		}

		// Insert using API
		products := []Product{
			{Name: "Laptop", Price: 999.99, Stock: 10},
			{Name: "Mouse", Price: 29.99, Stock: 50},
			{Name: "Keyboard", Price: 79.99, Stock: 30},
		}

		for _, p := range products {
			_, insertErr := surrealdb.Insert[Product](ctx, db, "products", p)
			require.NoError(t, insertErr, "Failed to insert product")
		}

		// Insert using raw queries with specific IDs
		_, createErr := surrealdb.Query[any](ctx, db, `
			CREATE products:phone SET name = "Phone", price = 599.99, stock = 20;
			CREATE users:alice SET name = "Alice", role = "admin", age = 30;
			CREATE users:bob SET name = "Bob", role = "user", age = 25;
			CREATE orders:order1 SET user = users:alice, product = products:laptop, quantity = 1;
		`, nil)
		require.NoError(t, createErr, "Failed to create initial data")
	})

	t.Run("Do_Incremental_With_Auto_Detect_Fails_Before_Full_Dump", func(t *testing.T) {
		// Try to create an incremental dump with auto-detection before any full dump exists
		// This should fail because there are no previous dumps to detect from
		failedIncPath := filepath.Join(tempDir, "dump-000-inc-fail.cbor")

		config := &surrealdump.Config{
			Endpoint:          testenv.MustParseSurrealDBWSURL().String(),
			Username:          "root",
			Password:          "root",
			Namespace:         ns,
			Database:          testdb,
			Output:            failedIncPath,
			Incremental:       true, // Incremental dump
			SinceVersionstamp: 0,    // Zero triggers auto-detection, but should fail
			Tables:            []string{"products", "users", "orders"},
			Verbose:           false,
		}

		err := surrealdump.Do(ctx, config)
		assert.Error(t, err, "Should fail to create incremental dump when no previous dumps exist")
		assert.Contains(t, err.Error(), "no start versionstamp specified and no previous dumps found",
			"Error should indicate that no previous dumps were found for auto-detection")

		// Verify the failed dump file was not created
		_, statErr := os.Stat(failedIncPath)
		assert.True(t, os.IsNotExist(statErr), "Failed incremental dump file should not exist")

		t.Logf("Auto-detection correctly failed before any dumps exist")
	})

	t.Run("Do_Creates_Full_Dump_With_Manifest", func(t *testing.T) {
		fullDumpPath = filepath.Join(tempDir, "dump-001-full.cbor")

		// Use Do function with Config for full dump
		config := &surrealdump.Config{
			Endpoint:    testenv.MustParseSurrealDBWSURL().String(),
			Username:    "root",
			Password:    "root",
			Namespace:   ns,
			Database:    testdb,
			Output:      fullDumpPath,
			Incremental: false, // Full dump
			Tables:      []string{"products", "users", "orders"},
			Verbose:     false, // Suppress verbose logging in tests
		}

		fullErr := surrealdump.Do(ctx, config)
		require.NoError(t, fullErr, "Failed to create full dump using Do")

		// Verify dump file exists and has content
		fullDumpData, err := os.ReadFile(fullDumpPath)
		require.NoError(t, err, "Failed to read full dump file")
		assert.NotEmpty(t, fullDumpData, "Full dump file is empty")
	})

	t.Run("Manifest_Contains_Expected_Information", func(t *testing.T) {
		manifest, err := surrealdump.ReadManifest(fullDumpPath)
		require.NoError(t, err, "Failed to read manifest")
		assert.Equal(t, surrealdump.ManifestTypeFull, manifest.Type, "Expected full dump type")
		assert.Equal(t, ns, manifest.Namespace, "Expected namespace")
		assert.Equal(t, testdb, manifest.Database, "Expected database")
		assert.NotZero(t, manifest.EndVersionstamp, "EndVersionstamp should not be zero")
		assert.NotEmpty(t, manifest.SHA256, "SHA256 should not be empty")
	})

	t.Run("ReadManifest_Reads_And_Validates_Full_Manifest", func(t *testing.T) {
		var err error
		fullManifest, err = surrealdump.ReadManifest(fullDumpPath)
		require.NoError(t, err, "Failed to read full dump manifest")

		// Comprehensive manifest validation
		assert.Equal(t, filepath.Base(fullDumpPath), fullManifest.Filename, "Expected filename")
		assert.Equal(t, surrealdump.ManifestTypeFull, fullManifest.Type, "Expected type")
		assert.Equal(t, ns, fullManifest.Namespace, "Expected namespace")
		assert.Equal(t, testdb, fullManifest.Database, "Expected database")
		assert.Zero(t, fullManifest.StartVersionstamp, "Full dump should have zero StartVersionstamp")
		assert.NotEmpty(t, fullManifest.SHA256, "Manifest missing SHA256 hash")
		assert.NotZero(t, fullManifest.Size, "Manifest has zero size")
		assert.False(t, fullManifest.CreatedAt.IsZero(), "Manifest has zero CreatedAt time")
		assert.NotZero(t, fullManifest.EndVersionstamp, "Manifest has zero EndVersionstamp")

		t.Logf("Full dump created: %s (vs: 0-%d, size: %d bytes, SHA256: %s)",
			fullManifest.Filename, fullManifest.EndVersionstamp, fullManifest.Size, fullManifest.SHA256)
	})

	t.Run("Do_Creates_First_Incremental_Dump", func(t *testing.T) {
		_, changeErr := surrealdb.Query[any](ctx, db, `
			UPDATE products:laptop SET stock = 5, last_sold = time::now();
			CREATE products:tablet SET name = "Tablet", price = 799.99, stock = 15;
			DELETE users:bob;
			UPDATE products SET on_sale = true WHERE price > 900;
		`, nil)
		require.NoError(t, changeErr, "Failed to make first changes")

		inc1DumpPath = filepath.Join(tempDir, "dump-002-inc.cbor")

		// Use Do function with Config for incremental dump
		config := &surrealdump.Config{
			Endpoint:          testenv.MustParseSurrealDBWSURL().String(),
			Username:          "root",
			Password:          "root",
			Namespace:         ns,
			Database:          testdb,
			Output:            inc1DumpPath,
			Incremental:       true, // Incremental dump
			SinceVersionstamp: fullManifest.EndVersionstamp,
			Tables:            []string{"products", "users", "orders"},
			Verbose:           false, // Suppress verbose logging in tests
		}

		inc1Err := surrealdump.Do(ctx, config)
		require.NoError(t, inc1Err, "Failed to create first incremental dump using Do")

		// Verify incremental dump file
		inc1Data, err := os.ReadFile(inc1DumpPath)
		require.NoError(t, err, "Failed to read incremental dump file")
		assert.NotEmpty(t, inc1Data, "Incremental dump file is empty")
	})

	t.Run("ReadManifest_Reads_First_Incremental_Manifest", func(t *testing.T) {
		var err error
		inc1Manifest, err = surrealdump.ReadManifest(inc1DumpPath)
		require.NoError(t, err, "Failed to read first incremental manifest")

		// Validate incremental manifest fields
		assert.Equal(t, surrealdump.ManifestTypeIncremental, inc1Manifest.Type, "Expected type incremental")
		assert.Equal(t, fullManifest.EndVersionstamp, inc1Manifest.StartVersionstamp, "StartVersionstamp mismatch")
		assert.Greater(t, inc1Manifest.EndVersionstamp, inc1Manifest.StartVersionstamp, "EndVersionstamp should be greater than StartVersionstamp")

		t.Logf("First incremental dump created: %s (vs: %d-%d, size: %d bytes)",
			inc1Manifest.Filename, inc1Manifest.StartVersionstamp, inc1Manifest.EndVersionstamp, inc1Manifest.Size)
	})

	t.Run("Do_Creates_Second_Incremental_Dump", func(t *testing.T) {
		_, change2Err := surrealdb.Query[any](ctx, db, `
			UPDATE products:phone SET price = 549.99, on_sale = true;
			CREATE products:watch SET name = "Smart Watch", price = 299.99, stock = 30;
			CREATE users:charlie SET name = "Charlie", role = "moderator", age = 35;
			DELETE products WHERE name = "Mouse";
			CREATE orders:order2 SET user = users:charlie, product = products:watch, quantity = 2;
		`, nil)
		require.NoError(t, change2Err, "Failed to make second changes")

		inc2DumpPath = filepath.Join(tempDir, "dump-003-inc.cbor")

		// Use Do function with Config for second incremental dump
		config := &surrealdump.Config{
			Endpoint:          testenv.MustParseSurrealDBWSURL().String(),
			Username:          "root",
			Password:          "root",
			Namespace:         ns,
			Database:          testdb,
			Output:            inc2DumpPath,
			Incremental:       true, // Incremental dump
			SinceVersionstamp: inc1Manifest.EndVersionstamp,
			Tables:            []string{"products", "users", "orders"},
			Verbose:           false, // Suppress verbose logging in tests
		}

		inc2Err := surrealdump.Do(ctx, config)
		require.NoError(t, inc2Err, "Failed to create second incremental dump using Do")
	})

	t.Run("ReadManifest_Reads_Second_Incremental_Manifest", func(t *testing.T) {
		var err error
		inc2Manifest, err = surrealdump.ReadManifest(inc2DumpPath)
		require.NoError(t, err, "Failed to read second incremental manifest")

		// Verify second incremental builds on first
		assert.Equal(t, inc1Manifest.EndVersionstamp, inc2Manifest.StartVersionstamp, "Second incremental base doesn't match first incremental max")

		t.Logf("Second incremental dump created: %s (vs: %d-%d, size: %d bytes)",
			inc2Manifest.Filename, inc2Manifest.StartVersionstamp, inc2Manifest.EndVersionstamp, inc2Manifest.Size)
	})

	t.Run("Create_Orphan_Dump_Without_Manifest", func(t *testing.T) {
		orphanPath := filepath.Join(tempDir, "dump-orphan.cbor")
		writeErr := os.WriteFile(orphanPath, []byte("SURDUMP01fake_content"), 0600)
		require.NoError(t, writeErr, "Failed to create orphan dump")

		// Also create an invalid manifest to test error handling
		invalidManifestPath := filepath.Join(tempDir, "dump-invalid.cbor.manifest")
		writeErr = os.WriteFile(invalidManifestPath, []byte("invalid json"), 0600)
		require.NoError(t, writeErr, "Failed to create invalid manifest")
	})

	t.Run("ScanChains_Scans_Directory_And_Builds_Chains", func(t *testing.T) {
		var err error
		chains, err = surrealdump.ScanChains(tempDir)
		require.NoError(t, err, "Failed to scan and build chains")
		require.Len(t, chains, 1, "Expected 1 chain")
	})

	t.Run("Chain_Validate_Validates_Chain_Consistency", func(t *testing.T) {
		chain := chains[0]

		require.NotNil(t, chain.FullDump, "Chain missing full dump")
		assert.Equal(t, "dump-001-full.cbor", chain.FullDump.Filename, "Expected full dump filename")

		assert.Len(t, chain.IncrementalDumps, 2, "Expected 2 incremental dumps in chain")

		// Verify incremental dumps are in correct order
		if len(chain.IncrementalDumps) >= 1 {
			assert.Equal(t, "dump-002-inc.cbor", chain.IncrementalDumps[0].Filename, "First incremental filename")
		}
		if len(chain.IncrementalDumps) >= 2 {
			assert.Equal(t, "dump-003-inc.cbor", chain.IncrementalDumps[1].Filename, "Second incremental filename")
		}

		// Explicitly test Validate() method
		validateErr := chain.Validate()
		assert.NoError(t, validateErr, "Chain validation failed")

		// Verify chain metadata
		expectedSize := fullManifest.Size + inc1Manifest.Size + inc2Manifest.Size
		assert.Equal(t, expectedSize, chain.TotalSize, "Chain total size mismatch")
		assert.Equal(t, inc2Manifest.EndVersionstamp, chain.LatestVersionstamp, "Chain latest versionstamp mismatch")
	})

	t.Run("Chain_GetPointInTimeOptions_Retrieves_Restore_Points", func(t *testing.T) {
		chain := chains[0]
		points := chain.GetRestorationPoints()

		assert.Len(t, points, 3, "Expected 3 restore points (1 full + 2 incremental)")

		// Verify points are in order
		if len(points) >= 3 {
			assert.Equal(t, fullManifest.EndVersionstamp, points[0], "First point should be full dump versionstamp")
			assert.Equal(t, inc1Manifest.EndVersionstamp, points[1], "Second point should be first incremental versionstamp")
			assert.Equal(t, inc2Manifest.EndVersionstamp, points[2], "Third point should be second incremental versionstamp")
		}
	})

	t.Run("Chain_GetDumpsForVersionstamp_Tests_Dump_Selection", func(t *testing.T) {
		chain := chains[0]

		// Test restoration to full dump point
		dumpsForFull, err := chain.GetManifestsForVersionstamp(fullManifest.EndVersionstamp)
		assert.NoError(t, err, "Failed to get dumps for full versionstamp")
		assert.Len(t, dumpsForFull, 1, "Expected 1 dump for full restore point")
		if len(dumpsForFull) > 0 {
			assert.Equal(t, "dump-001-full.cbor", dumpsForFull[0].Filename, "Expected full dump for full restore point")
		}

		// Test restoration to first incremental point
		dumpsForInc1, err := chain.GetManifestsForVersionstamp(inc1Manifest.EndVersionstamp)
		assert.NoError(t, err, "Failed to get dumps for first incremental")
		assert.Len(t, dumpsForInc1, 2, "Expected 2 dumps for first incremental restore point")

		// Test restoration to latest point
		dumpsForLatest, err := chain.GetManifestsForVersionstamp(chain.LatestVersionstamp)
		assert.NoError(t, err, "Failed to get dumps for latest point")
		assert.Len(t, dumpsForLatest, 3, "Expected 3 dumps for latest restore point")

		// Test error case: versionstamp before full dump
		if fullManifest.EndVersionstamp > 0 {
			_, err := chain.GetManifestsForVersionstamp(fullManifest.EndVersionstamp - 1)
			assert.Error(t, err, "Expected error when requesting versionstamp before full dump")
		}
	})

	t.Run("GetCurrentVersionstamp_Retrieves_Current_DB_Versionstamp", func(t *testing.T) {
		currentVs, err := dumper.GetCurrentVersionstamp(ctx)
		assert.NoError(t, err, "Failed to get current versionstamp")
		assert.Greater(t, currentVs, inc2Manifest.EndVersionstamp, "Current versionstamp should be greater than last dump")
		t.Logf("Current datastart versionstamp: %d", currentVs)
	})

	t.Run("CanApplyIncremental_Checks_Incremental_Applicability", func(t *testing.T) {
		// Should be able to apply inc1 after full dump
		err := surrealdump.CanApplyIncremental(fullManifest.EndVersionstamp, inc1Manifest)
		assert.NoError(t, err, "Should be able to apply first incremental after full dump")

		// Should be able to apply inc2 after inc1
		err = surrealdump.CanApplyIncremental(inc1Manifest.EndVersionstamp, inc2Manifest)
		assert.NoError(t, err, "Should be able to apply second incremental after first")

		// Should NOT be able to apply inc2 directly after full (gap)
		err = surrealdump.CanApplyIncremental(fullManifest.EndVersionstamp, inc2Manifest)
		assert.Error(t, err, "Should not be able to apply second incremental directly after full dump (gap)")

		// Should NOT be able to apply full dump as incremental
		err = surrealdump.CanApplyIncremental(fullManifest.EndVersionstamp, fullManifest)
		assert.Error(t, err, "Should not be able to apply full dump as incremental")
	})

	t.Run("Do_Creates_Incremental_With_Auto_Detected_Versionstamp", func(t *testing.T) {
		// Make some additional changes
		_, changeErr := surrealdb.Query[any](ctx, db, `
			UPDATE products:tablet SET price = 699.99;
			CREATE products:headphones SET name = "Wireless Headphones", price = 199.99, stock = 25;
			CREATE users:david SET name = "David", role = "user", age = 28;
		`, nil)
		require.NoError(t, changeErr, "Failed to make changes for auto-detected incremental")

		// Create a new incremental dump with zero versionstamp to trigger auto-detection
		inc3DumpPath := filepath.Join(tempDir, "dump-004-inc-auto.cbor")

		// Use Do function with Config for incremental dump with auto-detection
		// Note: Output path is already in tempDir, and Config.FindLatestVersionstamp will
		// use the directory of the output path when Dir is empty
		config := &surrealdump.Config{
			Endpoint:          testenv.MustParseSurrealDBWSURL().String(),
			Username:          "root",
			Password:          "root",
			Namespace:         ns,
			Database:          testdb,
			Output:            inc3DumpPath,
			Dir:               "",   // Empty Dir - FindLatestVersionstamp will use directory of Output
			Incremental:       true, // Incremental dump
			SinceVersionstamp: 0,    // Zero triggers auto-detection
			Tables:            []string{"products", "users", "orders"},
			Verbose:           false, // Suppress verbose logging in tests
		}

		inc3Err := surrealdump.Do(ctx, config)
		require.NoError(t, inc3Err, "Failed to create incremental dump with auto-detected versionstamp")

		// Verify the dump was created
		inc3Data, err := os.ReadFile(inc3DumpPath)
		require.NoError(t, err, "Failed to read auto-detected incremental dump file")
		assert.NotEmpty(t, inc3Data, "Auto-detected incremental dump file is empty")

		// Read and validate the manifest
		inc3Manifest, err := surrealdump.ReadManifest(inc3DumpPath)
		require.NoError(t, err, "Failed to read auto-detected incremental manifest")

		// Verify it's an incremental dump
		assert.Equal(t, surrealdump.ManifestTypeIncremental, inc3Manifest.Type, "Expected type incremental")

		// Verify the auto-detected start versionstamp matches the latest previous dump
		assert.Equal(t, inc2Manifest.EndVersionstamp, inc3Manifest.StartVersionstamp,
			"Auto-detected start versionstamp should match the previous incremental's end versionstamp")

		// Verify the new versionstamp is greater than the base
		assert.Greater(t, inc3Manifest.EndVersionstamp, inc3Manifest.StartVersionstamp,
			"EndVersionstamp should be greater than StartVersionstamp")

		t.Logf("Auto-detected incremental dump created: %s (vs: %d-%d, size: %d bytes)",
			inc3Manifest.Filename, inc3Manifest.StartVersionstamp, inc3Manifest.EndVersionstamp, inc3Manifest.Size)
	})

	t.Run("Dumps_Without_Manifests_Correctly_Ignored", func(t *testing.T) {
		// Check that no chain contains the orphan dump
		for _, c := range chains {
			if c.FullDump != nil {
				assert.NotEqual(t, "dump-orphan.cbor", c.FullDump.Filename, "Orphan dump without manifest should have been ignored")
			}
			for _, inc := range c.IncrementalDumps {
				assert.NotEqual(t, "dump-orphan.cbor", inc.Filename, "Orphan dump without manifest should have been ignored")
			}
		}

		// Verify that files in the directory that aren't dumps are handled correctly
		files, _ := os.ReadDir(tempDir)
		t.Logf("Directory contains %d files total", len(files))
	})

	t.Run("All_Manifests_Contain_Required_Metadata", func(t *testing.T) {
		// Verify full manifest has required fields
		assert.NotEmpty(t, fullManifest.SHA256, "Full manifest missing SHA256")
		assert.NotZero(t, fullManifest.Size, "Full manifest missing size")
		assert.NotZero(t, fullManifest.EndVersionstamp, "Full manifest missing EndVersionstamp")

		// Verify incremental manifests have required fields
		assert.NotEmpty(t, inc1Manifest.SHA256, "First incremental manifest missing SHA256")
		assert.NotZero(t, inc1Manifest.Size, "First incremental manifest missing size")
		assert.NotZero(t, inc1Manifest.EndVersionstamp, "First incremental manifest missing EndVersionstamp")

		assert.NotEmpty(t, inc2Manifest.SHA256, "Second incremental manifest missing SHA256")
		assert.NotZero(t, inc2Manifest.Size, "Second incremental manifest missing size")
		assert.NotZero(t, inc2Manifest.EndVersionstamp, "Second incremental manifest missing EndVersionstamp")
	})
}

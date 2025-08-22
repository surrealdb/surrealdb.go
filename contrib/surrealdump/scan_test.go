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

//nolint:gocyclo // Test requires complex scenario validation
func TestScanChains_integration(t *testing.T) {
	ctx := context.Background()

	// Setup connection
	conf := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec
	conf.Logger = nil

	conn := gws.New(conf)
	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close(ctx)

	ns := "test_scan"
	testdb := fmt.Sprintf("scan_db_%d", time.Now().UnixNano())

	// Initialize database
	if _, initErr := testenv.Init(db, ns, testdb); initErr != nil {
		t.Fatalf("Failed to init test environment: %v", initErr)
	}

	// Create temporary directory for dumps
	tempDir := t.TempDir()

	// Test 1: Empty directory should return no chains
	t.Run("EmptyDirectory", func(t *testing.T) {
		chains, scanErr := surrealdump.ScanChains(tempDir)
		if scanErr != nil {
			t.Fatalf("ScanChains failed: %v", scanErr)
		}
		if len(chains) != 0 {
			t.Errorf("Expected 0 chains, got %d", len(chains))
		}
	})

	// Test 2: Directory with dumps and manifests
	t.Run("WithDumpsAndManifests", func(t *testing.T) {
		// Enable change feeds
		if _, feedErr := surrealdb.Query[any](ctx, db, `
			DEFINE TABLE products CHANGEFEED 1h;
		`, nil); feedErr != nil {
			t.Fatalf("Failed to enable change feed: %v", feedErr)
		}

		// Create initial data
		if _, createErr := surrealdb.Query[any](ctx, db, `
			CREATE products:laptop SET name = "Laptop", price = 999.99;
			CREATE products:phone SET name = "Phone", price = 599.99;
		`, nil); createErr != nil {
			t.Fatalf("Failed to create data: %v", createErr)
		}

		// Create full dump
		fullPath := filepath.Join(tempDir, "full.cbor")
		dumper := surrealdump.New(db, ns, testdb)
		if fullErr := dumper.Full(ctx, fullPath); fullErr != nil {
			t.Fatalf("Failed to create full dump: %v", fullErr)
		}

		// Make changes for incremental dump
		if _, changeErr := surrealdb.Query[any](ctx, db, `
			CREATE products:tablet SET name = "Tablet", price = 799.99;
		`, nil); changeErr != nil {
			t.Fatalf("Failed to create tablet: %v", changeErr)
		}

		// Get versionstamp from full dump manifest
		fullManifest, readErr := surrealdump.ReadManifest(fullPath)
		if readErr != nil {
			t.Fatalf("Failed to read full manifest: %v", readErr)
		}

		// Create incremental dump
		incPath := filepath.Join(tempDir, "inc.cbor")
		if incErr := dumper.Incremental(ctx, incPath, fullManifest.EndVersionstamp); incErr != nil {
			t.Fatalf("Failed to create incremental dump: %v", incErr)
		}

		// Test ScanChains (always validates)
		chains, scanErr := surrealdump.ScanChains(tempDir)
		if scanErr != nil {
			t.Fatalf("ScanChains failed: %v", scanErr)
		}

		if len(chains) != 1 {
			t.Fatalf("Expected 1 chain, got %d", len(chains))
		}

		chain := chains[0]
		if chain.FullDump == nil {
			t.Error("Chain missing full dump")
		}
		if len(chain.IncrementalDumps) != 1 {
			t.Errorf("Expected 1 incremental dump, got %d", len(chain.IncrementalDumps))
		}

		// Chains are already validated by ScanChains
	})

	// Test 3: Directory with dump but no manifest (should be ignored)
	t.Run("DumpWithoutManifest", func(t *testing.T) {
		orphanDir := t.TempDir()
		orphanPath := filepath.Join(orphanDir, "orphan.cbor")
		if writeErr := os.WriteFile(orphanPath, []byte("SURDUMP01fake"), 0600); writeErr != nil {
			t.Fatalf("Failed to create orphan dump: %v", writeErr)
		}

		chains, scanErr := surrealdump.ScanChains(orphanDir)
		if scanErr != nil {
			t.Fatalf("ScanChains failed: %v", scanErr)
		}

		if len(chains) != 0 {
			t.Errorf("Expected 0 chains (orphan should be ignored), got %d", len(chains))
		}
	})

	// Test 4: Invalid chain should fail validation
	t.Run("InvalidChainValidation", func(t *testing.T) {
		invalidDir := t.TempDir()

		// Create a full dump manifest first
		fullManifest := &surrealdump.Manifest{
			Filename:          "full.cbor",
			Type:              surrealdump.ManifestTypeFull,
			Namespace:         ns,
			Database:          testdb,
			EndVersionstamp:   100,
			StartVersionstamp: 0,
		}

		fullPath := filepath.Join(invalidDir, "full.cbor")
		if writeErr := os.WriteFile(fullPath, []byte("SURDUMP01fake"), 0600); writeErr != nil {
			t.Fatalf("Failed to create full dump file: %v", writeErr)
		}
		if manifestErr := surrealdump.WriteManifest(fullPath, fullManifest); manifestErr != nil {
			t.Fatalf("Failed to write full manifest: %v", manifestErr)
		}

		// Create an incremental manifest with invalid versionstamp range
		incManifest := &surrealdump.Manifest{
			Filename:          "inc.cbor",
			Type:              surrealdump.ManifestTypeIncremental,
			Namespace:         ns,
			Database:          testdb,
			EndVersionstamp:   120, // Invalid: max < base
			StartVersionstamp: 150, // Invalid: base > max
		}

		incPath := filepath.Join(invalidDir, "inc.cbor")
		if writeIncErr := os.WriteFile(incPath, []byte("SURINC01fake"), 0600); writeIncErr != nil {
			t.Fatalf("Failed to create inc dump file: %v", writeIncErr)
		}
		if incManifestErr := surrealdump.WriteManifest(incPath, incManifest); incManifestErr != nil {
			t.Fatalf("Failed to write inc manifest: %v", incManifestErr)
		}

		// ScanChains should succeed but exclude the invalid incremental dump
		chains, err := surrealdump.ScanChains(invalidDir)
		if err != nil {
			t.Fatalf("ScanChains failed: %v", err)
		}

		// Should have one chain with only the full dump (invalid incremental is excluded)
		if len(chains) != 1 {
			t.Errorf("Expected 1 chain, got %d", len(chains))
		} else {
			chain := chains[0]
			if chain.FullDump == nil {
				t.Error("Chain missing full dump")
			}
			if len(chain.IncrementalDumps) != 0 {
				t.Error("Invalid incremental dump should not be included in chain")
			}
			t.Log("Invalid incremental correctly excluded from chain")
		}
	})
}

func TestScanChains(t *testing.T) {
	// Test chain building through ScanChains
	tempDir := t.TempDir()

	// Use UTC time for consistent comparison
	now := time.Now().UTC().Round(time.Second)

	// Create manifests for two different databases
	db1Full := &surrealdump.Manifest{
		Filename:          "db1-001.cbor",
		Type:              surrealdump.ManifestTypeFull,
		Namespace:         "ns1",
		Database:          "db1",
		EndVersionstamp:   200,
		StartVersionstamp: 0,
		Size:              1000,
		CreatedAt:         now,
	}

	db1Inc1 := &surrealdump.Manifest{
		Filename:          "db1-002.cbor",
		Type:              surrealdump.ManifestTypeIncremental,
		Namespace:         "ns1",
		Database:          "db1",
		EndVersionstamp:   300,
		StartVersionstamp: 200,
		Size:              500,
		CreatedAt:         now,
	}

	db2Full := &surrealdump.Manifest{
		Filename:          "db2-001.cbor",
		Type:              surrealdump.ManifestTypeFull,
		Namespace:         "ns1",
		Database:          "db2",
		EndVersionstamp:   250,
		StartVersionstamp: 0,
		Size:              800,
		CreatedAt:         now,
	}

	// Create dump files and manifests
	for _, manifest := range []*surrealdump.Manifest{db1Full, db1Inc1, db2Full} {
		dumpPath := filepath.Join(tempDir, manifest.Filename)
		err := os.WriteFile(dumpPath, []byte("SURDUMP01fake"), 0600)
		require.NoError(t, err)
		err = surrealdump.WriteManifest(dumpPath, manifest)
		require.NoError(t, err)
	}

	// Use ScanChains which internally calls buildChains
	chains, err := surrealdump.ScanChains(tempDir)
	require.NoError(t, err, "Failed to scan and build chains")
	assert.Len(t, chains, 2, "Should build 2 chains for 2 databases")

	// Find chains by database
	var db1Chain, db2Chain *surrealdump.Chain
	for _, chain := range chains {
		switch chain.FullDump.Database {
		case "db1":
			db1Chain = chain
		case "db2":
			db2Chain = chain
		}
	}

	require.NotNil(t, db1Chain, "Should have chain for db1")
	require.NotNil(t, db2Chain, "Should have chain for db2")

	// Verify db1 chain
	assert.Equal(t, db1Full, db1Chain.FullDump)
	assert.Len(t, db1Chain.IncrementalDumps, 1)
	assert.Equal(t, db1Inc1, db1Chain.IncrementalDumps[0])
	assert.Equal(t, uint64(300), db1Chain.LatestVersionstamp)

	// Verify db2 chain
	assert.Equal(t, db2Full, db2Chain.FullDump)
	assert.Len(t, db2Chain.IncrementalDumps, 0)
	assert.Equal(t, uint64(250), db2Chain.LatestVersionstamp)
}

func TestScanChains_onlyProcessDumpsWithManifests(t *testing.T) {
	// This test verifies that ScanChains only processes dumps that have manifests.
	// Dumps without manifests are not valid and cannot be processed since we need
	// the metadata to build and validate chains.
	tempDir := t.TempDir()

	// Create a valid full dump with manifest
	dumpPath1 := filepath.Join(tempDir, "dump1.cbor")
	err := os.WriteFile(dumpPath1, []byte("SURDUMP01..."), 0600)
	require.NoError(t, err)

	// Create manifest for the valid dump
	manifest1 := &surrealdump.Manifest{
		Filename:          "dump1.cbor",
		Type:              surrealdump.ManifestTypeFull,
		CreatedAt:         time.Now().UTC().Round(time.Second),
		Size:              12,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   200,
		StartVersionstamp: 0,
	}
	err = surrealdump.WriteManifest(dumpPath1, manifest1)
	require.NoError(t, err)

	// Create a dump file without manifest - this represents a corrupted or incomplete dump
	// that should not be processed
	orphanDumpPath := filepath.Join(tempDir, "orphan.cbor")
	err = os.WriteFile(orphanDumpPath, []byte("SURINC01..."), 0600)
	require.NoError(t, err)

	// ScanChains should only find and process the dump with a manifest
	chains, err := surrealdump.ScanChains(tempDir)
	require.NoError(t, err)

	assert.Equal(t, 1, len(chains), "Should find exactly one valid chain")

	// Check the chain
	if len(chains) > 0 {
		chain := chains[0]
		assert.NotNil(t, chain.FullDump)
		assert.Equal(t, "dump1.cbor", chain.FullDump.Filename)
		assert.Equal(t, manifest1.Type, chain.FullDump.Type)
		assert.Equal(t, manifest1.Namespace, chain.FullDump.Namespace)
		assert.Equal(t, manifest1.Database, chain.FullDump.Database)
		assert.Equal(t, manifest1.StartVersionstamp, chain.FullDump.StartVersionstamp)
		assert.Equal(t, manifest1.EndVersionstamp, chain.FullDump.EndVersionstamp)
	}
}

func TestScanChainsExcludesIncrementalDumpsWithGaps(t *testing.T) {
	// This test verifies that ScanChains excludes incremental dumps that would create gaps
	// in the chain. Only continuous sequences of dumps are included in valid chains.
	tempDir := t.TempDir()

	// Create a full dump
	fullPath := filepath.Join(tempDir, "full.cbor")
	err := os.WriteFile(fullPath, []byte("SURDUMP01..."), 0600)
	require.NoError(t, err)

	fullManifest := &surrealdump.Manifest{
		Filename:          "full.cbor",
		Type:              surrealdump.ManifestTypeFull,
		CreatedAt:         time.Now().UTC().Round(time.Second),
		Size:              100,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   200,
		StartVersionstamp: 0,
	}
	err = surrealdump.WriteManifest(fullPath, fullManifest)
	require.NoError(t, err)

	// Create an incremental dump with a gap (invalid chain)
	incPath := filepath.Join(tempDir, "inc.cbor")
	err = os.WriteFile(incPath, []byte("SURINC01..."), 0600)
	require.NoError(t, err)

	incManifest := &surrealdump.Manifest{
		Filename:          "inc.cbor",
		Type:              surrealdump.ManifestTypeIncremental,
		CreatedAt:         time.Now().UTC().Round(time.Second),
		Size:              50,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   400,
		StartVersionstamp: 300, // Gap! Should be 200 to match full dump's EndVersionstamp
	}
	err = surrealdump.WriteManifest(incPath, incManifest)
	require.NoError(t, err)

	// ScanChains will succeed but the incremental dump with gap won't be included
	chains, err := surrealdump.ScanChains(tempDir)
	require.NoError(t, err, "ScanChains should succeed")

	// Verify that the chain only contains the full dump (incremental with gap is excluded)
	require.Len(t, chains, 1, "Should have one chain")
	chain := chains[0]
	assert.NotNil(t, chain.FullDump, "Should have full dump")
	assert.Empty(t, chain.IncrementalDumps, "Incremental dump with gap should not be included")

	// The chain is valid because it only contains the full dump
	// Incremental dumps that would create gaps are simply not included in the chain
}

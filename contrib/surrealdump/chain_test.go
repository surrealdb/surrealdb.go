package surrealdump_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
)

func TestChain(t *testing.T) {
	// Create test manifests
	fullDump := &surrealdump.Manifest{
		Filename:          "dump-001.cbor",
		Type:              surrealdump.ManifestTypeFull,
		CreatedAt:         time.Now(),
		Size:              1000,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   200,
		StartVersionstamp: 0, // Full dumps start from 0
	}

	// Valid incremental that continues from full dump
	validIncremental1 := &surrealdump.Manifest{
		Filename:          "dump-002.cbor",
		Type:              surrealdump.ManifestTypeIncremental,
		CreatedAt:         time.Now(),
		Size:              500,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   300,
		StartVersionstamp: 200, // Matches full dump's max
	}

	// Valid incremental that continues from first incremental
	validIncremental2 := &surrealdump.Manifest{
		Filename:          "dump-003.cbor",
		Type:              surrealdump.ManifestTypeIncremental,
		CreatedAt:         time.Now(),
		Size:              300,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   400,
		StartVersionstamp: 300, // Matches incremental1's max
	}

	// Invalid incremental with gap
	invalidIncremental := &surrealdump.Manifest{
		Filename:          "dump-004.cbor",
		Type:              surrealdump.ManifestTypeIncremental,
		CreatedAt:         time.Now(),
		Size:              200,
		Namespace:         "test",
		Database:          "db",
		EndVersionstamp:   600,
		StartVersionstamp: 500, // Gap - doesn't match any previous max
	}

	t.Run("ValidChain", func(t *testing.T) {
		chain := &surrealdump.Chain{
			FullDump:           fullDump,
			IncrementalDumps:   []*surrealdump.Manifest{validIncremental1, validIncremental2},
			TotalSize:          1800,
			LatestVersionstamp: 400,
		}

		err := chain.Validate()
		assert.NoError(t, err, "Valid chain should pass validation")
	})

	t.Run("ChainWithGap", func(t *testing.T) {
		chain := &surrealdump.Chain{
			FullDump:           fullDump,
			IncrementalDumps:   []*surrealdump.Manifest{invalidIncremental},
			TotalSize:          1200,
			LatestVersionstamp: 600,
		}

		err := chain.Validate()
		assert.Error(t, err, "Chain with gap should fail validation")
		assert.Contains(t, err.Error(), "mismatched start versionstamp")
	})

	t.Run("EmptyChain", func(t *testing.T) {
		chain := &surrealdump.Chain{
			FullDump:         nil,
			IncrementalDumps: []*surrealdump.Manifest{},
		}

		err := chain.Validate()
		assert.Error(t, err, "Empty chain should fail validation")
		assert.Contains(t, err.Error(), "missing full dump")
	})

	t.Run("CanApplyIncremental", func(t *testing.T) {
		// Valid case
		err := surrealdump.CanApplyIncremental(200, validIncremental1)
		assert.NoError(t, err, "Should be able to apply incremental at correct versionstamp")

		// Invalid case - wrong base
		err = surrealdump.CanApplyIncremental(100, validIncremental1)
		assert.Error(t, err, "Should not be able to apply incremental at wrong versionstamp")
		assert.Contains(t, err.Error(), "expects start versionstamp 200, but current is 100")

		// Invalid case - not incremental
		err = surrealdump.CanApplyIncremental(200, fullDump)
		assert.Error(t, err, "Should not be able to apply full dump as incremental")
		assert.Contains(t, err.Error(), "not an incremental dump")
	})

	t.Run("GetPointInTimeOptions", func(t *testing.T) {
		chain := &surrealdump.Chain{
			FullDump:           fullDump,
			IncrementalDumps:   []*surrealdump.Manifest{validIncremental1, validIncremental2},
			TotalSize:          1800,
			LatestVersionstamp: 400,
		}

		options := chain.GetRestorationPoints()
		assert.Equal(t, []uint64{200, 300, 400}, options, "Should return all restoration points")
	})

	t.Run("GetDumpsForVersionstamp", func(t *testing.T) {
		chain := &surrealdump.Chain{
			FullDump:           fullDump,
			IncrementalDumps:   []*surrealdump.Manifest{validIncremental1, validIncremental2},
			TotalSize:          1800,
			LatestVersionstamp: 400,
		}

		// Restore to full dump point
		dumps, err := chain.GetManifestsForVersionstamp(200)
		require.NoError(t, err)
		assert.Len(t, dumps, 1, "Should only need full dump")
		assert.Equal(t, fullDump, dumps[0])

		// Restore to first incremental
		dumps, err = chain.GetManifestsForVersionstamp(300)
		require.NoError(t, err)
		assert.Len(t, dumps, 2, "Should need full dump and first incremental")
		assert.Equal(t, fullDump, dumps[0])
		assert.Equal(t, validIncremental1, dumps[1])

		// Restore to latest
		dumps, err = chain.GetManifestsForVersionstamp(400)
		require.NoError(t, err)
		assert.Len(t, dumps, 3, "Should need all dumps")

		// Restore to point before full dump
		_, err = chain.GetManifestsForVersionstamp(50)
		assert.Error(t, err, "Should fail for versionstamp before full dump")
		assert.Contains(t, err.Error(), "before the full dump")
	})
}

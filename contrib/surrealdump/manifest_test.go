package surrealdump_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
)

func TestManifestReadWrite(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	dumpPath := filepath.Join(tempDir, "test.cbor")

	// Create test manifest
	original := &surrealdump.Manifest{
		Filename:          "test.cbor",
		Type:              surrealdump.ManifestTypeFull,
		CreatedAt:         time.Now().UTC().Round(time.Second),
		Size:              12345,
		Namespace:         "test_ns",
		Database:          "test_db",
		EndVersionstamp:   2000,
		StartVersionstamp: 0, // Full dump starts from 0
		SHA256:            "abcdef123456",
	}

	// Write manifest
	err := surrealdump.WriteManifest(dumpPath, original)
	require.NoError(t, err, "Failed to write manifest")

	// Read manifest back
	read, err := surrealdump.ReadManifest(dumpPath)
	require.NoError(t, err, "Failed to read manifest")

	// Compare
	assert.Equal(t, original, read)
}

func TestReadManifest_FileOperations(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("MissingManifestFile", func(t *testing.T) {
		// Try to read a manifest that doesn't exist
		nonExistentPath := filepath.Join(tempDir, "nonexistent.cbor")
		manifest, err := surrealdump.ReadManifest(nonExistentPath)
		assert.Error(t, err, "Should error when manifest file doesn't exist")
		assert.Nil(t, manifest, "Should return nil manifest on error")
		assert.Contains(t, err.Error(), "manifest not found")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		// Create a manifest with invalid JSON
		invalidPath := filepath.Join(tempDir, "invalid.cbor")
		manifestPath := invalidPath + ".manifest.json"

		// Write invalid JSON to manifest file
		err := os.WriteFile(manifestPath, []byte("not valid json{"), 0600)
		require.NoError(t, err, "Failed to write invalid manifest")

		manifest, err := surrealdump.ReadManifest(invalidPath)
		assert.Error(t, err, "Should error on invalid JSON")
		assert.Nil(t, manifest, "Should return nil manifest on invalid JSON")
		assert.Contains(t, err.Error(), "failed to unmarshal manifest")
	})

	t.Run("EmptyManifestFile", func(t *testing.T) {
		// Create an empty manifest file
		emptyPath := filepath.Join(tempDir, "empty.cbor")
		manifestPath := emptyPath + ".manifest.json"

		err := os.WriteFile(manifestPath, []byte(""), 0600)
		require.NoError(t, err, "Failed to write empty manifest")

		manifest, err := surrealdump.ReadManifest(emptyPath)
		assert.Error(t, err, "Should error on empty manifest file")
		assert.Nil(t, manifest, "Should return nil manifest on empty file")
	})

	t.Run("ValidJSONButFailsValidation", func(t *testing.T) {
		// Create a manifest with valid JSON but that fails validation
		incompletePath := filepath.Join(tempDir, "incomplete.cbor")
		manifestPath := incompletePath + ".manifest.json"

		incompleteManifest := map[string]any{
			"filename": "incomplete.cbor",
			"type":     "full",
			// Missing required fields like namespace, database
		}

		data, err := json.Marshal(incompleteManifest)
		require.NoError(t, err, "Failed to marshal incomplete manifest")

		err = os.WriteFile(manifestPath, data, 0600)
		require.NoError(t, err, "Failed to write incomplete manifest")

		manifest, err := surrealdump.ReadManifest(incompletePath)
		assert.Error(t, err, "Should error when validation fails")
		assert.Nil(t, manifest, "Should return nil manifest when validation fails")
		// The error should come from validation
		assert.Contains(t, err.Error(), "missing namespace")
	})

	t.Run("SuccessfulReadWithCompleteManifest", func(t *testing.T) {
		// Create a valid manifest file
		validPath := filepath.Join(tempDir, "valid.cbor")

		manifest := &surrealdump.Manifest{
			Filename:        "valid.cbor",
			Type:            surrealdump.ManifestTypeFull,
			CreatedAt:       time.Now().UTC().Round(time.Second),
			Size:            5000,
			Namespace:       "test_ns",
			Database:        "test_db",
			EndVersionstamp: 1000,
			SHA256:          "sha256hash",
		}

		err := surrealdump.WriteManifest(validPath, manifest)
		require.NoError(t, err, "Failed to write manifest")

		readManifest, err := surrealdump.ReadManifest(validPath)
		require.NoError(t, err, "Should successfully read valid manifest")
		assert.Equal(t, manifest, readManifest, "Read manifest should match written manifest")
	})
}

func TestManifest_Validate(t *testing.T) {
	t.Run("ValidFullManifest", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:          "full.cbor",
			Type:              surrealdump.ManifestTypeFull,
			CreatedAt:         time.Now().UTC(),
			Size:              5000,
			Namespace:         "test_ns",
			Database:          "test_db",
			StartVersionstamp: 0, // Full dumps should have 0 base
			EndVersionstamp:   1000,
			SHA256:            "sha256hash",
		}

		err := manifest.Validate()
		assert.NoError(t, err, "Valid full manifest should not return error")
	})

	t.Run("ValidIncrementalManifest", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:          "incremental.cbor",
			Type:              surrealdump.ManifestTypeIncremental,
			CreatedAt:         time.Now().UTC(),
			Size:              3000,
			Namespace:         "test_ns",
			Database:          "test_db",
			StartVersionstamp: 1000, // Incremental should have non-zero base
			EndVersionstamp:   2000,
			SHA256:            "sha256hash_inc",
		}

		err := manifest.Validate()
		assert.NoError(t, err, "Valid incremental manifest should not return error")
	})

	t.Run("InvalidManifestType", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:        "invalid.cbor",
			Type:            "invalid_type",
			Namespace:       "test_ns",
			Database:        "test_db",
			EndVersionstamp: 1000,
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error on invalid manifest type")
		assert.Contains(t, err.Error(), "invalid manifest type")
	})

	t.Run("MissingNamespace", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:        "missing_ns.cbor",
			Type:            surrealdump.ManifestTypeFull,
			Namespace:       "", // Missing namespace
			Database:        "test_db",
			EndVersionstamp: 1000,
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error when namespace is missing")
		assert.Contains(t, err.Error(), "missing namespace")
	})

	t.Run("MissingDatabase", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:        "missing_db.cbor",
			Type:            surrealdump.ManifestTypeFull,
			Namespace:       "test_ns",
			Database:        "", // Missing database
			EndVersionstamp: 1000,
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error when database is missing")
		assert.Contains(t, err.Error(), "missing database")
	})

	t.Run("IncrementalWithoutStartVersionstamp", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:          "inc_no_base.cbor",
			Type:              surrealdump.ManifestTypeIncremental,
			Namespace:         "test_ns",
			Database:          "test_db",
			StartVersionstamp: 0, // Missing base for incremental
			EndVersionstamp:   1000,
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error when incremental manifest has no start versionstamp")
		assert.Contains(t, err.Error(), "incremental manifest missing start versionstamp")
	})

	t.Run("IncrementalWithInvalidVersionstampOrder", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:          "inc_bad_order.cbor",
			Type:              surrealdump.ManifestTypeIncremental,
			Namespace:         "test_ns",
			Database:          "test_db",
			StartVersionstamp: 2000,
			EndVersionstamp:   1000, // Max is less than base
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error when EndVersionstamp <= StartVersionstamp for incremental")
		assert.Contains(t, err.Error(), "EndVersionstamp must be greater than StartVersionstamp")
	})

	t.Run("FullManifestWithNonZeroBase", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:          "full_with_base.cbor",
			Type:              surrealdump.ManifestTypeFull,
			Namespace:         "test_ns",
			Database:          "test_db",
			StartVersionstamp: 1000, // Full should have zero base
			EndVersionstamp:   2000,
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error when full manifest has non-zero StartVersionstamp")
		assert.Contains(t, err.Error(), "full manifest should have zero StartVersionstamp")
	})

	t.Run("EmptyManifestType", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:        "empty_type.cbor",
			Type:            "", // Empty type
			Namespace:       "test_ns",
			Database:        "test_db",
			EndVersionstamp: 1000,
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error on empty manifest type")
		assert.Contains(t, err.Error(), "invalid manifest type")
	})

	t.Run("ManifestWithZeroEndVersionstamp", func(t *testing.T) {
		// Zero EndVersionstamp is allowed as it's a valid versionstamp
		manifest := &surrealdump.Manifest{
			Filename:        "zero_max.cbor",
			Type:            surrealdump.ManifestTypeFull,
			Namespace:       "test_ns",
			Database:        "test_db",
			EndVersionstamp: 0, // Zero is valid
		}

		err := manifest.Validate()
		assert.NoError(t, err, "Zero EndVersionstamp should be valid")
	})

	t.Run("ManifestWithEmptySHA256", func(t *testing.T) {
		// Empty SHA256 is allowed as it's optional
		manifest := &surrealdump.Manifest{
			Filename:        "no_hash.cbor",
			Type:            surrealdump.ManifestTypeFull,
			Namespace:       "test_ns",
			Database:        "test_db",
			EndVersionstamp: 1000,
			SHA256:          "", // Empty is ok
		}

		err := manifest.Validate()
		assert.NoError(t, err, "Empty SHA256 should be valid (optional field)")
	})

	t.Run("ManifestWithZeroSize", func(t *testing.T) {
		// Zero size is allowed (though unusual)
		manifest := &surrealdump.Manifest{
			Filename:        "zero_size.cbor",
			Type:            surrealdump.ManifestTypeFull,
			Namespace:       "test_ns",
			Database:        "test_db",
			EndVersionstamp: 1000,
			Size:            0, // Zero size
		}

		err := manifest.Validate()
		assert.NoError(t, err, "Zero size should be valid")
	})

	t.Run("IncrementalWithEqualBaseAndEndVersionstamp", func(t *testing.T) {
		manifest := &surrealdump.Manifest{
			Filename:          "inc_equal.cbor",
			Type:              surrealdump.ManifestTypeIncremental,
			Namespace:         "test_ns",
			Database:          "test_db",
			StartVersionstamp: 1000,
			EndVersionstamp:   1000, // Equal to base
		}

		err := manifest.Validate()
		assert.Error(t, err, "Should error when EndVersionstamp equals StartVersionstamp for incremental")
		assert.Contains(t, err.Error(), "EndVersionstamp must be greater than StartVersionstamp")
	})

	t.Run("CompleteManifestWithAllFields", func(t *testing.T) {
		// Test a manifest with all fields populated
		manifest := &surrealdump.Manifest{
			Filename:          "complete.cbor",
			Type:              surrealdump.ManifestTypeFull,
			CreatedAt:         time.Now().UTC(),
			Size:              123456,
			Namespace:         "production",
			Database:          "main_db",
			EndVersionstamp:   999999,
			StartVersionstamp: 0,
			SHA256:            "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		err := manifest.Validate()
		assert.NoError(t, err, "Complete manifest with all fields should be valid")
	})
}

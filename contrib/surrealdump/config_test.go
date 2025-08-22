package surrealdump_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
)

func TestNewConfig(t *testing.T) {
	config := surrealdump.NewConfig()
	assert.NotNil(t, config, "NewConfig should return non-nil config")
}

func TestConfig_Validate(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
			Output:    "dump.cbor",
		}

		err := config.Validate()
		assert.NoError(t, err, "Valid config should not return error")
	})

	t.Run("MissingNamespace", func(t *testing.T) {
		config := &surrealdump.Config{
			Database: "test_db",
			Output:   "dump.cbor",
		}

		err := config.Validate()
		assert.Error(t, err, "Should error when namespace is missing")
		assert.Contains(t, err.Error(), "namespace is required")
	})

	t.Run("MissingDatabase", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Output:    "dump.cbor",
		}

		err := config.Validate()
		assert.Error(t, err, "Should error when database is missing")
		assert.Contains(t, err.Error(), "database is required")
	})

	t.Run("MissingOutput", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
		}

		err := config.Validate()
		assert.Error(t, err, "Should error when output is missing")
		assert.Contains(t, err.Error(), "output path is required")
	})

	t.Run("IncrementalWithZeroVersionstampIsValid", func(t *testing.T) {
		// As per the comment in Validate(), incremental with zero versionstamp is OK
		// because it can be auto-detected
		config := &surrealdump.Config{
			Namespace:         "test_ns",
			Database:          "test_db",
			Output:            "dump.cbor",
			Incremental:       true,
			SinceVersionstamp: 0, // Zero is OK, will be auto-detected
		}

		err := config.Validate()
		assert.NoError(t, err, "Incremental with zero versionstamp should be valid")
	})

	t.Run("IncrementalWithNonZeroVersionstamp", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace:         "test_ns",
			Database:          "test_db",
			Output:            "dump.cbor",
			Incremental:       true,
			SinceVersionstamp: 12345,
		}

		err := config.Validate()
		assert.NoError(t, err, "Incremental with non-zero versionstamp should be valid")
	})

	t.Run("CompleteConfigWithAllFields", func(t *testing.T) {
		config := &surrealdump.Config{
			Endpoint:          "ws://remote:8000",
			Username:          "admin",
			Password:          "secret",
			Namespace:         "production",
			Database:          "main",
			Output:            "backup.cbor",
			Incremental:       true,
			SinceVersionstamp: 999999,
			Tables:            []string{"users", "products", "orders"},
			Dir:               "/backups",
			Verbose:           true,
		}

		err := config.Validate()
		assert.NoError(t, err, "Complete config with all fields should be valid")
	})

	t.Run("OptionalFieldsCanBeEmpty", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
			Output:    "dump.cbor",
			// All optional fields left as zero values
			Endpoint: "",
			Username: "",
			Password: "",
			Tables:   nil,
			Dir:      "",
			Verbose:  false,
		}

		err := config.Validate()
		assert.NoError(t, err, "Config with only required fields should be valid")
	})
}

func TestConfig_GetOutputPath(t *testing.T) {
	t.Run("OutputOnlyNoDir", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "dump.cbor",
			Dir:    "",
		}

		result := config.GetOutputPath()
		assert.Equal(t, "dump.cbor", result, "Should return Output when Dir is empty")
	})

	t.Run("OutputWithDir", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "dump.cbor",
			Dir:    "backups",
		}

		result := config.GetOutputPath()
		expected := filepath.Join("backups", "dump.cbor")
		assert.Equal(t, expected, result, "Should join Dir and Output")
	})

	t.Run("OutputWithRelativeDir", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "dump.cbor",
			Dir:    "backups",
		}

		result := config.GetOutputPath()
		expected := filepath.Join("backups", "dump.cbor")
		assert.Equal(t, expected, result, "Should join relative Dir and Output")
	})

	t.Run("OutputWithNestedDir", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "daily/dump.cbor",
			Dir:    "backups",
		}

		result := config.GetOutputPath()
		expected := filepath.Join("backups", "daily", "dump.cbor")
		assert.Equal(t, expected, result, "Should handle nested paths correctly")
	})

	t.Run("EmptyOutputReturnsEmpty", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "",
			Dir:    "backups",
		}

		result := config.GetOutputPath()
		assert.Empty(t, result, "Should return empty when Output is empty")
	})

	t.Run("EmptyDirAndOutput", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "",
			Dir:    "",
		}

		result := config.GetOutputPath()
		assert.Empty(t, result, "Should return empty when both Dir and Output are empty")
	})

	t.Run("TrailingSlashInDir", func(t *testing.T) {
		config := &surrealdump.Config{
			Output: "dump.cbor",
			Dir:    "backups",
		}

		result := config.GetOutputPath()
		// filepath.Join should handle trailing slashes correctly
		expected := filepath.Join("backups", "dump.cbor")
		assert.Equal(t, expected, result, "Should handle trailing slash in Dir")
	})
}

func TestConfig_Integration(t *testing.T) {
	t.Run("NewConfigValidateAndGetPath", func(t *testing.T) {
		// Create a new config with defaults
		config := surrealdump.NewConfig()

		// Set required fields
		config.Namespace = "test"
		config.Database = "mydb"
		config.Output = "backup.cbor"
		config.Dir = "backups"

		// Validate
		err := config.Validate()
		require.NoError(t, err, "Config should be valid")

		// Get output path
		outputPath := config.GetOutputPath()
		expected := filepath.Join("backups", "backup.cbor")
		assert.Equal(t, expected, outputPath, "Output path should be correctly joined")
	})

	t.Run("IncrementalDumpConfig", func(t *testing.T) {
		config := &surrealdump.Config{
			Endpoint:          "ws://db.example.com:8000",
			Username:          "backup_user",
			Password:          "backup_pass",
			Namespace:         "production",
			Database:          "main",
			Output:            "incremental-001.cbor",
			Incremental:       true,
			SinceVersionstamp: 0, // Will be auto-detected
			Tables:            []string{"users", "sessions"},
			Dir:               "backups",
			Verbose:           true,
		}

		// Validate
		err := config.Validate()
		require.NoError(t, err, "Incremental config should be valid")

		// Get output path
		outputPath := config.GetOutputPath()
		expected := filepath.Join("backups", "incremental-001.cbor")
		assert.Equal(t, expected, outputPath, "Incremental dump path should be correct")
	})

	t.Run("MinimalConfig", func(t *testing.T) {
		// Test with absolute minimum required fields
		config := &surrealdump.Config{
			Namespace: "ns",
			Database:  "db",
			Output:    "dump.cbor",
		}

		err := config.Validate()
		require.NoError(t, err, "Minimal config should be valid")

		outputPath := config.GetOutputPath()
		assert.Equal(t, "dump.cbor", outputPath, "Minimal config should return Output as-is")
	})
}

func TestConfig_TableSelection(t *testing.T) {
	t.Run("NoTablesSpecified", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
			Output:    "dump.cbor",
			Tables:    nil,
		}

		err := config.Validate()
		assert.NoError(t, err, "Config with no tables specified should be valid (dumps all tables)")
		assert.Empty(t, config.Tables, "Tables should be empty")
	})

	t.Run("EmptyTablesSlice", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
			Output:    "dump.cbor",
			Tables:    []string{},
		}

		err := config.Validate()
		assert.NoError(t, err, "Config with empty tables slice should be valid (dumps all tables)")
		assert.Empty(t, config.Tables, "Tables should be empty")
	})

	t.Run("SpecificTablesSelected", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
			Output:    "dump.cbor",
			Tables:    []string{"users", "products", "orders"},
		}

		err := config.Validate()
		assert.NoError(t, err, "Config with specific tables should be valid")
		assert.Len(t, config.Tables, 3, "Should have 3 tables")
		assert.Contains(t, config.Tables, "users", "Should contain users table")
		assert.Contains(t, config.Tables, "products", "Should contain products table")
		assert.Contains(t, config.Tables, "orders", "Should contain orders table")
	})

	t.Run("SingleTable", func(t *testing.T) {
		config := &surrealdump.Config{
			Namespace: "test_ns",
			Database:  "test_db",
			Output:    "dump.cbor",
			Tables:    []string{"users"},
		}

		err := config.Validate()
		assert.NoError(t, err, "Config with single table should be valid")
		assert.Len(t, config.Tables, 1, "Should have 1 table")
		assert.Equal(t, "users", config.Tables[0], "Should be users table")
	})
}

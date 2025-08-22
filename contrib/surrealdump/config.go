package surrealdump

import (
	"fmt"
	"path/filepath"
)

// Config holds all configuration options for dump operations
type Config struct {
	// SurrealDB server endpoint (e.g., "ws://localhost:8000")
	Endpoint string
	// Authentication username
	Username string
	// Authentication password
	Password string

	// Namespace to dump
	Namespace string
	// Database to dump
	Database string

	// Output file path
	Output string
	// Perform incremental dump instead of full
	Incremental bool
	// Versionstamp to start incremental dump from (for incremental dumps)
	SinceVersionstamp uint64
	// Specific tables to dump
	// If empty, all tables in the specified ns/db will be dumped
	Tables []string

	// Base directory for dumps (prefixes output path)
	Dir string

	// Enable verbose logging
	Verbose bool
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		Endpoint: "ws://localhost:8000",
		Username: "root",
		Password: "root",
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if c.Database == "" {
		return fmt.Errorf("database is required")
	}
	if c.Output == "" {
		return fmt.Errorf("output path is required")
	}
	// Note: c.Incremental with c.SinceVersionstamp == 0 is OK - it can be auto-detected
	return nil
}

// GetOutputPath returns the full output path, applying Dir prefix if set
func (c *Config) GetOutputPath() string {
	if c.Dir != "" && c.Output != "" {
		return filepath.Join(c.Dir, c.Output)
	}
	return c.Output
}

// findLatestVersionstamp searches for the latest versionstamp from previous dumps
// in the search directory for the configured namespace and database.
// It uses c.Dir if set, otherwise the directory of c.Output.
func (c *Config) findLatestVersionstamp() (uint64, error) {
	searchDir := c.Dir
	if searchDir == "" && c.Output != "" {
		searchDir = filepath.Dir(c.GetOutputPath())
	}
	if searchDir == "" {
		searchDir = "."
	}

	return findLatestVersionstamp(searchDir, c.Namespace, c.Database)
}

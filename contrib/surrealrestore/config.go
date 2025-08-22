package surrealrestore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// Configuration validation errors
var (
	// ErrMutuallyExclusive is returned when both -dir and -input flags are provided
	ErrMutuallyExclusive = errors.New("-dir and -input are mutually exclusive. Use one or the other.\n" +
		"  -dir: For point-in-time restoration from a dump chain\n" +
		"  -input: For restoring a single dump file")

	// ErrNoInput is returned when neither -dir nor -input flags are provided
	ErrNoInput = errors.New("either -dir or -input must be provided.\nUsage:\n" +
		"  For single dump: surrealrestore -input dump.cbor\n" +
		"  For dump chain: surrealrestore -dir /path/to/dumps -latest\n" +
		"  For chain info: surrealrestore -dir /path/to/dumps -info")

	// ErrInvalidDirFlags is returned when -latest, -point-in-time, or -info are used with -input
	ErrInvalidDirFlags = errors.New("-latest, -point-in-time, and -info can only be used with -dir")

	// ErrInvalidIncrementalFlag is returned when -incremental is used with -dir
	ErrInvalidIncrementalFlag = errors.New("-incremental can only be used with -input\n" +
		"Note: Incremental restoration from chains is automatic based on the selected point-in-time")
)

// Config holds all configuration for the restore operation
type Config struct {
	// Connection settings

	Endpoint string
	Username string
	Password string

	// Input options (mutually exclusive: Input OR Dir)

	// Input file path for single dump restore
	Input string
	// Directory containing dump chain
	Dir string

	// Operation flags

	// Namespace to which the database belongs
	// Defaults to the namespace in the manifest
	Namespace string
	// Database to restore
	// Defaults to the database in the manifest
	Database string

	// Perform incremental restore (used with Input)
	Incremental bool
	// Restore to specific versionstamp (used with Dir)
	PointInTime uint64
	// Restore to latest available versionstamp (used with Dir)
	Latest bool
	// Show dump chain information only (used with Dir)
	Info bool
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

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate mutually exclusive options
	if c.Dir != "" && c.Input != "" {
		return ErrMutuallyExclusive
	}

	// Validate that at least one is provided
	if c.Dir == "" && c.Input == "" {
		return ErrNoInput
	}

	// Validate flag combinations
	if c.Input != "" {
		// With -input, only -incremental and -verbose are valid
		if c.Latest || c.PointInTime > 0 || c.Info {
			return ErrInvalidDirFlags
		}
	}

	if c.Dir != "" {
		// With -dir, -incremental is not valid
		if c.Incremental {
			return ErrInvalidIncrementalFlag
		}
	}

	return nil
}

// newRestorer creates a new restorer from the configuration
func newRestorer(ctx context.Context, config *Config) (*Restorer, func(), error) {
	// Parse URL
	u, err := url.ParseRequestURI(config.Endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse server endpoint: %w", err)
	}

	// Setup connection with surrealcbor
	conf := connection.NewConfig(u)
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec

	if !config.Verbose {
		conf.Logger = nil
	}

	// Use gws connection
	conn := gws.New(conf)

	// Connect to database
	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
	}

	// Create cleanup function
	cleanup := func() {
		if closeErr := db.Close(ctx); closeErr != nil {
			log.Printf("Warning: failed to close database connection: %v", closeErr)
		}
	}

	// Authenticate
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: config.Username,
		Password: config.Password,
	})
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	// Create restorer
	restorer := New(db)
	restorer.Verbose = config.Verbose
	restorer.Namespace = config.Namespace
	restorer.Database = config.Database

	return restorer, cleanup, nil
}

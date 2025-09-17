package surrealnote

import (
	"fmt"
	"log"
	"os"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/postgres"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/surrealdb"
)

// Config holds application configuration.
// A production system would use structured config with validation (e.g., Viper),
// TLS settings, connection pool configs, and observability endpoints.
type Config struct {
	// Database configuration
	// A production system would include connection pooling settings, timeout configs, and retry policies
	PostgresDSN   string
	SurrealDBURL  string
	SurrealDBNS   string
	SurrealDBDB   string
	SurrealDBUser string
	SurrealDBPass string

	// Mode configuration
	MigrationMode cqrs.MigrationMode
	PostgresOnly  bool
	SurrealOnly   bool
	ReadOnly      bool // When true, all write operations are rejected

	// Server configuration
	// A production system would need TLS config, graceful shutdown timeout, and rate limiting settings
	ServerPort string
}

// App holds the application state.
// A production system would include context for graceful shutdown, metrics registry,
// logger instance, and middleware chain for cross-cutting concerns.
type App struct {
	store    store.Store
	config   *Config
	readOnly bool // Runtime read-only state (can be toggled)
	// A production system would have logger, metrics, tracer, and circuit breaker instances
}

// New creates a new application instance.
// A production system would accept a context parameter, implement proper health checks,
// initialize monitoring/tracing, and validate all connections before returning.
func New(config *Config) (*App, error) {
	// Initialize stores
	var appStore store.Store
	var err error

	if config.SurrealOnly {
		// Use only SurrealDB with surrealcbor
		appStore, err = surrealdb.NewSurrealStoreCBOR(
			config.SurrealDBURL,
			config.SurrealDBNS,
			config.SurrealDBDB,
			config.SurrealDBUser,
			config.SurrealDBPass,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
		}
		log.Println("Connected to SurrealDB with surrealcbor")
	} else if config.PostgresOnly {
		// Use only PostgreSQL
		appStore, err = postgres.NewPostgresStore(config.PostgresDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		log.Println("Connected to PostgreSQL")
	} else {
		// Use CQRS with both stores
		pgStore, err := postgres.NewPostgresStore(config.PostgresDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		log.Println("Connected to PostgreSQL")

		sdbStore, err := surrealdb.NewSurrealStoreCBOR(
			config.SurrealDBURL,
			config.SurrealDBNS,
			config.SurrealDBDB,
			config.SurrealDBUser,
			config.SurrealDBPass,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
		}
		log.Println("Connected to SurrealDB with surrealcbor")

		appStore = cqrs.NewCQRSStore(pgStore, sdbStore, config.MigrationMode)
		log.Printf("Using CQRS store in %s mode", config.MigrationMode)
	}

	app := &App{
		store:    nil, // Will be set below
		config:   config,
		readOnly: config.ReadOnly, // Initialize from config
	}

	// Wrap the store with read-only protection
	app.store = store.NewReadOnlyStore(appStore, app.IsReadOnly)

	return app, nil
}

// Close closes the application and its resources
func (a *App) Close() error {
	if a.store != nil {
		return a.store.Close()
	}
	return nil
}

// Store returns the underlying store (useful for testing)
func (a *App) Store() store.Store {
	return a.store
}

// SetReadOnly sets the application's read-only mode for maintenance or migration operations.
// When enabled, all write operations (Create, Update, Delete) will be rejected with errors,
// while read operations continue to function normally. This is essential for safe database
// migrations and maintenance windows.
//
// Read-only mode is implemented at the store wrapper level, meaning it affects all
// data operations regardless of which underlying store implementation is active.
// The mode change is logged for operational visibility.
//
// Common use cases:
//   - Pre-migration safety: Prevent writes before starting database migration
//   - Maintenance windows: Allow safe database maintenance without data corruption
//   - Emergency response: Quick way to stop writes during incident investigation
//   - Migration validation: Ensure no new data during consistency checks
//   - Blue-green deployments: Coordinate read-only periods during traffic switching
//
// The read-only state is checked by the ReadOnlyStore wrapper on every write operation,
// providing immediate enforcement without requiring application restarts.
func (a *App) SetReadOnly(readOnly bool) {
	a.readOnly = readOnly
	log.Printf("Application read-only mode: %v", readOnly)
}

// IsReadOnly returns whether the application is currently in read-only mode.
// This method is used by the ReadOnlyStore wrapper to determine whether
// write operations should be permitted or rejected.
//
// The read-only state can be changed at runtime using SetReadOnly() and is
// initially set from the Config.ReadOnly value when the application starts.
//
// This method is called frequently during normal operation as it's checked
// on every write operation, so it should remain lightweight and fast.
func (a *App) IsReadOnly() bool {
	return a.readOnly
}

// getEnv retrieves an environment variable value with a fallback default value.
// This helper function simplifies environment variable handling by providing
// a consistent pattern for configuration with sensible defaults.
//
// Parameters:
//   - key: Environment variable name to look up
//   - defaultValue: Value to return if environment variable is unset or empty
//
// Returns:
//   - Environment variable value if set and non-empty
//   - Default value if environment variable is unset or empty string
//
// This function follows the common pattern of treating empty environment
// variables the same as unset variables, which is useful for container
// environments where empty values may be accidentally set.
//
// Usage example:
//
//	port := getEnv("PORT", "8080")
//	dbURL := getEnv("DATABASE_URL", "postgres://localhost/mydb")
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

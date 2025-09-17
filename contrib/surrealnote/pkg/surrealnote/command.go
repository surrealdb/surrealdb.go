package surrealnote

// Command represents a discrete application operation with its specific configuration.
//
// The Command interface enables a clean separation between command parsing, validation,
// and execution. Each command implementation encapsulates the parameters and options
// needed for its specific operation, while the application layer handles the routing
// and execution through the [App] struct.
//
// Command implementations provide their name for routing purposes and carry all
// necessary configuration as struct fields. This design supports type-safe command
// handling and makes it easy to add new operations without modifying existing code.
//
// Current command implementations:
//   - [MigrateCommand]: Database schema migration operations
//   - [RunCommand]: HTTP server startup and operation
//   - [SyncCommand]: Data synchronization between database backends
//
// Commands are typically created by parsing command-line arguments through the
// application's Parse function and then executed by calling the appropriate
// method on [App] (App.Migrate, App.Run, App.Sync).
type Command interface {
	// Name returns the command identifier used for routing to the appropriate handler.
	//
	// This method enables the application to dispatch command execution to the
	// correct handler method on [App]. The returned name must match the CLI
	// sub-command name for proper routing.
	//
	// Implementation examples:
	//   - MigrateCommand.Name() returns "migrate"
	//   - RunCommand.Name() returns "run"
	//   - SyncCommand.Name() returns "sync"
	Name() string
}

// MigrateCommand represents the database schema migration operation.
//
// MigrateCommand initializes or updates database schemas to match the application's
// current data model definition. This is distinct from data migration - it only
// handles structural changes like creating tables, adding columns, or updating
// indexes without moving data between different database systems.
//
// The migration behavior adapts to the current store configuration:
//   - Single store mode: Migrates only the active database (PostgreSQL or SurrealDB)
//   - CQRS mode: Migrates both primary and secondary stores to ensure schema consistency
//     across both backends during the migration process
//
// Store-specific migration behavior:
//   - [store/postgres.PostgresStore]: Uses GORM's AutoMigrate for DDL operations
//   - [store/surrealdb.SurrealStoreCBOR]: Minimal setup due to SurrealDB's schema flexibility
//   - [store/cqrs.CQRSStore]: Coordinates migration across both underlying stores
//
// # Usage Scenarios
//
// This command should be executed in these situations:
//   - Initial deployment: Create database schema before first application startup
//   - Development: Apply model changes during iterative development
//   - Production deployment: Ensure schema compatibility before serving traffic
//   - CQRS migration: Synchronize schema between primary and secondary stores
//
// # Safety and Idempotency
//
// The migrate command is safe to run multiple times without data loss. It only
// creates missing schema elements and updates existing ones when necessary.
// Data integrity is preserved during structural changes.
//
// # Future Extensions
//
// Currently implemented as an empty struct since all migrations automatically
// advance to the latest schema version. Future enhancements could include:
//   - TargetVersion: migrate to a specific schema version instead of latest
//   - Direction: support rollback migrations for schema downgrades
//   - DryRun: preview schema changes without applying them
//   - Verbose: detailed logging of migration operations and timing
//
// Example usage:
//
//	surrealnote migrate              # Apply all pending schema changes
type MigrateCommand struct {
	// Currently empty - all configuration comes from App.Config
	// Future migration-specific options will be added here as needed
}

// Name returns the command name for identification and routing purposes.
// This method is required by the Command interface and helps the application
// dispatcher route command execution to the appropriate handler function.
func (c *MigrateCommand) Name() string {
	return "migrate"
}

// RunCommand represents the HTTP server startup and operation.
//
// RunCommand launches the web application server that provides the complete REST API
// for the note-taking application. This includes all CRUD operations for workspaces,
// pages, blocks, users, comments, and permissions, along with authentication and
// administrative endpoints.
//
// # Server Features
//
// The HTTP server provides these demonstration features (NOT production-ready):
//   - REST API endpoints with JSON request/response handling for all [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models] entities
//   - Basic user and permission stubs without real authentication or enforcement
//   - Health check endpoints for monitoring
//   - Administrative endpoints for runtime migration mode changes (unsecured)
//   - Graceful shutdown support with configurable timeout periods
//   - Multi-store support enabling transparent backend switching
//
// # Store Configuration Support
//
// The server adapts its behavior based on the current store configuration:
//   - Single store mode: All operations use one database ([store/postgres] or [store/surrealdb])
//   - CQRS mode: Coordinates between primary and secondary stores ([store/cqrs])
//   - Read-only mode: Prevents write operations during maintenance or migration
//
// Migration mode controls read/write behavior:
//   - [store/cqrs.ModeSingle]: Use only primary store
//   - [store/cqrs.ModeDualWrite]: Write to both stores, read from primary
//   - [store/cqrs.ModeValidation]: Write to both, read from both and compare
//   - [store/cqrs.ModeSwitching]: Write to both, read from secondary
//
// # Lifecycle Management
//
// The server operates continuously until one of these conditions:
//   - Context cancellation (enables graceful shutdown)
//   - Fatal server errors (port binding failures, etc.)
//   - Manual termination signals (SIGTERM, SIGINT)
//
// During shutdown, the server completes in-flight requests before closing
// database connections and releasing resources.
//
// # Configuration
//
// Server behavior is controlled through the application [Config], including:
//   - Server port binding and host address
//   - Database connection settings for one or both stores
//   - Migration mode for CQRS operations
//   - Read-only mode for maintenance phases
//   - Authentication settings and session timeouts
//
// # Usage Scenarios
//
// This command serves different purposes throughout the application lifecycle:
//   - Development: Local development server with hot-reload capabilities
//   - Production: Production traffic serving with full feature availability
//   - Migration: Dual-write mode during database backend transitions
//   - Testing: API endpoint testing and integration validation
//   - Maintenance: Read-only mode during maintenance windows
//
// # Future Extensions
//
// Currently implemented as an empty struct since all configuration comes from
// the application Config. Future command-specific enhancements could include:
//   - Port/Host: Runtime override of binding configuration
//   - TLS: Certificate-based HTTPS configuration for secure deployments
//   - LogLevel: Per-session logging verbosity control
//   - MaxConnections: Connection limits for resource management
//   - Profiling: Runtime profiling endpoint enablement
//
// Example usage:
//
//	./bin/surrealnote run                       # Start with default configuration
//	./bin/surrealnote -mode read_only run       # Start in read-only mode
//	./bin/surrealnote -postgres-only run        # Use only PostgreSQL
//	./bin/surrealnote -surreal-only run         # Use only SurrealDB
type RunCommand struct {
	// Currently empty - all configuration comes from App.Config
	// Future run-specific options can be added here as needed
}

// Name returns the command name for identification and routing purposes.
// This method is required by the Command interface and helps the application
// dispatcher route command execution to the appropriate handler function.
// For RunCommand, this always returns "run" to match CLI argument parsing.
func (c *RunCommand) Name() string {
	return "run"
}

// SyncCommand represents data synchronization between database backends.
//
// SyncCommand performs catch-up synchronization to ensure data consistency between
// PostgreSQL and SurrealDB during migration phases when dual-write operations may
// have failed on one of the stores. This is essential for maintaining data integrity
// in CQRS migration scenarios.
//
// # Synchronization Strategy
//
// SyncCommand implements timestamp-based change detection rather than event streaming
// or distributed transactions. This approach uses existing CreatedAt and UpdatedAt
// fields in [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models] to identify records that need synchronization between stores.
//
// The synchronization process:
//  1. Query the source store for records modified within the specified time range
//  2. For each modified record, check if it exists in the destination store
//  3. Create missing records or update existing ones with the latest data
//  4. Log warnings for individual failures while continuing the overall process
//
// # Synchronization Directions
//
// Two synchronization directions support different migration scenarios:
//
//   - Forward sync (PostgreSQL → SurrealDB): Primary use case during migration phases.
//     Catches up SurrealDB with changes that may have failed during dual-write operations.
//     Typically used before switching read traffic to SurrealDB.
//
//   - Reverse sync (SurrealDB → PostgreSQL): Used for rollback scenarios when reverting
//     from SurrealDB back to PostgreSQL, or when SurrealDB becomes the primary store.
//
// # Entity Synchronization Scope
//
// SyncCommand processes all entity types defined in [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models]:
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Workspace]: Top-level organizational containers
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.User]: User accounts and profile information
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Page]: Documents and their hierarchical relationships
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Block]: Content elements within pages
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Comment]: Collaborative annotations on blocks
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Permission]: Access control rules for resources
//
// Synchronization respects entity relationships and handles foreign key dependencies
// appropriately. Missing parent entities are synchronized before their children.
//
// # Time Window Management
//
// Time-based filtering ensures efficiency by processing only relevant changes:
//   - Since: Inclusive start time for the synchronization window
//   - Until: Exclusive end time for the synchronization window
//   - Records are included if MAX(CreatedAt, UpdatedAt) falls within the range
//   - Default window is 24 hours when Since is not specified
//   - Default Until is current time when not specified
//
// # Usage Scenarios
//
// SyncCommand addresses these operational needs:
//   - Post-migration validation: Ensure consistency after switching from dual-write mode
//   - Network partition recovery: Catch up after temporary connectivity issues
//   - Regular consistency checks: Validate data integrity during long migrations
//   - Rollback preparation: Synchronize data before reverting to previous database
//   - Development testing: Verify migration logic with controlled time windows
//
// # Prerequisites and Constraints
//
// SyncCommand operation requires:
//   - Both PostgreSQL and SurrealDB must be accessible and configured
//   - Cannot run in single-store mode (requires CQRS configuration)
//   - Write access to destination store (read-only mode must be disabled)
//   - Sufficient network bandwidth for data transfer between stores
//   - Compatible schema versions in both stores
//
// # Performance Characteristics
//
//   - Time complexity: O(n) where n is records modified in the time range
//   - Space complexity: O(1) constant memory (processes records individually)
//   - Network overhead: Proportional to data differences between stores
//   - Failure handling: Individual record failures don't stop the overall process
//
// # Error Handling and Resilience
//
// SyncCommand is designed for robustness:
//   - Individual record sync failures are logged but don't halt the process
//   - Connection issues to one store don't affect discovery of changes
//   - Can be run multiple times safely (idempotent operation)
//   - Supports progressive sync with multiple passes using smaller time windows
//
// Example usage:
//
//	# Forward sync with specific time range
//	surrealnote sync --direction forward --since 2023-12-01T00:00:00Z --until 2023-12-02T00:00:00Z
//
//	# Reverse sync from last 24 hours
//	surrealnote sync --direction reverse --since 2023-12-01T12:00:00Z
//
//	# Default forward sync (last 24 hours)
//	surrealnote sync
type SyncCommand struct {
	// Direction specifies the data flow direction for synchronization.
	//
	// Valid values:
	//   - "forward": PostgreSQL → SurrealDB (default)
	//     Primary use case for migration phases. Synchronizes changes from the
	//     PostgreSQL primary store to the SurrealDB secondary store.
	//   - "reverse": SurrealDB → PostgreSQL
	//     Used for rollback scenarios or when SurrealDB has been promoted to
	//     primary status during migration.
	//
	// The direction determines which store serves as the source of truth for
	// conflict resolution during synchronization.
	Direction string

	// Since specifies the inclusive start time for the synchronization window.
	//
	// Only records with CreatedAt or UpdatedAt timestamps after this time will
	// be considered for synchronization. Must be provided in RFC3339 format.
	//
	// If empty, defaults to 24 hours ago to provide a reasonable catch-up window
	// for most operational scenarios.
	//
	// Examples:
	//   - "2023-12-01T00:00:00Z" (UTC timezone)
	//   - "2023-12-01T15:30:45+02:00" (with timezone offset)
	//   - "2023-12-01T10:30:45-05:00" (EST timezone)
	Since string

	// Until specifies the exclusive end time for the synchronization window.
	//
	// Only records with CreatedAt or UpdatedAt timestamps before this time will
	// be considered for synchronization. Must be provided in RFC3339 format.
	//
	// If empty, defaults to the current time to include all changes up to when
	// the sync operation starts. This bound helps ensure consistent snapshots
	// and prevents syncing incomplete transactions.
	//
	// Examples:
	//   - "2023-12-02T00:00:00Z" (end of day boundary)
	//   - "2023-12-01T23:59:59+00:00" (explicit end of day)
	//   - Leave empty to sync until current time
	Until string
}

// Name returns the command name for identification and routing purposes.
// This method is required by the Command interface and helps the application
// dispatcher route command execution to the appropriate handler function.
// For SyncCommand, this always returns "sync" to match CLI argument parsing.
func (c *SyncCommand) Name() string {
	return "sync"
}

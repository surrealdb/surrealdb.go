package surrealnote

import (
	"context"
	"fmt"
	"time"
)

// Main is the main entry point for the surrealnote application.
// It takes a context for cancellation and command line arguments, then executes the appropriate command.
// This function can be called directly from tests without needing to build the binary.
// The context is particularly useful for graceful shutdown in tests and production.
// Returns an error if any step fails (parsing, app creation, or command execution).
//
// # Command Line Usage
//
// The application supports multiple commands and flags for different operation modes:
//
//	# Run with PostgreSQL only
//	surrealnote -postgres-only
//
//	# Run with SurrealDB only
//	surrealnote -surreal-only
//
//	# Run with dual-write mode (default)
//	surrealnote -mode dual_write
//
//	# Run with validation mode (compare reads from both databases)
//	surrealnote -mode validation
//
//	# Run with switching mode (read from SurrealDB, write to both)
//	surrealnote -mode switching
//
// # Environment Variables
//
// The application reads configuration from these environment variables:
//
//	POSTGRES_DSN     - PostgreSQL connection string (default: constructed from individual settings)
//	SURREALDB_URL    - SurrealDB WebSocket URL (default: ws://localhost:8000/rpc)
//	SURREALDB_NS     - SurrealDB namespace (default: surrealnote)
//	SURREALDB_DB     - SurrealDB database (default: surrealnote)
//	SURREALDB_USER   - SurrealDB username (default: root)
//	SURREALDB_PASS   - SurrealDB password (default: root)
//
// # Migration Strategy
//
// The application supports a phased migration approach from PostgreSQL to SurrealDB:
//
//  1. Start with PostgreSQL (mode: single)
//     Application runs with PostgreSQL as the only database
//
//  2. Enable Dual-Write (mode: dual_write)
//     Writes go to both databases, reads from PostgreSQL
//     SurrealDB gets populated with data
//
//  3. Validation Phase (mode: validation)
//     Writes to both databases, reads from both and compares results
//     Monitor for discrepancies between databases
//
//  4. Switch to SurrealDB (mode: switching)
//     Writes to both databases, reads from SurrealDB
//     PostgreSQL acts as backup for rollback capability
//
//  5. Complete Migration (-surreal-only flag)
//     Use only SurrealDB, PostgreSQL can be decommissioned
func Main(ctx context.Context, args []string) error {
	// Parse command line arguments to get command and configuration
	cmd, config, err := Parse(args)
	if err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Create application with the configuration
	app, err := New(config)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	defer app.Close()

	// Execute the command based on its type
	switch c := cmd.(type) {
	case *MigrateCommand:
		if err := app.Migrate(ctx, c); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	case *RunCommand:
		if err := app.Run(ctx, c); err != nil {
			return fmt.Errorf("server failed: %w", err)
		}
	case *SyncCommand:
		// Parse time bounds
		since, err := ParseTime(c.Since, time.Now().Add(-24*time.Hour))
		if err != nil {
			return fmt.Errorf("invalid since time: %w", err)
		}
		until, err := ParseTime(c.Until, time.Now())
		if err != nil {
			return fmt.Errorf("invalid until time: %w", err)
		}

		// Perform sync
		if err := app.Sync(ctx, c.Direction, since, until); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown command type: %T", cmd)
	}

	return nil
}

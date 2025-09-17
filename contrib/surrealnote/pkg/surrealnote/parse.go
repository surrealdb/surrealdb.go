package surrealnote

import (
	"flag"
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs"
)

// Parse parses command line arguments and returns the command to execute,
// the application configuration, and any error that occurred.
// The first return value is the Command (either MigrateCommand or RunCommand)
// which contains command-specific options.
// The second return value is the Config which contains database and server configuration
// shared across all commands.
// The third return value is an error if parsing failed.
func Parse(args []string) (Command, *Config, error) {
	// Use flag package for parsing
	flagSet := flag.NewFlagSet("surrealnote", flag.ContinueOnError)

	var (
		migrate      = flagSet.Bool("migrate", false, "Run database migrations")
		sync         = flagSet.Bool("sync", false, "Run catch-up synchronization")
		syncDir      = flagSet.String("sync-direction", "forward", "Sync direction: forward (PG->SDB) or reverse (SDB->PG)")
		syncSince    = flagSet.String("sync-since", "", "Sync changes since this time (RFC3339)")
		syncUntil    = flagSet.String("sync-until", "", "Sync changes until this time (RFC3339)")
		mode         = flagSet.String("mode", "single", "Migration mode: single, read_only, switching, reversed")
		port         = flagSet.String("port", "8080", "Server port")
		postgresPort = flagSet.String("postgres-port", "5432", "PostgreSQL port")
		postgresOnly = flagSet.Bool("postgres-only", false, "Use only PostgreSQL")
		surrealOnly  = flagSet.Bool("surreal-only", false, "Use only SurrealDB")
		readOnly     = flagSet.Bool("read-only", false, "Enable read-only mode (required for sync operations)")
	)

	// Parse the arguments
	if err := flagSet.Parse(args); err != nil {
		return nil, nil, err
	}

	// Check for subcommands (e.g., "surrealnote migrate" or "surrealnote sync")
	remainingArgs := flagSet.Args()
	if len(remainingArgs) == 0 {
		return nil, nil, fmt.Errorf(`subcommand required

Usage: surrealnote [flags] <command>

Commands:
  run       Start the SurrealNote server
  migrate   Run database migrations
  sync      Perform catch-up synchronization between databases

Examples:
  # Normal operation
  surrealnote run                                    # Default: PostgreSQL only
  surrealnote -postgres-only run                     # Explicitly PostgreSQL only
  surrealnote -surreal-only run                      # SurrealDB only

  # Migration scenarios (matching E2E test stages)
  surrealnote -mode single run                       # Stage 3: CQRS with PostgreSQL primary
  surrealnote -mode read_only run                    # Stage 4: Read-only during validation
  surrealnote -mode switching run                    # Stage 5: Reads from SurrealDB
  surrealnote -mode reversed run                     # Stage 6: Writes to SurrealDB

  # Database migration and sync
  surrealnote migrate                                # Run schema migrations
  surrealnote sync                                   # Forward sync (PG->SDB) last 24h
  surrealnote sync -sync-direction forward -sync-since 2024-01-01T00:00:00Z
  surrealnote sync -sync-direction reverse           # Reverse sync (SDB->PG)

  # Custom ports
  surrealnote -postgres-port=5438 run
  surrealnote -port=8090 run`)
	}

	// Parse the subcommand
	var cmd Command
	config := &Config{
		ServerPort: *port,
		ReadOnly:   *readOnly,
	}

	switch remainingArgs[0] {
	case "run":
		cmd = &RunCommand{}
	case "migrate":
		*migrate = true
		cmd = &MigrateCommand{}
	case "sync":
		*sync = true
		// Validate sync direction
		if *syncDir != "forward" && *syncDir != "reverse" {
			return nil, nil, fmt.Errorf("invalid sync direction: %s (must be 'forward' or 'reverse')", *syncDir)
		}
		cmd = &SyncCommand{
			Direction: *syncDir,
			Since:     *syncSince,
			Until:     *syncUntil,
		}
	default:
		return nil, nil, fmt.Errorf("unknown command: %s\n\nValid commands: run, migrate, sync", remainingArgs[0])
	}

	// Parse migration mode
	switch *mode {
	case "single":
		config.MigrationMode = cqrs.ModeSingle
	case "read_only":
		config.MigrationMode = cqrs.ModeReadOnly
	case "switching":
		config.MigrationMode = cqrs.ModeSwitching
	case "reversed":
		config.MigrationMode = cqrs.ModeReversed
	default:
		return nil, nil, fmt.Errorf("invalid migration mode: %s", *mode)
	}

	// Override mode based on flags
	if *postgresOnly {
		config.PostgresOnly = true
		config.SurrealOnly = false
		config.MigrationMode = cqrs.ModeSingle
	}
	if *surrealOnly {
		config.SurrealOnly = true
		config.PostgresOnly = false
	}

	// Load configuration from environment
	defaultPgDSN := fmt.Sprintf("postgres://surrealnote:surrealnote123@localhost:%s/surrealnote?sslmode=disable", *postgresPort)
	config.PostgresDSN = getEnv("POSTGRES_DSN", defaultPgDSN)
	config.SurrealDBURL = getEnv("SURREALDB_URL", "ws://localhost:8000/rpc")
	config.SurrealDBNS = getEnv("SURREALDB_NS", "surrealnote")
	config.SurrealDBDB = getEnv("SURREALDB_DB", "surrealnote")
	config.SurrealDBUser = getEnv("SURREALDB_USER", "root")
	config.SurrealDBPass = getEnv("SURREALDB_PASS", "root")

	return cmd, config, nil
}

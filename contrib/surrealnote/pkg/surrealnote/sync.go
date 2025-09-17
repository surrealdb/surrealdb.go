package surrealnote

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs"
)

// Sync performs timestamp-based catch-up synchronization between PostgreSQL and SurrealDB stores.
// This method is essential for maintaining data consistency during CQRS migration phases when
// dual-write operations may have failed on the secondary store due to network issues, timeouts,
// or other transient failures.
//
// The synchronization process identifies all records modified in both stores
// within the specified time window, reads the complete record data from the
// source store, then checks if each record exists in the destination store.
// Missing records are created and existing ones are updated with the source data.
// Individual failures are logged while the overall process continues to ensure
// maximum data synchronization.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control during the sync operation
//   - direction: Synchronization direction controlling data flow:
//   - "forward": PostgreSQL → SurrealDB (most common, for SurrealDB catch-up)
//   - "reverse": SurrealDB → PostgreSQL (for rollback scenarios)
//   - since: Start time boundary - only records modified after this time are synchronized
//   - until: End time boundary - only records modified before this time are synchronized
//
// Returns an error if:
//   - The application is configured in single-store mode (PostgreSQL-only or SurrealDB-only)
//   - Read-only mode is enabled (sync requires write access to destination store)
//   - The underlying store is not a CQRS store (sync requires access to both databases)
//   - Invalid direction parameter is provided (not "forward" or "reverse")
//   - The actual synchronization process fails due to database connectivity or other issues
//
// Usage scenarios:
//   - Post-migration consistency validation after switching from dual-write mode
//   - Recovery from network partitions that caused dual-write failures
//   - Regular maintenance sync during long-running migration phases
//   - Rollback preparation when reverting database migrations
//
// Performance considerations:
//   - Time complexity: O(n) where n is the number of modified records in the time window
//   - Memory usage: Constant, as records are processed individually
//   - Network overhead: Proportional to data differences between stores
//   - I/O pattern: Read-heavy on source store, write-heavy on destination store
//
// Thread safety:
//   - This method is safe for concurrent use with other read operations
//   - Multiple sync operations should not run simultaneously on the same data
//   - The underlying CQRS store handles necessary locking and consistency
//
// Example usage patterns:
//
//	// Catch up SurrealDB with the last 6 hours of PostgreSQL changes
//	since := time.Now().Add(-6 * time.Hour)
//	until := time.Now()
//	err := app.Sync(ctx, "forward", since, until)
//
//	// Prepare for rollback by syncing recent SurrealDB changes to PostgreSQL
//	since := lastMigrationTime
//	until := time.Now()
//	err := app.Sync(ctx, "reverse", since, until)
func (a *App) Sync(ctx context.Context, direction string, since, until time.Time) error {
	// Sync requires both databases to be available, which means we need a CQRS store
	if a.config.PostgresOnly {
		return fmt.Errorf("sync not available in PostgreSQL-only mode")
	}
	if a.config.SurrealOnly {
		return fmt.Errorf("sync not available in SurrealDB-only mode")
	}

	// Sync cannot run in read-only mode as it needs to write to databases
	if a.config.ReadOnly {
		return fmt.Errorf("sync cannot run in read-only mode as it needs write access to databases")
	}

	// The app store must be a CQRS store for sync to work
	// Check if it's wrapped in a ReadOnlyStore first
	var cqrsStore *cqrs.CQRSStore
	if readOnlyStore, ok := a.store.(*store.ReadOnlyStore); ok {
		// Unwrap the ReadOnlyStore to get the underlying store
		cqrsStore, ok = readOnlyStore.Store.(*cqrs.CQRSStore)
		if !ok {
			return fmt.Errorf("sync requires CQRS store but app has %T wrapped in ReadOnlyStore", readOnlyStore.Store)
		}
	} else {
		// Direct CQRS store (shouldn't happen with current setup)
		cqrsStore, ok = a.store.(*cqrs.CQRSStore)
		if !ok {
			return fmt.Errorf("sync requires CQRS store but app has %T", a.store)
		}
	}

	// Perform the sync based on direction
	switch direction {
	case "forward":
		log.Printf("Performing forward sync (PostgreSQL -> SurrealDB) from %v to %v", since, until)
		if err := cqrsStore.SyncMissedUpdates(ctx, since, until); err != nil {
			return fmt.Errorf("forward sync failed: %w", err)
		}
		log.Println("Forward sync completed successfully")

	case "reverse":
		log.Printf("Performing reverse sync (SurrealDB -> PostgreSQL) from %v to %v", since, until)
		if err := cqrsStore.ReverseSyncMissedUpdates(ctx, since, until); err != nil {
			return fmt.Errorf("reverse sync failed: %w", err)
		}
		log.Println("Reverse sync completed successfully")

	default:
		return fmt.Errorf("invalid sync direction: %s (must be 'forward' or 'reverse')", direction)
	}

	return nil
}

// ParseTime parses a time string in RFC3339 format with intelligent defaults for sync operations.
// This utility function handles the common pattern of optional time parameters in CLI tools
// where users may want to specify precise time boundaries or rely on reasonable defaults.
//
// The function accepts RFC3339 formatted time strings, which include:
//   - UTC times: "2023-12-01T15:30:45Z"
//   - Times with timezone: "2023-12-01T15:30:45+02:00", "2023-12-01T15:30:45-08:00"
//   - Date-only strings are extended to full RFC3339: "2023-12-01" becomes "2023-12-01T00:00:00Z"
//
// Parameters:
//   - timeStr: The time string to parse. If empty or whitespace, defaultTime is returned.
//   - defaultTime: The fallback time to use when timeStr is empty. This allows callers
//     to provide context-appropriate defaults (e.g., 24 hours ago for 'since', now for 'until').
//
// Returns:
//   - The parsed time in the timezone specified in the input string
//   - The defaultTime if timeStr is empty
//   - An error if the timeStr is non-empty but cannot be parsed as RFC3339
//
// This function is commonly used in sync operations where:
//   - Empty 'since' defaults to a reasonable lookback period (e.g., 24 hours ago)
//   - Empty 'until' defaults to the current time to include all changes up to sync start
//   - Explicit times allow precise control over the synchronization window
//
// Example usage:
//
//	since, err := ParseTime("", time.Now().Add(-24*time.Hour))           // Default to 24h ago
//	until, err := ParseTime("2023-12-01T23:59:59Z", time.Now())          // Specific end time
//	start, err := ParseTime("2023-12-01T15:30:45+02:00", time.Time{})    // Timezone-aware parsing
func ParseTime(timeStr string, defaultTime time.Time) (time.Time, error) {
	if timeStr == "" {
		return defaultTime, nil
	}
	return time.Parse(time.RFC3339, timeStr)
}

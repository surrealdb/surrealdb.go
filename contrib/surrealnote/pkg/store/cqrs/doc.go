// Package cqrs provides CQRS (Command Query Responsibility Segregation) implementation for zero-downtime database migration.
//
// This package enables seamless migration between PostgreSQL and SurrealDB by coordinating
// dual writes and implementing eventual consistency through timestamp-based synchronization.
// It demonstrates how to build resilient systems that can change database backends without
// downtime or data loss.
//
// # Migration Strategy and Modes
//
// The application supports three migration modes for zero-downtime database transitions:
//
//  1. Single Mode ([ModeSingle]): Use only the primary store (PostgreSQL or SurrealDB).
//     This is the default operational mode before migration starts and after migration completes.
//
//  2. Read-Only Mode ([ModeReadOnly]): Reject all write operations while performing final
//     synchronization. This mode ensures data consistency during the critical switchover
//     phase by temporarily preventing modifications.
//
//  3. Switching Mode ([ModeSwitching]): Read from the secondary store while keeping primary
//     active. This mode validates that the secondary store is ready to become the new primary
//     by serving live read traffic while maintaining the ability to rollback.
//
// The migration progresses through these modes with background synchronization:
//  1. Run continuous background sync while in ModeSingle
//  2. Switch to ModeReadOnly for final catch-up synchronization
//  3. Switch to ModeSwitching to validate secondary store with live traffic
//  4. Swap stores and return to ModeSingle with secondary as new primary
//
// This approach eliminates dual-writing complexity while ensuring zero-downtime migration
// through brief read-only periods during switchover.
//
// # CQRS Migration Architecture
//
// [CQRSStore] coordinates between two [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] implementations without dual-writing.
// Background synchronization runs continuously while the application serves traffic from
// the primary store. Brief read-only periods during switchover ensure consistency while
// minimizing downtime to seconds rather than hours.
//
// # Consistency Strategies
//
// The implementation supports two synchronization strategies:
//
// Timestamp-Based Synchronization:
// Uses CreatedAt/UpdatedAt fields to identify changes within time windows.
// Simple to implement with existing models but may include unchanged records
// and cannot detect deletes without scanning all records.
//
// Change Tracking Table:
// Records all modifications in a dedicated table within the same transaction.
// Provides precise change capture including creates, updates, and deletes.
// Enables exact replay of changes without scanning entire tables.
//
// # Design Decision: Eliminating Dual-Writing
//
// Traditional dual-write approaches were rejected due to fundamental issues:
//
// Dual-Write Problems:
//   - Partial failures leave stores inconsistent
//   - Performance degradation from double writes
//   - Complex error handling and retry logic
//   - No guarantee of consistency without distributed transactions
//
// Our Solution:
//   - Background synchronization runs continuously
//   - Read-only mode during switchover ensures consistency
//   - Change tracking table provides transaction-level guarantees
//   - Simple rollback by not completing the switch
//
// This approach trades brief write unavailability (seconds) for:
//   - Guaranteed consistency at switchover
//   - Simpler error handling
//   - Better performance during normal operation
//   - Clear rollback semantics
//
// # Synchronization Process
//
// With Change Tracking Table:
//  1. Every write operation records changes within the same transaction
//  2. Background process reads unprocessed changes from tracking table
//  3. Apply changes to secondary store in order
//  4. Mark changes as processed or failed
//  5. Retry failed changes with exponential backoff
//
// With Timestamp-Based Sync:
//  1. Track time windows for synchronization
//  2. Query records where created_at >= start OR updated_at >= start
//  3. Copy or update records to secondary store
//  4. Handle deletes through full table comparison
//  5. Multiple passes for active systems
//
// Change tracking provides superior consistency guarantees while timestamp
// sync offers simplicity when modifying the schema is not possible.
//
// # Consistency Guarantees and Trade-offs
//
// [CQRSStore] provides eventual consistency with configurable time windows:
//   - Write operations may have temporary inconsistency between stores
//   - Read operations are consistent within the selected store (primary/secondary)
//   - Synchronization operations are idempotent and can be run multiple times
//   - Time-based windows provide bounded inconsistency periods
//
// The trade-off favors availability and partition tolerance over immediate consistency,
// following the CAP theorem principles for distributed systems.
//
// # Migration Mode Operations
//
// Each migration mode provides different operational characteristics:
//
// [ModeSingle]: Standard single-store operation where all reads and writes
// go to the primary store with no synchronization overhead. Background sync
// may run in this mode to prepare the secondary store.
//
// [ModeReadOnly]: All write operations return errors while reads continue
// from the primary store. Use this mode during final synchronization to
// ensure no new changes occur during the switchover process. This brief
// downtime (typically seconds) guarantees consistency.
//
// [ModeSwitching]: Reads come from the secondary store while writes still
// go to the primary. This mode validates that the secondary store can
// handle production read traffic before committing to the migration.
// Rollback is simple: just switch back to ModeSingle.
//
// # Error Handling and Resilience
//
// [CQRSStore] is designed for operational resilience:
//   - Background sync failures don't affect primary operations
//   - Individual entity sync failures are logged and retried
//   - Read-only mode prevents inconsistency during switchover
//   - Change tracking table persists changes for reliable replay
//   - Clear rollback path at each migration stage
//
// # Monitoring and Observability
//
// The implementation provides visibility into migration progress:
//   - Mode transition logging for operational tracking
//   - Sync operation metrics (records processed, failures, timing)
//   - Consistency validation results during dual-read mode
//   - Error logging with context for debugging failures
//
// # Production Deployment Considerations
//
// For production use, enhance this implementation with:
//   - Checksum validation to detect data corruption
//   - Metrics for sync lag, throughput, and error rates
//   - Automated switchover orchestration
//   - Health checks before mode transitions
//   - Canary deployments for gradual traffic shifting
//   - Backup and restore procedures for rollback
//
// # Usage Example
//
//	// Setup CQRS store for migration
//	primary, _ := postgres.NewPostgresStore(postgresDSN)
//	secondary, _ := surrealdb.NewSurrealStoreCBOR(surrealURL, ns, db, user, pass)
//
//	cqrsStore := cqrs.NewCQRSStore(primary, secondary, cqrs.ModeSingle)
//	defer cqrsStore.Close()
//
//	// Configure sync strategy (change tracking or timestamp)
//	cqrsStore.SetSyncStrategy(cqrs.SyncStrategyChangeTracking)
//
//	// Start background synchronization
//	cqrsStore.StartContinuousSync(ctx, 30*time.Second)
//
//	// Use with application
//	app := surrealnote.NewApp(cqrsStore, config)
//
//	// Migration sequence:
//	// 1. Let background sync run until caught up
//	// 2. Switch to read-only mode for final sync
//	cqrsStore.SetMode(cqrs.ModeReadOnly)
//	time.Sleep(5 * time.Second) // Brief downtime for final sync
//
//	// 3. Switch to secondary for reads
//	cqrsStore.SetMode(cqrs.ModeSwitching)
//
//	// 4. After validation, complete migration
//	cqrsStore.SwapStores()
//	cqrsStore.SetMode(cqrs.ModeSingle)
package cqrs

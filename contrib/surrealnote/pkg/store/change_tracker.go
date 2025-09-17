package store

import (
	"context"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// ChangeTracker defines operations for tracking database changes
// during migration. This interface is implemented by stores that
// support change tracking table functionality.
type ChangeTracker interface {
	// RecordChange records a database change to the change tracking table.
	// This method should be called within the same transaction as the
	// actual data modification to ensure consistency.
	RecordChange(ctx context.Context, entityType string, entityID string, operation models.ChangeOperation, payload models.JSONMap) error

	// ListUnprocessedChanges returns changes that haven't been synchronized yet.
	// Results are ordered by ChangedAt timestamp for sequential processing.
	ListUnprocessedChanges(ctx context.Context, limit int) ([]*models.ChangeTracking, error)

	// ListChangesSince returns changes since the specified timestamp.
	// This supports catch-up synchronization after outages.
	ListChangesSince(ctx context.Context, since time.Time, limit int) ([]*models.ChangeTracking, error)

	// MarkChangeProcessed marks a change as successfully synchronized.
	MarkChangeProcessed(ctx context.Context, changeID uint64) error

	// MarkChangeError marks a change as failed with an error message.
	// The retry count is incremented for failure tracking.
	MarkChangeError(ctx context.Context, changeID uint64, errorMessage string) error

	// GetChangeStats returns statistics about pending changes.
	GetChangeStats(ctx context.Context) (*ChangeStats, error)

	// PurgeProcessedChanges removes old processed changes for cleanup.
	// Keeps changes for the specified retention period for audit.
	PurgeProcessedChanges(ctx context.Context, before time.Time) error
}

// ChangeStats provides statistics about the change tracking table
type ChangeStats struct {
	TotalChanges      int64
	ProcessedChanges  int64
	PendingChanges    int64
	FailedChanges     int64
	OldestPendingTime *time.Time
	LatestChangeTime  *time.Time
}

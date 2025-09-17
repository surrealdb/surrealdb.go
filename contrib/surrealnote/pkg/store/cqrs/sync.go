package cqrs

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
)

// SyncWithStrategy performs synchronization using the configured strategy
func (c *CQRSStore) SyncWithStrategy(ctx context.Context, since, until time.Time) error {
	c.mu.RLock()
	strategy := c.syncStrategy
	c.mu.RUnlock()

	switch strategy {
	case SyncStrategyTimestamp:
		return c.SyncMissedUpdates(ctx, since, until)
	case SyncStrategyChangeTracking:
		return c.SyncFromChangeTracking(ctx, since, until)
	default:
		return fmt.Errorf("unknown sync strategy: %s", strategy)
	}
}

// SyncFromChangeTracking performs synchronization using the change tracking table
func (c *CQRSStore) SyncFromChangeTracking(ctx context.Context, since, until time.Time) error {
	// Type assert to check if primary supports change tracking
	tracker, ok := c.primary.(store.ChangeTracker)
	if !ok {
		// Fallback to timestamp-based sync if primary doesn't support change tracking
		return c.SyncMissedUpdates(ctx, since, until)
	}

	// Get unprocessed changes within the time range
	changes, err := tracker.ListChangesSince(ctx, since, 1000) // Process in batches of 1000
	if err != nil {
		return fmt.Errorf("failed to list changes: %w", err)
	}

	// Process each change
	for _, change := range changes {
		// Skip if already processed or outside our time window
		if change.ChangedAt.After(until) {
			continue
		}

		if err := c.processChange(ctx, change); err != nil {
			// Mark as error and continue
			tracker.MarkChangeError(ctx, change.ID, err.Error())
			fmt.Printf("Warning: failed to process change %d: %v\n", change.ID, err)
		} else {
			// Mark as processed
			tracker.MarkChangeProcessed(ctx, change.ID)
		}
	}

	return nil
}

// processChange applies a single change from the tracking table to the secondary store
func (c *CQRSStore) processChange(ctx context.Context, change *models.ChangeTracking) error {
	switch change.Operation {
	case models.ChangeOperationCreate:
		return c.processCreateChange(ctx, change)
	case models.ChangeOperationUpdate:
		return c.processUpdateChange(ctx, change)
	case models.ChangeOperationDelete:
		return c.processDeleteChange(ctx, change)
	default:
		return fmt.Errorf("unknown change operation: %s", change.Operation)
	}
}

// processEntityChange handles CREATE or UPDATE operations from the change tracking table
func (c *CQRSStore) processEntityChange(ctx context.Context, change *models.ChangeTracking, isCreate bool) error {
	switch change.EntityType {
	case "workspace":
		var workspace models.Workspace
		if err := mapToStruct(change.Payload, &workspace); err != nil {
			return fmt.Errorf("failed to unmarshal workspace: %w", err)
		}
		if isCreate {
			return c.secondary.CreateWorkspace(ctx, &workspace)
		}
		return c.secondary.UpdateWorkspace(ctx, &workspace)

	case "page":
		var page models.Page
		if err := mapToStruct(change.Payload, &page); err != nil {
			return fmt.Errorf("failed to unmarshal page: %w", err)
		}
		if isCreate {
			return c.secondary.CreatePage(ctx, &page)
		}
		return c.secondary.UpdatePage(ctx, &page)

	case "block":
		var block models.Block
		if err := mapToStruct(change.Payload, &block); err != nil {
			return fmt.Errorf("failed to unmarshal block: %w", err)
		}
		if isCreate {
			return c.secondary.CreateBlock(ctx, &block)
		}
		return c.secondary.UpdateBlock(ctx, &block)

	case "user":
		var user models.User
		if err := mapToStruct(change.Payload, &user); err != nil {
			return fmt.Errorf("failed to unmarshal user: %w", err)
		}
		if isCreate {
			return c.secondary.CreateUser(ctx, &user)
		}
		return c.secondary.UpdateUser(ctx, &user)

	case "comment":
		var comment models.Comment
		if err := mapToStruct(change.Payload, &comment); err != nil {
			return fmt.Errorf("failed to unmarshal comment: %w", err)
		}
		if isCreate {
			return c.secondary.CreateComment(ctx, &comment)
		}
		return c.secondary.UpdateComment(ctx, &comment)

	case "permission":
		var permission models.Permission
		if err := mapToStruct(change.Payload, &permission); err != nil {
			return fmt.Errorf("failed to unmarshal permission: %w", err)
		}
		if isCreate {
			return c.secondary.CreatePermission(ctx, &permission)
		}
		return c.secondary.UpdatePermission(ctx, &permission)

	default:
		return fmt.Errorf("unknown entity type: %s", change.EntityType)
	}
}

// processCreateChange handles CREATE operations from the change tracking table
func (c *CQRSStore) processCreateChange(ctx context.Context, change *models.ChangeTracking) error {
	return c.processEntityChange(ctx, change, true)
}

// processUpdateChange handles UPDATE operations from the change tracking table
func (c *CQRSStore) processUpdateChange(ctx context.Context, change *models.ChangeTracking) error {
	return c.processEntityChange(ctx, change, false)
}

// processDeleteChange handles DELETE operations from the change tracking table
func (c *CQRSStore) processDeleteChange(ctx context.Context, change *models.ChangeTracking) error {
	switch change.EntityType {
	case "workspace":
		id, err := models.ParseWorkspaceID(change.EntityID)
		if err != nil {
			return fmt.Errorf("failed to parse workspace ID: %w", err)
		}
		return c.secondary.DeleteWorkspace(ctx, id)

	case "page":
		id, err := models.ParsePageID(change.EntityID)
		if err != nil {
			return fmt.Errorf("failed to parse page ID: %w", err)
		}
		return c.secondary.DeletePage(ctx, id)

	case "block":
		id, err := models.ParseBlockID(change.EntityID)
		if err != nil {
			return fmt.Errorf("failed to parse block ID: %w", err)
		}
		return c.secondary.DeleteBlock(ctx, id)

	case "user":
		id, err := models.ParseUserID(change.EntityID)
		if err != nil {
			return fmt.Errorf("failed to parse user ID: %w", err)
		}
		return c.secondary.DeleteUser(ctx, id)

	case "comment":
		id, err := models.ParseCommentID(change.EntityID)
		if err != nil {
			return fmt.Errorf("failed to parse comment ID: %w", err)
		}
		return c.secondary.DeleteComment(ctx, id)

	case "permission":
		id, err := models.ParsePermissionID(change.EntityID)
		if err != nil {
			return fmt.Errorf("failed to parse permission ID: %w", err)
		}
		return c.secondary.DeletePermission(ctx, id)

	default:
		return fmt.Errorf("unknown entity type: %s", change.EntityType)
	}
}

// mapToStruct converts a JSONMap to a struct
func mapToStruct(data models.JSONMap, target interface{}) error {
	// This is a simplified implementation
	// In production, you'd use a proper JSON marshaling/unmarshaling library
	// or reflection to properly convert the map to the struct

	// For now, we'll use a simple approach
	// You might want to use mapstructure or similar library for this
	return fmt.Errorf("mapToStruct not fully implemented - use proper JSON conversion")
}

// GetSyncStats returns statistics about pending synchronization
func (c *CQRSStore) GetSyncStats(ctx context.Context) (*store.ChangeStats, error) {
	// Try to get stats from change tracking if available
	if tracker, ok := c.primary.(store.ChangeTracker); ok {
		return tracker.GetChangeStats(ctx)
	}

	// Otherwise return nil (no stats available for timestamp-based sync)
	return nil, fmt.Errorf("sync stats only available with change tracking strategy")
}

// StartContinuousSync starts a background process that continuously syncs changes
func (c *CQRSStore) StartContinuousSync(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		lastSync := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				if err := c.SyncWithStrategy(ctx, lastSync, now); err != nil {
					fmt.Printf("Continuous sync error: %v\n", err)
				}
				lastSync = now
			}
		}
	}()
}

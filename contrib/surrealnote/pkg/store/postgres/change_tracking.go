package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
	"gorm.io/gorm"
)

// recordChange records a change to the change tracking table
// This should be called within the same transaction as the main operation
func (s *PostgresStore) recordChange(tx *gorm.DB, entityType string, entityID string, operation models.ChangeOperation, entity interface{}) error {
	// Convert entity to JSONMap for payload
	var payload models.JSONMap
	if entity != nil && (operation == models.ChangeOperationCreate || operation == models.ChangeOperationUpdate) {
		jsonData, err := json.Marshal(entity)
		if err != nil {
			return fmt.Errorf("failed to marshal entity: %w", err)
		}
		if err := json.Unmarshal(jsonData, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal to JSONMap: %w", err)
		}
	}

	change := &models.ChangeTracking{
		EntityType: entityType,
		EntityID:   entityID,
		Operation:  operation,
		ChangedAt:  time.Now(),
		Payload:    payload,
	}

	return tx.Create(change).Error
}

// RecordChange records a database change to the change tracking table
func (s *PostgresStore) RecordChange(ctx context.Context, entityType string, entityID string, operation models.ChangeOperation, payload models.JSONMap) error {
	change := &models.ChangeTracking{
		EntityType: entityType,
		EntityID:   entityID,
		Operation:  operation,
		ChangedAt:  time.Now(),
		Payload:    payload,
	}
	return s.getDB().WithContext(ctx).Create(change).Error
}

// ListUnprocessedChanges returns changes that haven't been synchronized yet
func (s *PostgresStore) ListUnprocessedChanges(ctx context.Context, limit int) ([]*models.ChangeTracking, error) {
	var changes []*models.ChangeTracking
	query := s.getDB().WithContext(ctx).
		Where("processed_at IS NULL OR error_message != ''").
		Order("changed_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&changes).Error
	return changes, err
}

// ListChangesSince returns changes since the specified timestamp
func (s *PostgresStore) ListChangesSince(ctx context.Context, since time.Time, limit int) ([]*models.ChangeTracking, error) {
	var changes []*models.ChangeTracking
	query := s.getDB().WithContext(ctx).
		Where("changed_at >= ?", since).
		Order("changed_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&changes).Error
	return changes, err
}

// MarkChangeProcessed marks a change as successfully synchronized
func (s *PostgresStore) MarkChangeProcessed(ctx context.Context, changeID uint64) error {
	now := time.Now()
	return s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Where("id = ?", changeID).
		Updates(map[string]interface{}{
			"processed_at":  &now,
			"error_message": "",
		}).Error
}

// MarkChangeError marks a change as failed with an error message
func (s *PostgresStore) MarkChangeError(ctx context.Context, changeID uint64, errorMessage string) error {
	return s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Where("id = ?", changeID).
		Updates(map[string]interface{}{
			"error_message": errorMessage,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}).Error
}

// GetChangeStats returns statistics about pending changes
func (s *PostgresStore) GetChangeStats(ctx context.Context) (*store.ChangeStats, error) {
	stats := &store.ChangeStats{}

	// Get total changes
	if err := s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Count(&stats.TotalChanges).Error; err != nil {
		return nil, err
	}

	// Get processed changes
	if err := s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Where("processed_at IS NOT NULL AND error_message = ''").
		Count(&stats.ProcessedChanges).Error; err != nil {
		return nil, err
	}

	// Get pending changes
	if err := s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Where("processed_at IS NULL").
		Count(&stats.PendingChanges).Error; err != nil {
		return nil, err
	}

	// Get failed changes
	if err := s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Where("error_message != ''").
		Count(&stats.FailedChanges).Error; err != nil {
		return nil, err
	}

	// Get oldest pending time
	var oldestChange models.ChangeTracking
	if err := s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Where("processed_at IS NULL").
		Order("changed_at ASC").
		First(&oldestChange).Error; err == nil {
		stats.OldestPendingTime = &oldestChange.ChangedAt
	}

	// Get latest change time
	var latestChange models.ChangeTracking
	if err := s.getDB().WithContext(ctx).
		Model(&models.ChangeTracking{}).
		Order("changed_at DESC").
		First(&latestChange).Error; err == nil {
		stats.LatestChangeTime = &latestChange.ChangedAt
	}

	return stats, nil
}

// PurgeProcessedChanges removes old processed changes for cleanup
func (s *PostgresStore) PurgeProcessedChanges(ctx context.Context, before time.Time) error {
	return s.getDB().WithContext(ctx).
		Where("processed_at IS NOT NULL AND processed_at < ? AND error_message = ''", before).
		Delete(&models.ChangeTracking{}).Error
}

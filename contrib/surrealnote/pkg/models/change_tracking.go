package models

import (
	"time"
)

// ChangeOperation represents the type of database change
type ChangeOperation string

const (
	ChangeOperationCreate ChangeOperation = "CREATE"
	ChangeOperationUpdate ChangeOperation = "UPDATE"
	ChangeOperationDelete ChangeOperation = "DELETE"
)

// ChangeTracking represents a record in the change tracking table
// for capturing all database modifications during migration.
//
// This model enables precise change tracking without relying on
// CreatedAt/UpdatedAt fields, providing better guarantees for
// data synchronization between PostgreSQL and SurrealDB.
//
// The change tracking table captures changes within database transactions,
// ensuring that no modifications are missed during the migration process.
// Each change record includes the entity type, ID, operation, and timestamp
// for accurate replay to the secondary store.
type ChangeTracking struct {
	ID           uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	EntityType   string          `gorm:"not null;index:idx_entity_timestamp" json:"entity_type"`
	EntityID     string          `gorm:"not null;index:idx_entity_timestamp" json:"entity_id"`
	Operation    ChangeOperation `gorm:"not null" json:"operation"`
	ChangedAt    time.Time       `gorm:"not null;index:idx_entity_timestamp" json:"changed_at"`
	ProcessedAt  *time.Time      `gorm:"index" json:"processed_at,omitempty"`
	ErrorMessage string          `gorm:"type:text" json:"error_message,omitempty"`
	RetryCount   int             `gorm:"default:0" json:"retry_count"`
	// Payload stores the entity data for CREATE/UPDATE operations as JSON
	// This allows replay of changes without querying the main tables
	Payload JSONMap `gorm:"type:jsonb" json:"payload,omitempty"`
}

// TableName returns the table name for the change tracking model
func (ChangeTracking) TableName() string {
	return "change_tracking"
}

// IsProcessed returns true if the change has been successfully processed
func (c *ChangeTracking) IsProcessed() bool {
	return c.ProcessedAt != nil && c.ErrorMessage == ""
}

// MarkProcessed marks the change as successfully processed
func (c *ChangeTracking) MarkProcessed(processedTime time.Time) {
	c.ProcessedAt = &processedTime
	c.ErrorMessage = ""
}

// MarkError marks the change as failed with an error message
func (c *ChangeTracking) MarkError(errorMsg string) {
	c.ErrorMessage = errorMsg
	c.RetryCount++
}

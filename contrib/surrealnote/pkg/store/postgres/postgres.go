// Package postgres provides PostgreSQL implementation of the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] interface using GORM ORM.
//
// This package demonstrates how to implement the repository pattern with a traditional
// relational database using an Object-Relational Mapping (ORM) framework. It serves
// as a reference implementation for SQL-based backends and showcases ACID transaction
// support with immediate consistency guarantees.
//
// # Implementation Strategy
//
// [PostgresStore] uses GORM as the ORM layer to handle:
//   - Automatic SQL query generation from Go struct operations
//   - Foreign key relationship management and cascade operations
//   - Type-safe database operations with compile-time validation
//   - Built-in connection pooling and prepared statement caching
//   - Automatic schema migration through GORM's AutoMigrate feature
//
// This approach contrasts with the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/surrealdb.SurrealStoreCBOR] implementation which
// uses native query language without ORM abstractions.
//
// # Data Model Mapping
//
// The PostgreSQL schema directly maps [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models] entities to relational tables:
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Workspace] → workspaces table with owner foreign key
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Page] → pages table with workspace and parent page foreign keys
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Block] → blocks table with page and parent block foreign keys
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.User] → users table with unique email constraint
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Permission] → permissions table with polymorphic resource references
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Comment] → comments table with block and user foreign keys
//
// GORM struct tags define database constraints, indexes, and relationships
// automatically enforced at the database level.
//
// # Transaction and Consistency Model
//
// PostgreSQL provides ACID guarantees with strong consistency:
//   - Atomicity: All operations within a transaction succeed or fail together
//   - Consistency: Database constraints are enforced at commit time
//   - Isolation: Concurrent transactions don't interfere with each other
//   - Durability: Committed data survives system failures
//
// Individual [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] operations are automatically wrapped in transactions
// by GORM, ensuring data integrity without explicit transaction management.
//
// # Performance Characteristics
//
// PostgreSQL operations exhibit these performance characteristics:
//   - High throughput for read operations with proper indexing
//   - ACID overhead for write operations (slower than eventual consistency)
//   - Excellent performance for complex relational queries and joins
//   - Predictable performance scaling with proper connection pooling
//   - Strong performance for analytical queries across related entities
//
// # Schema Migration
//
// The [PostgresStore.Migrate] method uses GORM's AutoMigrate feature to:
//   - Create missing tables based on model struct definitions
//   - Add missing columns when models are extended
//   - Create indexes specified in GORM struct tags
//   - Establish foreign key constraints for relationships
//
// AutoMigrate is safe for production use as it only adds schema elements
// and never removes existing data or columns.
//
// # Production Considerations
//
// For production deployment, enhance this implementation with:
//   - Connection pool configuration (max connections, timeouts, lifetime)
//   - Query performance monitoring and slow query logging
//   - Retry logic with exponential backoff for transient connection failures
//   - Circuit breaker pattern for database failure scenarios
//   - Read replica support for read-heavy workloads
//   - Prepared statement optimization for repeated queries
//   - Connection health checks and automatic reconnection
//
// # CQRS Support
//
// [PostgresStore] supports CQRS migration patterns through:
//   - Timestamp-based change tracking using CreatedAt/UpdatedAt fields
//   - ListModified*IDs methods for efficient change detection
//   - Idempotent operations that support dual-write scenarios
//   - Consistent timestamp handling for synchronization windows
//
// These features enable [PostgresStore] to work as either primary or secondary
// store in [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs.CQRSStore] migration scenarios.
//
// # Usage Example
//
//	store, err := postgres.NewPostgresStore("postgres://user:pass@localhost/db")
//	if err != nil {
//		return err
//	}
//	defer store.Close()
//
//	// Initialize schema
//	if err := store.Migrate(ctx); err != nil {
//		return err
//	}
//
//	// Use with application
//	app := surrealnote.NewApp(store, config)
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgresStore implements the Store interface using PostgreSQL with GORM.
// A production system would add connection pool configuration, query metrics,
// and implement circuit breaker pattern for database failures.
type PostgresStore struct {
	db *gorm.DB
	// Missing: connection pool, metrics collector, circuit breaker
}

// NewPostgresStore creates a new PostgreSQL store.
// A production system would configure connection pooling, set timeouts,
// enable query logging for slow queries, and validate the connection.
func NewPostgresStore(dsn string) (store.Store, error) {
	// Should configure: MaxIdleConns, MaxOpenConns, ConnMaxLifetime, ConnMaxIdleTime
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// getDB returns the database connection
func (s *PostgresStore) getDB() *gorm.DB {
	return s.db
}

// Migrate performs PostgreSQL schema migration using GORM's AutoMigrate feature.
// This method creates all necessary tables, columns, indexes, and foreign key constraints
// for the surrealnote data model if they don't already exist.
//
// AutoMigrate operations performed:
//   - Creates tables for User, Workspace, Page, Block, Permission, and Comment models
//   - Adds missing columns to existing tables
//   - Creates indexes defined in model struct tags
//   - Establishes foreign key relationships between tables
//   - Updates column types if they've changed (with some limitations)
//
// This method is safe to run repeatedly - it only creates missing schema elements
// and doesn't drop or modify existing data. GORM's AutoMigrate has some limitations:
//   - It won't rename columns or tables
//   - It won't delete unused columns
//   - Complex constraint modifications may require manual intervention
//
// For production deployments, consider using explicit migration scripts for better
// control over schema changes and data preservation.
//
// Returns an error if the migration fails due to database connectivity issues,
// permission problems, or SQL constraint violations.
func (s *PostgresStore) Migrate(ctx context.Context) error {
	// Auto-migrate all models including change tracking
	return s.db.AutoMigrate(
		&models.User{},
		&models.Workspace{},
		&models.Page{},
		&models.Block{},
		&models.Permission{},
		&models.Comment{},
		&models.ChangeTracking{}, // Add change tracking table
	)
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Note: Transaction methods (BeginTx, Commit, Rollback) were removed.
// Transactions are handled internally within each operation as needed.
// For CQRS consistency, see the timestamp-based catch-up implementation.

// Workspace operations
func (s *PostgresStore) CreateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	return s.getDB().WithContext(ctx).Create(workspace).Error
}

func (s *PostgresStore) GetWorkspace(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error) {
	var workspace models.Workspace
	err := s.getDB().WithContext(ctx).First(&workspace, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &workspace, nil
}

func (s *PostgresStore) UpdateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	return s.getDB().WithContext(ctx).Save(workspace).Error
}

func (s *PostgresStore) DeleteWorkspace(ctx context.Context, id models.WorkspaceID) error {
	return s.getDB().WithContext(ctx).Delete(&models.Workspace{}, "id = ?", id).Error
}

func (s *PostgresStore) ListWorkspaces(ctx context.Context, ownerID models.UserID) ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	err := s.getDB().WithContext(ctx).Where("owner_id = ?", ownerID).Find(&workspaces).Error
	return workspaces, err
}

// Page operations
func (s *PostgresStore) CreatePage(ctx context.Context, page *models.Page) error {
	return s.getDB().WithContext(ctx).Create(page).Error
}

func (s *PostgresStore) GetPage(ctx context.Context, id models.PageID) (*models.Page, error) {
	var page models.Page
	err := s.getDB().WithContext(ctx).First(&page, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &page, nil
}

func (s *PostgresStore) UpdatePage(ctx context.Context, page *models.Page) error {
	return s.getDB().WithContext(ctx).Save(page).Error
}

func (s *PostgresStore) DeletePage(ctx context.Context, id models.PageID) error {
	return s.getDB().WithContext(ctx).Delete(&models.Page{}, "id = ?", id).Error
}

func (s *PostgresStore) ListPages(ctx context.Context, workspaceID models.WorkspaceID) ([]*models.Page, error) {
	var pages []*models.Page
	err := s.getDB().WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&pages).Error
	return pages, err
}

func (s *PostgresStore) ListChildPages(ctx context.Context, parentPageID models.PageID) ([]*models.Page, error) {
	var pages []*models.Page
	err := s.getDB().WithContext(ctx).Where("parent_page_id = ?", parentPageID).Find(&pages).Error
	return pages, err
}

// Block operations
func (s *PostgresStore) CreateBlock(ctx context.Context, block *models.Block) error {
	return s.getDB().WithContext(ctx).Create(block).Error
}

func (s *PostgresStore) GetBlock(ctx context.Context, id models.BlockID) (*models.Block, error) {
	var block models.Block
	err := s.getDB().WithContext(ctx).First(&block, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &block, nil
}

func (s *PostgresStore) UpdateBlock(ctx context.Context, block *models.Block) error {
	return s.getDB().WithContext(ctx).Save(block).Error
}

func (s *PostgresStore) DeleteBlock(ctx context.Context, id models.BlockID) error {
	return s.getDB().WithContext(ctx).Delete(&models.Block{}, "id = ?", id).Error
}

func (s *PostgresStore) ListBlocks(ctx context.Context, pageID models.PageID) ([]*models.Block, error) {
	var blocks []*models.Block
	err := s.getDB().WithContext(ctx).Where("page_id = ?", pageID).Order("\"order\"").Find(&blocks).Error
	return blocks, err
}

func (s *PostgresStore) ReorderBlocks(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error {
	// Update the order field for each block
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, blockID := range blockIDs {
			if err := tx.Model(&models.Block{}).Where("id = ? AND page_id = ?", blockID, pageID).Update("order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// User operations
func (s *PostgresStore) CreateUser(ctx context.Context, user *models.User) error {
	return s.getDB().WithContext(ctx).Create(user).Error
}

func (s *PostgresStore) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	var user models.User
	err := s.getDB().WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := s.getDB().WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) UpdateUser(ctx context.Context, user *models.User) error {
	return s.getDB().WithContext(ctx).Save(user).Error
}

func (s *PostgresStore) DeleteUser(ctx context.Context, id models.UserID) error {
	return s.getDB().WithContext(ctx).Delete(&models.User{}, "id = ?", id).Error
}

// Permission operations
func (s *PostgresStore) CreatePermission(ctx context.Context, permission *models.Permission) error {
	return s.getDB().WithContext(ctx).Create(permission).Error
}

func (s *PostgresStore) GetPermissions(ctx context.Context, resourceType models.ResourceType, resourceID models.ResourceID) ([]*models.Permission, error) {
	var permissions []*models.Permission
	err := s.getDB().WithContext(ctx).Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Find(&permissions).Error
	if err != nil {
		return nil, err
	}
	// Set tableName for ResourceID after loading from PostgreSQL
	for _, perm := range permissions {
		perm.ResourceID.SetTableForResourceType(perm.ResourceType)
	}
	return permissions, nil
}

func (s *PostgresStore) GetUserPermissions(ctx context.Context, userID models.UserID) ([]*models.Permission, error) {
	var permissions []*models.Permission
	err := s.getDB().WithContext(ctx).Where("user_id = ?", userID).Find(&permissions).Error
	if err != nil {
		return nil, err
	}
	// Set tableName for ResourceID after loading from PostgreSQL
	for _, perm := range permissions {
		perm.ResourceID.SetTableForResourceType(perm.ResourceType)
	}
	return permissions, nil
}

func (s *PostgresStore) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return s.getDB().WithContext(ctx).Save(permission).Error
}

func (s *PostgresStore) DeletePermission(ctx context.Context, id models.PermissionID) error {
	return s.getDB().WithContext(ctx).Delete(&models.Permission{}, "id = ?", id).Error
}

func (s *PostgresStore) CheckPermission(ctx context.Context, userID models.UserID, resourceType models.ResourceType, resourceID models.ResourceID, level models.PermissionLevel) (bool, error) {
	var count int64
	err := s.getDB().WithContext(ctx).Model(&models.Permission{}).
		Where("user_id = ? AND resource_type = ? AND resource_id = ? AND permission_level = ?",
			userID, resourceType, resourceID, level).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Comment operations
func (s *PostgresStore) CreateComment(ctx context.Context, comment *models.Comment) error {
	return s.getDB().WithContext(ctx).Create(comment).Error
}

func (s *PostgresStore) GetComment(ctx context.Context, id models.CommentID) (*models.Comment, error) {
	var comment models.Comment
	err := s.getDB().WithContext(ctx).First(&comment, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &comment, nil
}

func (s *PostgresStore) ListComments(ctx context.Context, blockID models.BlockID) ([]*models.Comment, error) {
	var comments []*models.Comment
	err := s.getDB().WithContext(ctx).Where("block_id = ?", blockID).Order("created_at").Find(&comments).Error
	return comments, err
}

func (s *PostgresStore) UpdateComment(ctx context.Context, comment *models.Comment) error {
	return s.getDB().WithContext(ctx).Save(comment).Error
}

func (s *PostgresStore) DeleteComment(ctx context.Context, id models.CommentID) error {
	return s.getDB().WithContext(ctx).Delete(&models.Comment{}, "id = ?", id).Error
}

func (s *PostgresStore) ResolveComment(ctx context.Context, id models.CommentID) error {
	return s.getDB().WithContext(ctx).Model(&models.Comment{}).Where("id = ?", id).Update("resolved_at", sql.NullTime{Time: gorm.DeletedAt{}.Time, Valid: true}).Error
}

// Timestamp-based catch-up methods for CQRS consistency

func (s *PostgresStore) ListModifiedWorkspaceIDs(ctx context.Context, since, until time.Time) ([]models.WorkspaceID, error) {
	var ids []models.WorkspaceID
	err := s.getDB().WithContext(ctx).
		Model(&models.Workspace{}).
		Where("created_at >= ? AND created_at <= ?", since, until).
		Or("updated_at >= ? AND updated_at <= ?", since, until).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *PostgresStore) ListModifiedPageIDs(ctx context.Context, since, until time.Time) ([]models.PageID, error) {
	var ids []models.PageID
	err := s.getDB().WithContext(ctx).
		Model(&models.Page{}).
		Where("created_at >= ? AND created_at <= ?", since, until).
		Or("updated_at >= ? AND updated_at <= ?", since, until).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *PostgresStore) ListModifiedBlockIDs(ctx context.Context, since, until time.Time) ([]models.BlockID, error) {
	var ids []models.BlockID
	err := s.getDB().WithContext(ctx).
		Model(&models.Block{}).
		Where("created_at >= ? AND created_at <= ?", since, until).
		Or("updated_at >= ? AND updated_at <= ?", since, until).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *PostgresStore) ListModifiedUserIDs(ctx context.Context, since, until time.Time) ([]models.UserID, error) {
	var ids []models.UserID
	err := s.getDB().WithContext(ctx).
		Model(&models.User{}).
		Where("(created_at >= ? AND created_at <= ?)", since, until).
		Or("(updated_at >= ? AND updated_at <= ?)", since, until).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *PostgresStore) ListModifiedCommentIDs(ctx context.Context, since, until time.Time) ([]models.CommentID, error) {
	var ids []models.CommentID
	err := s.getDB().WithContext(ctx).
		Model(&models.Comment{}).
		Where("created_at >= ? AND created_at <= ?", since, until).
		Or("updated_at >= ? AND updated_at <= ?", since, until).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *PostgresStore) ListModifiedPermissionIDs(ctx context.Context, since, until time.Time) ([]models.PermissionID, error) {
	var ids []models.PermissionID
	err := s.getDB().WithContext(ctx).
		Model(&models.Permission{}).
		Where("created_at >= ? AND created_at <= ?", since, until).
		Or("updated_at >= ? AND updated_at <= ?", since, until).
		Pluck("id", &ids).Error
	return ids, err
}

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"gorm.io/gorm"
)

// EnableChangeTracking enables change tracking for the store
// When enabled, all write operations will be recorded in the change tracking table
type EnableChangeTracking bool

// WithChangeTracking wraps write operations in transactions that record changes
func (s *PostgresStore) WithChangeTracking(enabled bool) *PostgresStore {
	// This would be implemented with a configuration flag
	// For now, we'll provide wrapper methods that can be used
	return s
}

// CreateWorkspaceWithTracking creates a workspace and records the change
func (s *PostgresStore) CreateWorkspaceWithTracking(ctx context.Context, workspace *models.Workspace) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the workspace
		if err := tx.Create(workspace).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "workspace", workspace.ID.String(), models.ChangeOperationCreate, workspace)
	})
}

// UpdateWorkspaceWithTracking updates a workspace and records the change
func (s *PostgresStore) UpdateWorkspaceWithTracking(ctx context.Context, workspace *models.Workspace) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the workspace
		if err := tx.Save(workspace).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "workspace", workspace.ID.String(), models.ChangeOperationUpdate, workspace)
	})
}

// DeleteWorkspaceWithTracking deletes a workspace and records the change
func (s *PostgresStore) DeleteWorkspaceWithTracking(ctx context.Context, id models.WorkspaceID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the workspace
		if err := tx.Delete(&models.Workspace{}, "id = ?", id).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "workspace", id.String(), models.ChangeOperationDelete, nil)
	})
}

// CreatePageWithTracking creates a page and records the change
func (s *PostgresStore) CreatePageWithTracking(ctx context.Context, page *models.Page) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the page
		if err := tx.Create(page).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "page", page.ID.String(), models.ChangeOperationCreate, page)
	})
}

// UpdatePageWithTracking updates a page and records the change
func (s *PostgresStore) UpdatePageWithTracking(ctx context.Context, page *models.Page) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the page
		if err := tx.Save(page).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "page", page.ID.String(), models.ChangeOperationUpdate, page)
	})
}

// DeletePageWithTracking deletes a page and records the change
func (s *PostgresStore) DeletePageWithTracking(ctx context.Context, id models.PageID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the page
		if err := tx.Delete(&models.Page{}, "id = ?", id).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "page", id.String(), models.ChangeOperationDelete, nil)
	})
}

// CreateBlockWithTracking creates a block and records the change
func (s *PostgresStore) CreateBlockWithTracking(ctx context.Context, block *models.Block) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the block
		if err := tx.Create(block).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "block", block.ID.String(), models.ChangeOperationCreate, block)
	})
}

// UpdateBlockWithTracking updates a block and records the change
func (s *PostgresStore) UpdateBlockWithTracking(ctx context.Context, block *models.Block) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the block
		if err := tx.Save(block).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "block", block.ID.String(), models.ChangeOperationUpdate, block)
	})
}

// DeleteBlockWithTracking deletes a block and records the change
func (s *PostgresStore) DeleteBlockWithTracking(ctx context.Context, id models.BlockID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the block
		if err := tx.Delete(&models.Block{}, "id = ?", id).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "block", id.String(), models.ChangeOperationDelete, nil)
	})
}

// CreateUserWithTracking creates a user and records the change
func (s *PostgresStore) CreateUserWithTracking(ctx context.Context, user *models.User) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the user
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "user", user.ID.String(), models.ChangeOperationCreate, user)
	})
}

// UpdateUserWithTracking updates a user and records the change
func (s *PostgresStore) UpdateUserWithTracking(ctx context.Context, user *models.User) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the user
		if err := tx.Save(user).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "user", user.ID.String(), models.ChangeOperationUpdate, user)
	})
}

// DeleteUserWithTracking deletes a user and records the change
func (s *PostgresStore) DeleteUserWithTracking(ctx context.Context, id models.UserID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the user
		if err := tx.Delete(&models.User{}, "id = ?", id).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "user", id.String(), models.ChangeOperationDelete, nil)
	})
}

// CreateCommentWithTracking creates a comment and records the change
func (s *PostgresStore) CreateCommentWithTracking(ctx context.Context, comment *models.Comment) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the comment
		if err := tx.Create(comment).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "comment", comment.ID.String(), models.ChangeOperationCreate, comment)
	})
}

// UpdateCommentWithTracking updates a comment and records the change
func (s *PostgresStore) UpdateCommentWithTracking(ctx context.Context, comment *models.Comment) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the comment
		if err := tx.Save(comment).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "comment", comment.ID.String(), models.ChangeOperationUpdate, comment)
	})
}

// DeleteCommentWithTracking deletes a comment and records the change
func (s *PostgresStore) DeleteCommentWithTracking(ctx context.Context, id models.CommentID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the comment
		if err := tx.Delete(&models.Comment{}, "id = ?", id).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "comment", id.String(), models.ChangeOperationDelete, nil)
	})
}

// CreatePermissionWithTracking creates a permission and records the change
func (s *PostgresStore) CreatePermissionWithTracking(ctx context.Context, permission *models.Permission) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the permission
		if err := tx.Create(permission).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "permission", permission.ID.String(), models.ChangeOperationCreate, permission)
	})
}

// UpdatePermissionWithTracking updates a permission and records the change
func (s *PostgresStore) UpdatePermissionWithTracking(ctx context.Context, permission *models.Permission) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the permission
		if err := tx.Save(permission).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "permission", permission.ID.String(), models.ChangeOperationUpdate, permission)
	})
}

// DeletePermissionWithTracking deletes a permission and records the change
func (s *PostgresStore) DeletePermissionWithTracking(ctx context.Context, id models.PermissionID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the permission
		if err := tx.Delete(&models.Permission{}, "id = ?", id).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "permission", id.String(), models.ChangeOperationDelete, nil)
	})
}

// ReorderBlocksWithTracking reorders blocks and records the changes
func (s *PostgresStore) ReorderBlocksWithTracking(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update block orders
		for i, blockID := range blockIDs {
			if err := tx.Model(&models.Block{}).
				Where("id = ? AND page_id = ?", blockID, pageID).
				Update("order", i).Error; err != nil {
				return err
			}

			// Record each order change
			if err := s.recordChange(tx, "block", blockID.String(), models.ChangeOperationUpdate,
				map[string]interface{}{"id": blockID, "page_id": pageID, "order": i}); err != nil {
				return err
			}
		}

		return nil
	})
}

// ResolveCommentWithTracking resolves a comment and records the change
func (s *PostgresStore) ResolveCommentWithTracking(ctx context.Context, id models.CommentID) error {
	return s.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Resolve the comment
		if err := tx.Model(&models.Comment{}).
			Where("id = ?", id).
			Update("resolved_at", &now).Error; err != nil {
			return err
		}

		// Record the change
		return s.recordChange(tx, "comment", id.String(), models.ChangeOperationUpdate,
			map[string]interface{}{"id": id, "resolved_at": &now})
	})
}

// Example of how to configure the store with change tracking
func NewPostgresStoreWithChangeTracking(dsn string) (*PostgresStore, error) {
	store, err := NewPostgresStore(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres store: %w", err)
	}

	// Type assertion to get the concrete type
	pgStore, ok := store.(*PostgresStore)
	if !ok {
		return nil, fmt.Errorf("unexpected store type")
	}

	// Here we would set a flag to enable change tracking
	// For demonstration, the WithTracking methods can be called explicitly
	return pgStore, nil
}

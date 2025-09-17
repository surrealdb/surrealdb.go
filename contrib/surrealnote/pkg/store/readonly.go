package store

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// ReadOnlyStore wraps a Store and prevents write operations when in read-only mode.
//
// This wrapper is primarily used during the final synchronization phase of database
// migrations, where the application needs to temporarily block writes to ensure
// data consistency while performing catch-up synchronization between stores.
//
// The read-only state is determined dynamically by the isReadOnly function,
// allowing the application to toggle between read-write and read-only modes
// without recreating the store instance.
//
// In migration scenarios, the store operates in three phases. During normal operation,
// isReadOnly returns false and all operations pass through. Before the final sync,
// isReadOnly returns true to block new writes and ensure consistency. After sync
// completion, isReadOnly returns false to resume normal operation.
//
// All write operations (Create, Update, Delete) return an error when in read-only mode,
// while read operations (Get, List) continue to work normally.
type ReadOnlyStore struct {
	Store
	isReadOnly func() bool
}

// NewReadOnlyStore creates a new read-only wrapper for a store
func NewReadOnlyStore(store Store, isReadOnly func() bool) Store {
	return &ReadOnlyStore{
		Store:      store,
		isReadOnly: isReadOnly,
	}
}

// Unwrap returns the underlying store
func (r *ReadOnlyStore) Unwrap() Store {
	return r.Store
}

// checkReadOnly returns an error if the store is in read-only mode
func (r *ReadOnlyStore) checkReadOnly() error {
	if r.isReadOnly() {
		return fmt.Errorf("operation denied: application is in read-only mode for data consistency")
	}
	return nil
}

// Write operations - check read-only mode first

func (r *ReadOnlyStore) CreateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.CreateWorkspace(ctx, workspace)
}

func (r *ReadOnlyStore) UpdateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.UpdateWorkspace(ctx, workspace)
}

func (r *ReadOnlyStore) DeleteWorkspace(ctx context.Context, id models.WorkspaceID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.DeleteWorkspace(ctx, id)
}

func (r *ReadOnlyStore) CreatePage(ctx context.Context, page *models.Page) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.CreatePage(ctx, page)
}

func (r *ReadOnlyStore) UpdatePage(ctx context.Context, page *models.Page) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.UpdatePage(ctx, page)
}

func (r *ReadOnlyStore) DeletePage(ctx context.Context, id models.PageID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.DeletePage(ctx, id)
}

func (r *ReadOnlyStore) CreateBlock(ctx context.Context, block *models.Block) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.CreateBlock(ctx, block)
}

func (r *ReadOnlyStore) UpdateBlock(ctx context.Context, block *models.Block) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.UpdateBlock(ctx, block)
}

func (r *ReadOnlyStore) DeleteBlock(ctx context.Context, id models.BlockID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.DeleteBlock(ctx, id)
}

func (r *ReadOnlyStore) ReorderBlocks(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.ReorderBlocks(ctx, pageID, blockIDs)
}

func (r *ReadOnlyStore) CreateUser(ctx context.Context, user *models.User) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.CreateUser(ctx, user)
}

func (r *ReadOnlyStore) UpdateUser(ctx context.Context, user *models.User) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.UpdateUser(ctx, user)
}

func (r *ReadOnlyStore) DeleteUser(ctx context.Context, id models.UserID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.DeleteUser(ctx, id)
}

func (r *ReadOnlyStore) CreatePermission(ctx context.Context, permission *models.Permission) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.CreatePermission(ctx, permission)
}

func (r *ReadOnlyStore) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.UpdatePermission(ctx, permission)
}

func (r *ReadOnlyStore) DeletePermission(ctx context.Context, id models.PermissionID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.DeletePermission(ctx, id)
}

func (r *ReadOnlyStore) CreateComment(ctx context.Context, comment *models.Comment) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.CreateComment(ctx, comment)
}

func (r *ReadOnlyStore) UpdateComment(ctx context.Context, comment *models.Comment) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.UpdateComment(ctx, comment)
}

func (r *ReadOnlyStore) DeleteComment(ctx context.Context, id models.CommentID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.DeleteComment(ctx, id)
}

func (r *ReadOnlyStore) ResolveComment(ctx context.Context, id models.CommentID) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.ResolveComment(ctx, id)
}

func (r *ReadOnlyStore) Migrate(ctx context.Context) error {
	if err := r.checkReadOnly(); err != nil {
		return err
	}
	return r.Store.Migrate(ctx)
}

// Read operations and timestamp-based methods pass through without checks
// These are already defined in the embedded Store interface

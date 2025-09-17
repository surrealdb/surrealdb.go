package cqrs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
)

// MigrationMode defines the mode of database migration
type MigrationMode string

const (
	// ModeSingle operates with only the primary store, used before migration starts
	// or after migration completes. This is the default operational mode with no
	// synchronization overhead.
	ModeSingle MigrationMode = "single"

	// ModeReadOnly puts the application in read-only mode during store switching.
	// All write operations are rejected while reads continue from the primary store.
	// Use this mode during the critical switchover phase to ensure data consistency.
	ModeReadOnly MigrationMode = "read_only"

	// ModeSwitching reads from the secondary store while keeping primary active.
	// Writes are still directed to primary store. This mode validates that the
	// secondary store is ready to become the new primary.
	ModeSwitching MigrationMode = "switching"

	// ModeReversed operates with secondary store as primary for writes.
	// This mode is used before final cutover to validate the secondary store
	// can handle all operations, while keeping primary in sync for rollback.
	ModeReversed MigrationMode = "reversed"
)

// SyncStrategy defines how data synchronization is performed
type SyncStrategy string

const (
	// SyncStrategyTimestamp uses CreatedAt/UpdatedAt fields for change detection.
	// This strategy works with any data model that includes timestamp fields
	// but may include unchanged records in the synchronization window.
	SyncStrategyTimestamp SyncStrategy = "timestamp"

	// SyncStrategyChangeTracking uses a dedicated change tracking table.
	// This strategy provides precise change capture within transactions
	// and doesn't require timestamp fields in the data model.
	SyncStrategyChangeTracking SyncStrategy = "change_tracking"
)

// CQRSStore implements the Store interface without dual-writing.
//
// This implementation supports zero-downtime migration through background
// synchronization and read-only mode switching, eliminating the complexity
// and potential inconsistencies of dual-write patterns.
//
// Migration flow:
//  1. Start with ModeSingle using primary store
//  2. Run background sync (timestamp or change tracking based)
//  3. Switch to ModeReadOnly for final catch-up sync
//  4. Switch to ModeSwitching to validate secondary store
//  5. Complete with ModeSingle using secondary as new primary
//
// This approach ensures:
//   - No dual-write complexity or partial failure scenarios
//   - Consistent data through read-only switching phase
//   - Background sync minimizes downtime to switching duration only
//   - Change tracking provides transaction-level consistency
type CQRSStore struct {
	primary      store.Store
	secondary    store.Store
	mode         MigrationMode
	syncStrategy SyncStrategy
	mu           sync.RWMutex
}

// NewCQRSStore creates a new CQRS store for database migration
func NewCQRSStore(primary, secondary store.Store, mode MigrationMode) store.Store {
	return &CQRSStore{
		primary:      primary,
		secondary:    secondary,
		mode:         mode,
		syncStrategy: SyncStrategyTimestamp,
	}
}

// SetMode changes the migration mode
func (c *CQRSStore) SetMode(mode MigrationMode) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate mode transition
	if c.mode == ModeReadOnly && mode != ModeSwitching && mode != ModeSingle {
		return fmt.Errorf("can only transition from read_only to switching or single mode")
	}

	c.mode = mode
	return nil
}

// SetSyncStrategy sets the synchronization strategy
func (c *CQRSStore) SetSyncStrategy(strategy SyncStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncStrategy = strategy
}

// GetSyncStrategy returns the current synchronization strategy
func (c *CQRSStore) GetSyncStrategy() SyncStrategy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.syncStrategy
}

// SwapStores swaps primary and secondary stores
// This is used after successful migration to make secondary the new primary
func (c *CQRSStore) SwapStores() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.primary, c.secondary = c.secondary, c.primary
}

// GetMode returns the current migration mode
func (c *CQRSStore) GetMode() MigrationMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode
}

// getReadStore returns the appropriate store for read operations
func (c *CQRSStore) getReadStore() store.Store {
	switch c.mode {
	case ModeSwitching, ModeReversed:
		// In switching mode: test reads from secondary
		// In reversed mode: secondary is primary for both reads and writes
		return c.secondary
	default:
		return c.primary
	}
}

// getWriteStore returns the appropriate store for write operations
func (c *CQRSStore) getWriteStore() (store.Store, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mode == ModeReadOnly {
		return nil, fmt.Errorf("system is in read-only mode during migration")
	}

	// In reversed mode, write to secondary store (SurrealDB)
	if c.mode == ModeReversed {
		return c.secondary, nil
	}

	// Otherwise write to primary store (PostgreSQL)
	return c.primary, nil
}

// Migrate performs schema migration on both primary and secondary stores in the CQRS setup.
// This ensures that both databases have identical schema definitions, which is critical
// for maintaining data consistency during the zero-downtime migration process.
//
// Migration sequence:
//  1. Migrate the primary store first (typically PostgreSQL)
//  2. If successful, migrate the secondary store (typically SurrealDB)
//  3. If either migration fails, return an error and halt the process
//
// Both stores must have compatible schemas to support the dual-write operations
// that occur during migration phases. Schema inconsistencies between stores would
// lead to write failures and data synchronization issues.
//
// Error handling:
//   - If primary migration fails: Operation stops, no changes to secondary
//   - If secondary migration fails: Operation stops, primary already migrated
//
// This method is essential for preparing both databases before starting the
// actual data migration process. It should be run before switching to dual-write mode.
//
// Returns an error if migration fails on either store, with a descriptive message
// indicating which store failed and the underlying cause.
func (c *CQRSStore) Migrate(ctx context.Context) error {
	// Always migrate both stores
	if err := c.primary.Migrate(ctx); err != nil {
		return fmt.Errorf("primary migration failed: %w", err)
	}
	if c.secondary != nil {
		if err := c.secondary.Migrate(ctx); err != nil {
			return fmt.Errorf("secondary migration failed: %w", err)
		}
	}
	return nil
}

// Close closes both stores
func (c *CQRSStore) Close() error {
	var primaryErr, secondaryErr error

	primaryErr = c.primary.Close()
	if c.secondary != nil {
		secondaryErr = c.secondary.Close()
	}

	if primaryErr != nil {
		return primaryErr
	}
	return secondaryErr
}

// Workspace operations
func (c *CQRSStore) CreateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.CreateWorkspace(ctx, workspace)
}

func (c *CQRSStore) GetWorkspace(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error) {
	return c.getReadStore().GetWorkspace(ctx, id)
}

func (c *CQRSStore) UpdateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.UpdateWorkspace(ctx, workspace)
}

func (c *CQRSStore) DeleteWorkspace(ctx context.Context, id models.WorkspaceID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.DeleteWorkspace(ctx, id)
}

func (c *CQRSStore) ListWorkspaces(ctx context.Context, ownerID models.UserID) ([]*models.Workspace, error) {
	return c.getReadStore().ListWorkspaces(ctx, ownerID)
}

// Page operations
func (c *CQRSStore) CreatePage(ctx context.Context, page *models.Page) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.CreatePage(ctx, page)
}

func (c *CQRSStore) GetPage(ctx context.Context, id models.PageID) (*models.Page, error) {
	return c.getReadStore().GetPage(ctx, id)
}

func (c *CQRSStore) UpdatePage(ctx context.Context, page *models.Page) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	// Debug: Log which store we're writing to in reversed mode
	if c.mode == ModeReversed {
		fmt.Printf("CQRS UpdatePage in ModeReversed: writing to secondary (SurrealDB), page.Title=%s\n", page.Title)
	}
	return store.UpdatePage(ctx, page)
}

func (c *CQRSStore) DeletePage(ctx context.Context, id models.PageID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.DeletePage(ctx, id)
}

func (c *CQRSStore) ListPages(ctx context.Context, workspaceID models.WorkspaceID) ([]*models.Page, error) {
	return c.getReadStore().ListPages(ctx, workspaceID)
}

func (c *CQRSStore) ListChildPages(ctx context.Context, parentPageID models.PageID) ([]*models.Page, error) {
	return c.getReadStore().ListChildPages(ctx, parentPageID)
}

// Block operations
func (c *CQRSStore) CreateBlock(ctx context.Context, block *models.Block) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.CreateBlock(ctx, block)
}

func (c *CQRSStore) GetBlock(ctx context.Context, id models.BlockID) (*models.Block, error) {
	return c.getReadStore().GetBlock(ctx, id)
}

func (c *CQRSStore) UpdateBlock(ctx context.Context, block *models.Block) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.UpdateBlock(ctx, block)
}

func (c *CQRSStore) DeleteBlock(ctx context.Context, id models.BlockID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.DeleteBlock(ctx, id)
}

func (c *CQRSStore) ListBlocks(ctx context.Context, pageID models.PageID) ([]*models.Block, error) {
	return c.getReadStore().ListBlocks(ctx, pageID)
}

func (c *CQRSStore) ReorderBlocks(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.ReorderBlocks(ctx, pageID, blockIDs)
}

// User operations
func (c *CQRSStore) CreateUser(ctx context.Context, user *models.User) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.CreateUser(ctx, user)
}

func (c *CQRSStore) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	return c.getReadStore().GetUser(ctx, id)
}

func (c *CQRSStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return c.getReadStore().GetUserByEmail(ctx, email)
}

func (c *CQRSStore) UpdateUser(ctx context.Context, user *models.User) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.UpdateUser(ctx, user)
}

func (c *CQRSStore) DeleteUser(ctx context.Context, id models.UserID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.DeleteUser(ctx, id)
}

// Permission operations
func (c *CQRSStore) CreatePermission(ctx context.Context, permission *models.Permission) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.CreatePermission(ctx, permission)
}

func (c *CQRSStore) GetPermissions(ctx context.Context, resourceType models.ResourceType, resourceID models.ResourceID) ([]*models.Permission, error) {
	return c.getReadStore().GetPermissions(ctx, resourceType, resourceID)
}

func (c *CQRSStore) GetUserPermissions(ctx context.Context, userID models.UserID) ([]*models.Permission, error) {
	return c.getReadStore().GetUserPermissions(ctx, userID)
}

func (c *CQRSStore) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.UpdatePermission(ctx, permission)
}

func (c *CQRSStore) DeletePermission(ctx context.Context, id models.PermissionID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.DeletePermission(ctx, id)
}

func (c *CQRSStore) CheckPermission(ctx context.Context, userID models.UserID, resourceType models.ResourceType, resourceID models.ResourceID, level models.PermissionLevel) (bool, error) {
	return c.getReadStore().CheckPermission(ctx, userID, resourceType, resourceID, level)
}

// Comment operations
func (c *CQRSStore) CreateComment(ctx context.Context, comment *models.Comment) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.CreateComment(ctx, comment)
}

func (c *CQRSStore) GetComment(ctx context.Context, id models.CommentID) (*models.Comment, error) {
	return c.getReadStore().GetComment(ctx, id)
}

func (c *CQRSStore) ListComments(ctx context.Context, blockID models.BlockID) ([]*models.Comment, error) {
	return c.getReadStore().ListComments(ctx, blockID)
}

func (c *CQRSStore) UpdateComment(ctx context.Context, comment *models.Comment) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.UpdateComment(ctx, comment)
}

func (c *CQRSStore) DeleteComment(ctx context.Context, id models.CommentID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.DeleteComment(ctx, id)
}

func (c *CQRSStore) ResolveComment(ctx context.Context, id models.CommentID) error {
	store, err := c.getWriteStore()
	if err != nil {
		return err
	}
	return store.ResolveComment(ctx, id)
}

// Timestamp-based catch-up methods
func (c *CQRSStore) ListModifiedWorkspaceIDs(ctx context.Context, since, until time.Time) ([]models.WorkspaceID, error) {
	return c.primary.ListModifiedWorkspaceIDs(ctx, since, until)
}

func (c *CQRSStore) ListModifiedPageIDs(ctx context.Context, since, until time.Time) ([]models.PageID, error) {
	return c.primary.ListModifiedPageIDs(ctx, since, until)
}

func (c *CQRSStore) ListModifiedBlockIDs(ctx context.Context, since, until time.Time) ([]models.BlockID, error) {
	return c.primary.ListModifiedBlockIDs(ctx, since, until)
}

func (c *CQRSStore) ListModifiedUserIDs(ctx context.Context, since, until time.Time) ([]models.UserID, error) {
	return c.primary.ListModifiedUserIDs(ctx, since, until)
}

func (c *CQRSStore) ListModifiedCommentIDs(ctx context.Context, since, until time.Time) ([]models.CommentID, error) {
	return c.primary.ListModifiedCommentIDs(ctx, since, until)
}

func (c *CQRSStore) ListModifiedPermissionIDs(ctx context.Context, since, until time.Time) ([]models.PermissionID, error) {
	return c.primary.ListModifiedPermissionIDs(ctx, since, until)
}

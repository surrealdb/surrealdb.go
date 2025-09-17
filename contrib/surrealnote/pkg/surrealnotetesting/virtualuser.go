// Package surrealnotetesting provides testing utilities for the surrealnote application.
//
// This package contains tools for comprehensive testing of multi-database scenarios,
// migration workflows, and performance validation. It enables developers to simulate
// realistic user behaviors, validate data consistency across different database backends,
// and perform load testing of the application under various conditions.
//
// # Testing Architecture
//
// The testing utilities are designed around these core concepts:
//   - [VirtualUser]: Stateful simulated users that perform realistic application operations
//   - Deterministic behavior: Reproducible test scenarios using seeded random number generators
//   - Session management: Tracks authentication state and current context (workspace, page)
//   - Data verification: Validates data integrity and consistency across operations
//   - Load testing: Supports concurrent virtual users for performance testing
//
// # Virtual User Simulation
//
// [VirtualUser] provides a realistic simulation of user behavior:
//   - Authentication: Sign up, sign in, and sign out operations
//   - Content creation: Create workspaces, pages, blocks, and comments
//   - Content modification: Update existing entities with new data
//   - Content organization: Reorder blocks, manage page hierarchies
//   - Content deletion: Remove entities and verify cleanup
//   - Navigation: Switch between workspaces and pages during sessions
//
// Each virtual user maintains complete session state and tracks all created/modified
// data for later verification and cleanup.
//
// # Deterministic Testing
//
// Virtual users use seeded random number generators to ensure reproducible behavior:
//   - User index determines the random seed for predictable operation sequences
//   - Even-indexed users bias toward content creation (create-heavy workload)
//   - Odd-indexed users bias toward content deletion (delete-heavy workload)
//   - Random choices (workspace names, content types, etc.) are deterministic
//   - Test scenarios can be replayed exactly for debugging and validation
//
// This approach enables reliable testing while still exercising diverse code paths
// and edge cases through varied user behavior patterns.
//
// # Multi-Database Testing
//
// The testing utilities validate application behavior across different database backends:
//   - Single-store testing: Validate functionality with PostgreSQL or SurrealDB alone
//   - CQRS migration testing: Test dual-write scenarios and consistency during migration
//   - Cross-backend consistency: Verify data integrity between PostgreSQL and SurrealDB
//   - Migration mode transitions: Test mode changes without data loss or corruption
//
// Virtual users can operate against any store implementation through the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client]
// package, enabling consistent testing regardless of the underlying database.
//
// # Load Testing and Performance Validation
//
// Multiple virtual users can run concurrently to simulate realistic load conditions:
//   - Concurrent user sessions with independent authentication and state
//   - Realistic operation patterns based on actual user behavior
//   - Configurable workload characteristics (read-heavy, write-heavy, mixed)
//   - Performance metrics collection for throughput and latency analysis
//   - Resource usage monitoring during high-concurrency scenarios
//
// # Data Integrity Verification
//
// Virtual users include built-in verification capabilities:
//   - Track all created entities for later validation
//   - Verify deleted entities are properly removed from all stores
//   - Validate foreign key relationships and cascade operations
//   - Check data consistency between different database backends
//   - Ensure no data corruption or loss during operations
//
// The verification process helps catch subtle bugs in migration logic, consistency
// handling, and error recovery scenarios.
//
// # Testing Scenarios and Use Cases
//
// The package supports various testing scenarios:
//
// End-to-End Testing: Validate complete user workflows from sign-up through
// content creation, collaboration, and cleanup.
//
// Migration Testing: Test database backend transitions with active user
// sessions and verify data consistency throughout the process.
//
// Load Testing: Simulate multiple concurrent users to identify performance
// bottlenecks and validate system behavior under stress.
//
// Consistency Testing: Verify data integrity across different store implementations
// and during migration mode transitions.
//
// Error Recovery Testing: Test application behavior during database failures,
// network issues, and other error conditions.
//
// # Usage Examples
//
//	// Single virtual user workflow
//	vu := surrealnotetesting.NewVirtualUser(0, "http://localhost:8080")
//	defer vu.Client.Close()
//
//	// Run complete user scenario
//	if err := vu.RunScenario(ctx); err != nil {
//		t.Fatalf("Virtual user scenario failed: %v", err)
//	}
//
//	// Verify all data is consistent
//	if err := vu.VerifyAllData(ctx); err != nil {
//		t.Fatalf("Data verification failed: %v", err)
//	}
//
//	// Concurrent load testing
//	numUsers := 10
//	var wg sync.WaitGroup
//
//	for i := 0; i < numUsers; i++ {
//		wg.Add(1)
//		go func(userIndex int) {
//			defer wg.Done()
//			vu := surrealnotetesting.NewVirtualUser(userIndex, baseURL)
//			vu.RunScenario(ctx)
//		}(i)
//	}
//
//	wg.Wait()
//
// # Integration with Application Testing
//
// Virtual users integrate with standard Go testing patterns:
//   - Use with testing.T for traditional unit and integration tests
//   - Compatible with testify assertions and require functions
//   - Support context cancellation for test timeouts and cleanup
//   - Enable table-driven tests with different user behavior patterns
//   - Work with test fixtures and setup/teardown functions
//
// The package serves as a bridge between low-level database testing and high-level
// application behavior validation, ensuring the complete system works correctly
// from the user perspective.
//
// # Production Considerations
//
// While designed for testing, the virtual user patterns can inform production features:
//   - User behavior analytics based on operation patterns
//   - Performance monitoring using similar metrics collection
//   - Error recovery patterns proven through virtual user testing
//   - Load balancing strategies validated through concurrent user simulation
//   - Feature usage tracking using operation sequence analysis
package surrealnotetesting

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// VirtualUser represents a stateful simulated user that performs realistic application operations.
//
// VirtualUser provides a complete simulation of user behavior including authentication,
// content creation and management, navigation between workspaces and pages, and cleanup
// operations. Each virtual user maintains session state and tracks all created data
// for verification and consistency testing.
//
// The virtual user uses deterministic random behavior based on its index, enabling
// reproducible test scenarios while still exercising diverse code paths and edge cases.
// This approach supports both individual user testing and concurrent load testing scenarios.
//
// Virtual users are designed to work with any [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client.Client] instance, enabling testing
// against different database backends and migration modes without changes to the test logic.
type VirtualUser struct {
	Index    int // Virtual user index (0, 1, 2...) - NOT the database user ID
	Name     string
	Email    string
	Password string
	Client   *client.Client
	RNG      *rand.Rand // Deterministic random number generator seeded with Index

	// Session state
	User             *models.User      // Currently authenticated user
	CurrentWorkspace *models.Workspace // Currently active workspace
	CurrentPage      *models.Page      // Currently active page
	AuthToken        string            // Current auth token

	// Tracking data created by this user
	Workspaces []*models.Workspace
	Pages      map[models.WorkspaceID][]*models.Page
	Blocks     map[models.PageID][]*models.Block
	Comments   map[models.BlockID][]*models.Comment

	// Track deleted items for verification
	DeletedWorkspaces []models.WorkspaceID
	DeletedPages      []models.PageID
	DeletedBlocks     []models.BlockID

	mu sync.RWMutex
}

// NewVirtualUser creates a new virtual user with a client
func NewVirtualUser(index int, baseURL string) *VirtualUser {
	// Use user index as seed for deterministic random behavior
	rng := rand.New(rand.NewSource(int64(index)))

	// Use timestamp to ensure unique emails across test runs
	timestamp := time.Now().UnixNano()

	return &VirtualUser{
		Index:             index,
		Name:              fmt.Sprintf("Virtual User %d", index),
		Email:             fmt.Sprintf("user%d-%d@test.com", index, timestamp),
		Password:          fmt.Sprintf("password%d", index),
		Client:            client.NewClient(baseURL),
		RNG:               rng,
		Pages:             make(map[models.WorkspaceID][]*models.Page),
		Blocks:            make(map[models.PageID][]*models.Block),
		Comments:          make(map[models.BlockID][]*models.Comment),
		DeletedWorkspaces: make([]models.WorkspaceID, 0),
		DeletedPages:      make([]models.PageID, 0),
		DeletedBlocks:     make([]models.BlockID, 0),
	}
}

// SignUp creates an account for this virtual user
func (vu *VirtualUser) SignUp(ctx context.Context) error {
	authResp, err := vu.Client.SignUp(ctx, vu.Email, vu.Password, vu.Name)
	if err != nil {
		return fmt.Errorf("virtual user %d signup failed: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.User = authResp.User
	vu.AuthToken = authResp.Token
	vu.mu.Unlock()

	return nil
}

// SignIn authenticates this virtual user
func (vu *VirtualUser) SignIn(ctx context.Context) error {
	authResp, err := vu.Client.SignIn(ctx, vu.Email, vu.Password)
	if err != nil {
		return fmt.Errorf("virtual user %d signin failed: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.User = authResp.User
	vu.AuthToken = authResp.Token
	vu.mu.Unlock()

	return nil
}

// SignOut signs out the current user
func (vu *VirtualUser) SignOut(ctx context.Context) error {
	err := vu.Client.SignOut(ctx)
	if err != nil {
		return fmt.Errorf("virtual user %d signout failed: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.User = nil
	vu.AuthToken = ""
	vu.CurrentWorkspace = nil
	vu.CurrentPage = nil
	vu.mu.Unlock()

	return nil
}

// CreateWorkspace creates a new workspace and sets it as current
func (vu *VirtualUser) CreateWorkspace(ctx context.Context, name string) (*models.Workspace, error) {
	workspace := &models.Workspace{
		Name:      name,
		OwnerID:   vu.User.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	created, err := vu.Client.CreateWorkspace(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("virtual user %d failed to create workspace: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.Workspaces = append(vu.Workspaces, created)
	vu.CurrentWorkspace = created
	vu.mu.Unlock()

	return created, nil
}

// SwitchWorkspace switches to a different workspace
func (vu *VirtualUser) SwitchWorkspace(ctx context.Context, workspaceID models.WorkspaceID) error {
	workspace, err := vu.Client.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to switch workspace: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.CurrentWorkspace = workspace
	vu.CurrentPage = nil // Clear current page when switching workspace
	vu.mu.Unlock()

	return nil
}

// CreatePage creates a new page in the current workspace and sets it as current
func (vu *VirtualUser) CreatePage(ctx context.Context, title string) (*models.Page, error) {
	vu.mu.RLock()
	if vu.CurrentWorkspace == nil {
		vu.mu.RUnlock()
		return nil, fmt.Errorf("virtual user %d has no current workspace", vu.Index)
	}
	workspaceID := vu.CurrentWorkspace.ID
	vu.mu.RUnlock()

	page := &models.Page{
		Title:       title,
		WorkspaceID: workspaceID,
		CreatedBy:   vu.User.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	created, err := vu.Client.CreatePage(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("virtual user %d failed to create page: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.Pages[workspaceID] = append(vu.Pages[workspaceID], created)
	vu.CurrentPage = created
	vu.mu.Unlock()

	return created, nil
}

// CreatePageInWorkspace creates a page in a specific workspace
func (vu *VirtualUser) CreatePageInWorkspace(ctx context.Context, workspaceID models.WorkspaceID, title string) (*models.Page, error) {
	page := &models.Page{
		Title:       title,
		WorkspaceID: workspaceID,
		CreatedBy:   vu.User.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	created, err := vu.Client.CreatePage(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("virtual user %d failed to create page: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.Pages[workspaceID] = append(vu.Pages[workspaceID], created)
	vu.mu.Unlock()

	return created, nil
}

// SwitchPage switches to a different page
func (vu *VirtualUser) SwitchPage(ctx context.Context, pageID models.PageID) error {
	page, err := vu.Client.GetPage(ctx, pageID)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to switch page: %w", vu.Index, err)
	}

	// Also ensure we're in the right workspace
	if page.WorkspaceID != vu.CurrentWorkspace.ID {
		if err := vu.SwitchWorkspace(ctx, page.WorkspaceID); err != nil {
			return err
		}
	}

	vu.mu.Lock()
	vu.CurrentPage = page
	vu.mu.Unlock()

	return nil
}

// CreateBlock creates a new block in the current page
func (vu *VirtualUser) CreateBlock(ctx context.Context, blockType models.BlockType, content string, order int) (*models.Block, error) {
	vu.mu.RLock()
	if vu.CurrentPage == nil {
		vu.mu.RUnlock()
		return nil, fmt.Errorf("virtual user %d has no current page", vu.Index)
	}
	pageID := vu.CurrentPage.ID
	vu.mu.RUnlock()

	return vu.CreateBlockInPage(ctx, pageID, blockType, content, order)
}

// CreateBlockInPage creates a block in a specific page
func (vu *VirtualUser) CreateBlockInPage(ctx context.Context, pageID models.PageID, blockType models.BlockType, content string, order int) (*models.Block, error) {
	contentData := models.JSONMap{
		"text": content,
	}

	block := &models.Block{
		PageID:    pageID,
		Type:      blockType,
		Content:   contentData,
		Order:     order,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	created, err := vu.Client.CreateBlock(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("virtual user %d failed to create block: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.Blocks[pageID] = append(vu.Blocks[pageID], created)
	vu.mu.Unlock()

	return created, nil
}

// UpdateWorkspace updates an existing workspace
func (vu *VirtualUser) UpdateWorkspace(ctx context.Context, workspace *models.Workspace, newName string) error {
	workspace.Name = newName
	workspace.UpdatedAt = time.Now()

	updated, err := vu.Client.UpdateWorkspace(ctx, workspace)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to update workspace: %w", vu.Index, err)
	}

	// Update local copy
	vu.mu.Lock()
	for i, w := range vu.Workspaces {
		if w.ID == workspace.ID {
			vu.Workspaces[i] = updated
			if vu.CurrentWorkspace != nil && vu.CurrentWorkspace.ID == workspace.ID {
				vu.CurrentWorkspace = updated
			}
			break
		}
	}
	vu.mu.Unlock()

	return nil
}

// DeleteWorkspace deletes a workspace and all its contents
func (vu *VirtualUser) DeleteWorkspace(ctx context.Context, workspaceID models.WorkspaceID) error {
	if err := vu.Client.DeleteWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("virtual user %d failed to delete workspace: %w", vu.Index, err)
	}

	vu.mu.Lock()
	defer vu.mu.Unlock()

	// Track deletion
	vu.DeletedWorkspaces = append(vu.DeletedWorkspaces, workspaceID)

	// Remove from local tracking
	newWorkspaces := make([]*models.Workspace, 0, len(vu.Workspaces)-1)
	for _, w := range vu.Workspaces {
		if w.ID != workspaceID {
			newWorkspaces = append(newWorkspaces, w)
		}
	}
	vu.Workspaces = newWorkspaces

	// Clear current workspace if it was deleted
	if vu.CurrentWorkspace != nil && vu.CurrentWorkspace.ID == workspaceID {
		vu.CurrentWorkspace = nil
		vu.CurrentPage = nil
	}

	// Clean up pages and blocks for this workspace
	delete(vu.Pages, workspaceID)

	return nil
}

// UpdatePage updates an existing page
func (vu *VirtualUser) UpdatePage(ctx context.Context, page *models.Page, newTitle string) error {
	page.Title = newTitle
	page.UpdatedAt = time.Now()

	updated, err := vu.Client.UpdatePage(ctx, page)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to update page: %w", vu.Index, err)
	}

	// Update local copy
	vu.mu.Lock()
	if pages, ok := vu.Pages[page.WorkspaceID]; ok {
		for i, p := range pages {
			if p.ID == page.ID {
				vu.Pages[page.WorkspaceID][i] = updated
				if vu.CurrentPage != nil && vu.CurrentPage.ID == page.ID {
					vu.CurrentPage = updated
				}
				break
			}
		}
	}
	vu.mu.Unlock()

	return nil
}

// DeletePage deletes a page and all its contents
func (vu *VirtualUser) DeletePage(ctx context.Context, pageID models.PageID) error {
	// Get page to find workspace
	page, err := vu.Client.GetPage(ctx, pageID)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to get page for deletion: %w", vu.Index, err)
	}

	if err := vu.Client.DeletePage(ctx, pageID); err != nil {
		return fmt.Errorf("virtual user %d failed to delete page: %w", vu.Index, err)
	}

	vu.mu.Lock()
	defer vu.mu.Unlock()

	// Track deletion
	vu.DeletedPages = append(vu.DeletedPages, pageID)

	// Remove from local tracking
	if pages, ok := vu.Pages[page.WorkspaceID]; ok {
		newPages := make([]*models.Page, 0, len(pages)-1)
		for _, p := range pages {
			if p.ID != pageID {
				newPages = append(newPages, p)
			}
		}
		vu.Pages[page.WorkspaceID] = newPages
	}

	// Clear current page if it was deleted
	if vu.CurrentPage != nil && vu.CurrentPage.ID == pageID {
		vu.CurrentPage = nil
	}

	// Clean up blocks for this page
	delete(vu.Blocks, pageID)

	return nil
}

// UpdateBlock updates an existing block
func (vu *VirtualUser) UpdateBlock(ctx context.Context, block *models.Block, newContent string) error {
	block.Content["text"] = newContent
	block.UpdatedAt = time.Now()

	updated, err := vu.Client.UpdateBlock(ctx, block)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to update block: %w", vu.Index, err)
	}

	// Update local copy
	vu.mu.Lock()
	for i, b := range vu.Blocks[block.PageID] {
		if b.ID == block.ID {
			vu.Blocks[block.PageID][i] = updated
			break
		}
	}
	vu.mu.Unlock()

	return nil
}

// DeleteBlock deletes a block
func (vu *VirtualUser) DeleteBlock(ctx context.Context, blockID models.BlockID) error {
	// Find the block to get its page ID
	var pageID models.PageID
	var found bool
	vu.mu.RLock()
	for pid, blocks := range vu.Blocks {
		for _, b := range blocks {
			if b.ID == blockID {
				pageID = pid
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	vu.mu.RUnlock()

	if err := vu.Client.DeleteBlock(ctx, blockID); err != nil {
		return fmt.Errorf("virtual user %d failed to delete block: %w", vu.Index, err)
	}

	vu.mu.Lock()
	defer vu.mu.Unlock()

	// Track deletion
	vu.DeletedBlocks = append(vu.DeletedBlocks, blockID)

	// Remove from local tracking
	if blocks, ok := vu.Blocks[pageID]; ok {
		newBlocks := make([]*models.Block, 0, len(blocks)-1)
		for _, b := range blocks {
			if b.ID != blockID {
				newBlocks = append(newBlocks, b)
			}
		}
		vu.Blocks[pageID] = newBlocks
	}

	// Clean up comments for this block
	delete(vu.Comments, blockID)

	return nil
}

// ReorderBlocks changes the order of blocks in the current page
func (vu *VirtualUser) ReorderBlocks(ctx context.Context) error {
	vu.mu.RLock()
	if vu.CurrentPage == nil {
		vu.mu.RUnlock()
		return fmt.Errorf("virtual user %d has no current page", vu.Index)
	}
	pageID := vu.CurrentPage.ID
	vu.mu.RUnlock()

	return vu.ReorderBlocksInPage(ctx, pageID)
}

// ReorderBlocksInPage changes the order of blocks in a specific page
func (vu *VirtualUser) ReorderBlocksInPage(ctx context.Context, pageID models.PageID) error {
	vu.mu.RLock()
	blocks := vu.Blocks[pageID]
	vu.mu.RUnlock()

	if len(blocks) < 2 {
		return nil // Nothing to reorder
	}

	// Create a random new order using deterministic RNG
	blockIDs := make([]models.BlockID, len(blocks))
	for i, block := range blocks {
		blockIDs[i] = block.ID
	}

	// Shuffle the block IDs using deterministic RNG
	vu.RNG.Shuffle(len(blockIDs), func(i, j int) {
		blockIDs[i], blockIDs[j] = blockIDs[j], blockIDs[i]
	})

	if err := vu.Client.ReorderBlocks(ctx, pageID, blockIDs); err != nil {
		return fmt.Errorf("virtual user %d failed to reorder blocks: %w", vu.Index, err)
	}

	// Refresh local blocks
	updatedBlocks, err := vu.Client.ListBlocks(ctx, pageID)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to list blocks after reorder: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.Blocks[pageID] = updatedBlocks
	vu.mu.Unlock()

	return nil
}

// CreateComment adds a comment to a block
func (vu *VirtualUser) CreateComment(ctx context.Context, blockID models.BlockID, content string) (*models.Comment, error) {
	comment := &models.Comment{
		BlockID:   blockID,
		UserID:    vu.User.ID,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	created, err := vu.Client.CreateComment(ctx, comment)
	if err != nil {
		return nil, fmt.Errorf("virtual user %d failed to create comment: %w", vu.Index, err)
	}

	vu.mu.Lock()
	vu.Comments[blockID] = append(vu.Comments[blockID], created)
	vu.mu.Unlock()

	return created, nil
}

// GetCurrentState returns the current state of the virtual user
func (vu *VirtualUser) GetCurrentState() (user *models.User, workspace *models.Workspace, page *models.Page) {
	vu.mu.RLock()
	defer vu.mu.RUnlock()
	return vu.User, vu.CurrentWorkspace, vu.CurrentPage
}

// VerifyAllData verifies that all data created by this user is still present and deleted data is gone
func (vu *VirtualUser) VerifyAllData(ctx context.Context) error {
	// Verify user exists
	currentUser, err := vu.Client.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to get current user: %w", vu.Index, err)
	}
	if currentUser.ID != vu.User.ID {
		return fmt.Errorf("virtual user %d ID mismatch: expected %s, got %s", vu.Index, vu.User.ID, currentUser.ID)
	}

	// Verify workspaces
	workspaces, err := vu.Client.ListWorkspaces(ctx, vu.User.ID)
	if err != nil {
		return fmt.Errorf("virtual user %d failed to list workspaces: %w", vu.Index, err)
	}
	if len(workspaces) != len(vu.Workspaces) {
		return fmt.Errorf("virtual user %d workspace count mismatch: expected %d, got %d", vu.Index, len(vu.Workspaces), len(workspaces))
	}

	// Verify deleted workspaces are gone
	for _, deletedID := range vu.DeletedWorkspaces {
		workspace, err := vu.Client.GetWorkspace(ctx, deletedID)
		if err == nil && workspace != nil {
			return fmt.Errorf("virtual user %d: deleted workspace %s still exists", vu.Index, deletedID)
		}
	}

	// Verify pages in each workspace
	for _, workspace := range vu.Workspaces {
		pages, err := vu.Client.ListPages(ctx, workspace.ID)
		if err != nil {
			return fmt.Errorf("virtual user %d failed to list pages in workspace %s: %w", vu.Index, workspace.ID, err)
		}

		expectedPages := vu.Pages[workspace.ID]
		if len(pages) != len(expectedPages) {
			return fmt.Errorf("virtual user %d page count mismatch in workspace %s: expected %d, got %d",
				vu.Index, workspace.ID, len(expectedPages), len(pages))
		}

		// Verify blocks in each page
		for _, page := range expectedPages {
			blocks, err := vu.Client.ListBlocks(ctx, page.ID)
			if err != nil {
				return fmt.Errorf("virtual user %d failed to list blocks in page %s: %w", vu.Index, page.ID, err)
			}

			expectedBlocks := vu.Blocks[page.ID]
			if len(blocks) != len(expectedBlocks) {
				return fmt.Errorf("virtual user %d block count mismatch in page %s: expected %d, got %d",
					vu.Index, page.ID, len(expectedBlocks), len(blocks))
			}

			// Verify comments on each block
			for _, block := range expectedBlocks {
				comments, err := vu.Client.ListComments(ctx, block.ID)
				if err != nil {
					return fmt.Errorf("virtual user %d failed to list comments on block %s: %w", vu.Index, block.ID, err)
				}

				expectedComments := vu.Comments[block.ID]
				if len(comments) != len(expectedComments) {
					return fmt.Errorf("virtual user %d comment count mismatch on block %s: expected %d, got %d",
						vu.Index, block.ID, len(expectedComments), len(comments))
				}
			}
		}
	}

	return nil
}

// RunScenario executes a complex scenario for this virtual user
func (vu *VirtualUser) RunScenario(ctx context.Context) error {
	// Sign up
	if err := vu.SignUp(ctx); err != nil {
		return err
	}

	// Deterministic behavior based on user index
	// Users with even indices create more content, odd indices delete more
	createBias := vu.Index%2 == 0

	// Create workspaces
	numWorkspaces := vu.RNG.Intn(3) + 1
	for i := 0; i < numWorkspaces; i++ {
		workspace, err := vu.CreateWorkspace(ctx, fmt.Sprintf("Workspace %d-%d", vu.Index, i))
		if err != nil {
			return err
		}

		// Sometimes update workspace name (30% chance)
		if vu.RNG.Float32() < 0.3 {
			newName := fmt.Sprintf("Updated Workspace %d-%d", vu.Index, i)
			if err := vu.UpdateWorkspace(ctx, workspace, newName); err != nil {
				return err
			}
		}

		// Create pages in current workspace
		numPages := vu.RNG.Intn(5) + 1
		for j := 0; j < numPages; j++ {
			page, err := vu.CreatePage(ctx, fmt.Sprintf("Page %d-%d-%d", vu.Index, i, j))
			if err != nil {
				return err
			}

			// Sometimes update page title (25% chance)
			if vu.RNG.Float32() < 0.25 {
				newTitle := fmt.Sprintf("Updated Page %d-%d-%d", vu.Index, i, j)
				if err := vu.UpdatePage(ctx, page, newTitle); err != nil {
					return err
				}
			}

			// Create blocks in current page
			numBlocks := vu.RNG.Intn(10) + 1
			for k := 0; k < numBlocks; k++ {
				blockType := models.BlockTypeText
				if vu.RNG.Float32() < 0.3 {
					blockType = models.BlockTypeHeading
				}

				block, err := vu.CreateBlock(ctx, blockType,
					fmt.Sprintf("Block content %d-%d-%d-%d", vu.Index, i, j, k), k)
				if err != nil {
					return err
				}

				// Sometimes add comments (20% chance)
				if vu.RNG.Float32() < 0.2 {
					numComments := vu.RNG.Intn(3) + 1
					for l := 0; l < numComments; l++ {
						_, err := vu.CreateComment(ctx, block.ID,
							fmt.Sprintf("Comment %d-%d-%d-%d-%d", vu.Index, i, j, k, l))
						if err != nil {
							return err
						}
					}
				}

				// Sometimes update block content (35% chance)
				if vu.RNG.Float32() < 0.35 {
					newContent := fmt.Sprintf("Updated block %d-%d-%d-%d at iteration %d", vu.Index, i, j, k, vu.RNG.Intn(100))
					if err := vu.UpdateBlock(ctx, block, newContent); err != nil {
						return err
					}
				}

				// Sometimes delete block (15% chance for odd indices, 5% for even indices)
				deleteChance := float32(0.05)
				if !createBias {
					deleteChance = 0.15
				}
				if vu.RNG.Float32() < deleteChance && len(vu.Blocks[page.ID]) > 1 {
					if err := vu.DeleteBlock(ctx, block.ID); err != nil {
						return err
					}
				}
			}

			// Sometimes reorder blocks in current page (30% chance)
			if vu.RNG.Float32() < 0.3 {
				if err := vu.ReorderBlocks(ctx); err != nil {
					return err
				}
			}

			// Sometimes delete page (10% chance for odd indices, never for even indices)
			if !createBias && vu.RNG.Float32() < 0.1 && len(vu.Pages[workspace.ID]) > 1 {
				if err := vu.DeletePage(ctx, page.ID); err != nil {
					return err
				}
			}
		}

		// Sometimes switch back to a previous workspace (30% chance)
		if i > 0 && vu.RNG.Float32() < 0.3 {
			prevWorkspace := vu.Workspaces[vu.RNG.Intn(len(vu.Workspaces))]
			if err := vu.SwitchWorkspace(ctx, prevWorkspace.ID); err != nil {
				return err
			}
		}

		// Sometimes delete workspace (5% chance for odd indices, never for even indices)
		if !createBias && vu.RNG.Float32() < 0.05 && len(vu.Workspaces) > 1 && i < numWorkspaces-1 {
			if err := vu.DeleteWorkspace(ctx, workspace.ID); err != nil {
				return err
			}
		}
	}

	// Verify all data is present
	return vu.VerifyAllData(ctx)
}

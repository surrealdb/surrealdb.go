// Package surrealdb provides SurrealDB implementation of the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] interface using native SurrealQL.
//
// This package demonstrates how to implement the repository pattern with a multi-model
// database using its native query language without ORM abstractions. It serves as a
// reference implementation for schema-flexible backends and showcases eventual consistency
// within individual operations.
//
// # Implementation Strategy
//
// [SurrealStoreCBOR] uses SurrealDB's native capabilities:
//   - Direct SurrealQL query execution without ORM translation layers
//   - Schema-flexible document storage with optional relational constraints
//   - Native support for complex data types (JSON, arrays, nested objects)
//   - Graph database features for traversing entity relationships
//   - Built-in authentication and permission systems
//
// This approach contrasts with the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/postgres.PostgresStore] implementation which
// uses GORM ORM for automatic SQL generation and strict relational schema enforcement.
//
// # CBOR Marshaling Strategy
//
// The implementation uses a custom CBOR (Concise Binary Object Representation) codec
// to ensure proper data serialization between Go types and SurrealDB's internal format:
//
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.User] structs marshal directly to SurrealDB records
//   - Typed IDs ([github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.UserID], [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.WorkspaceID], etc.) automatically convert to SurrealDB RecordIDs
//   - time.Time values use SurrealDB's native datetime format
//   - Optional fields and nil values are handled correctly
//   - Complex JSON data in [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.JSONMap] preserves type information
//
// # Design Evolution and CBOR Importance
//
// This package evolved through three iterations to solve SurrealDB-specific challenges:
//
//  1. SimpleSurrealStore: Used default Go JSON marshaling, suffered from time.Time
//     serialization issues and fragile RecordID string concatenation
//  2. SurrealStoreV2: Improved RecordID handling but still had marshaling problems
//     with complex types and couldn't customize formats for SurrealDB
//  3. SurrealStoreCBOR (current): Uses surrealcbor codec for complete control over
//     marshaling, proper typed ID handling, and correct time.Time format
//
// The CBOR approach is essential because SurrealDB internally uses CBOR for data
// storage, and default Go marshaling doesn't produce SurrealDB-compatible formats.
//
// # Data Model and Schema Flexibility
//
// SurrealDB's schema-flexible nature enables different data modeling approaches:
//   - Documents can contain nested structures without predefined schemas
//   - Relationships can be bidirectional graph edges or traditional foreign keys
//   - Records can have different fields even within the same table
//   - Schema constraints can be added incrementally as needed
//
// However, this implementation maintains compatibility with the PostgreSQL schema
// by using consistent field names and relationship patterns across both backends.
//
// # Consistency Model
//
// SurrealDB provides eventual consistency within single query operations:
//   - Individual queries execute atomically within their transaction scope
//   - Cross-query consistency depends on application-level coordination
//   - Distributed operations may have eventual consistency characteristics
//   - Performance trades-off some consistency for flexibility and speed
//
// This differs from PostgreSQL's ACID guarantees but enables higher performance
// for read-heavy workloads and flexible schema evolution.
//
// # Security and Query Safety
//
// The implementation follows strict security practices:
//   - ALWAYS use parameterized queries ($param syntax) to prevent injection attacks
//   - Typed IDs automatically marshal to secure RecordID references
//   - Structs marshal directly with CBOR - no string interpolation required
//   - Never use fmt.Sprintf or string concatenation for user-provided values
//
// Example of secure query patterns:
//
//	// Safe: parameterized query with typed ID
//	query := "SELECT * FROM users WHERE id = $user_id"
//	result, err := db.Query(ctx, query, map[string]any{
//		"user_id": userID, // userID is models.UserID, marshals to RecordID
//	})
//
//	// Unsafe: string interpolation (never do this)
//	query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID)
//
// # Performance Characteristics
//
// SurrealDB operations exhibit these performance characteristics:
//   - Very high throughput for read operations, especially with flexible queries
//   - Good write performance with eventual consistency trade-offs
//   - Excellent performance for graph traversals and nested data access
//   - Horizontal scaling capabilities for large datasets
//   - Efficient handling of semi-structured and evolving data schemas
//
// # CQRS Support
//
// [SurrealStoreCBOR] supports CQRS migration patterns through:
//   - Timestamp-based change tracking compatible with PostgreSQL
//   - ListModified*IDs methods using SurrealQL time range queries
//   - Idempotent operations that handle dual-write scenarios gracefully
//   - Consistent RecordID generation for cross-store synchronization
//
// These features enable [SurrealStoreCBOR] to work as either primary or secondary
// store in [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs.CQRSStore] migration scenarios.
//
// # Usage Example
//
//	store, err := surrealdb.NewSurrealStoreCBOR(
//		"ws://localhost:8000/rpc",
//		"test", "test", "root", "root",
//	)
//	if err != nil {
//		return err
//	}
//	defer store.Close()
//
//	// Initialize schema (minimal for SurrealDB)
//	if err := store.Migrate(ctx); err != nil {
//		return err
//	}
//
//	// Use with application
//	app := surrealnote.NewApp(store, config)
package surrealdb

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// SurrealStoreCBOR implements the Store interface using SurrealDB with proper CBOR handling.
//
// Why CBOR?
// SurrealDB uses CBOR (Concise Binary Object Representation) internally. Using the surrealcbor
// codec ensures that complex types like time.Time, UUID, and custom types are properly
// serialized in a format that SurrealDB expects. Without this, we encountered issues like:
// - time.Time being serialized incorrectly, causing queries to fail
// - RecordID not being properly recognized
// - Optional fields causing unmarshaling errors
type SurrealStoreCBOR struct {
	db       *surrealdb.DB
	ns       string
	database string
}

// NewSurrealStoreCBOR creates a new SurrealDB store with surrealcbor for proper time.Time handling.
//
// Connection Design:
// Unlike simpler approaches using FromEndpointURLString, we manually configure the connection
// to use the surrealcbor codec. This gives us full control over marshaling/unmarshaling,
// which is critical for data integrity between Go types and SurrealDB's CBOR format.
func NewSurrealStoreCBOR(wsURL, namespace, database, username, password string) (store.Store, error) {
	ctx := context.Background()

	// Parse the URL to create connection config
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	conf := connection.NewConfig(u)

	// Critical: Use surrealcbor for proper time.Time and RecordID handling
	// Without this custom codec, time.Time values would be marshaled incorrectly,
	// causing "invalid datetime" errors in SurrealDB
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec

	// Use gorillaws for WebSocket connection (most stable implementation)
	conn := gorillaws.New(conf)

	// Create DB from connection
	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
	}

	// Authenticate if credentials provided
	if username != "" && password != "" {
		if _, err := db.SignIn(ctx, map[string]any{
			"user": username,
			"pass": password,
		}); err != nil {
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	// Use the specified namespace and database
	if err := db.Use(ctx, namespace, database); err != nil {
		return nil, fmt.Errorf("failed to use namespace/database: %w", err)
	}

	return &SurrealStoreCBOR{
		db:       db,
		ns:       namespace,
		database: database,
	}, nil
}

// Migrate performs schema migration for SurrealDB, which is essentially a no-op.
// Unlike traditional SQL databases, SurrealDB is schemaless and creates tables
// automatically when data is first inserted. This design eliminates the need
// for explicit table creation or schema migration commands.
//
// SurrealDB's automatic schema behavior:
//   - Tables are created implicitly when the first record is inserted
//   - Field types are inferred from the data being stored
//   - No need to define columns, constraints, or relationships upfront
//   - Schema evolution happens naturally as new fields are added to documents
//
// While SurrealDB supports optional schema definitions for data validation
// and optimization, this implementation uses the schemaless approach for
// maximum flexibility during the database migration demonstration.
//
// The method returns nil (success) immediately since no schema operations
// are required. This allows the CQRS store to complete migration on both
// PostgreSQL and SurrealDB without errors.
//
// In a production system, you might want to:
//   - Define explicit schemas for data validation
//   - Create indexes for query performance
//   - Set up database-level permissions and access controls
func (s *SurrealStoreCBOR) Migrate(ctx context.Context) error {
	// SurrealDB automatically creates tables when data is inserted
	return nil
}

// Close closes the database connection
func (s *SurrealStoreCBOR) Close() error {
	return s.db.Close(context.Background())
}

// Note: Transaction methods (BeginTx, Commit, Rollback) were removed.
// SurrealDB handles transactions within a single Query RPC call.
// Complex transactional operations should be implemented using SurrealDB's
// native transaction support via the Query method.

// recordIDProvider interface is no longer needed since typed IDs
// now implement RecordID() method directly

// Helper to handle not found errors for surrealcbor store
func handleNotFoundCBOR(err error) error {
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "Expected a single or multiple results but got 0") ||
			strings.Contains(errStr, "cannot unmarshal array into Go value") {
			return nil
		}
	}
	return err
}

// Workspace operations
func (s *SurrealStoreCBOR) CreateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	if workspace.ID.IsZero() {
		workspace.ID = models.NewWorkspaceID()
	}

	// Set timestamps if needed
	if workspace.CreatedAt.IsZero() {
		workspace.CreatedAt = time.Now()
	}
	if workspace.UpdatedAt.IsZero() {
		workspace.UpdatedAt = time.Now()
	}

	// Use models directly - typed IDs handle RecordID marshaling automatically
	// The OwnerID field will be stored as a RecordID thanks to UserID's MarshalCBOR
	_, err := surrealdb.Create[models.Workspace](ctx, s.db, "workspaces", workspace)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create the ownership relationship: user->owns->workspace
	// This graph edge enables efficient traversal queries
	relateQuery := "RELATE $user->owns->$workspace"
	params := map[string]any{
		"user":      workspace.OwnerID.RecordID(),
		"workspace": workspace.ID.RecordID(),
	}
	if _, err := surrealdb.Query[any](ctx, s.db, relateQuery, params); err != nil {
		return fmt.Errorf("failed to create ownership relationship: %w", err)
	}

	return nil
}

func (s *SurrealStoreCBOR) GetWorkspace(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error) {
	rid := id.RecordID()
	workspace, err := surrealdb.Select[models.Workspace](ctx, s.db, rid)
	if err != nil {
		if handleNotFoundCBOR(err) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}
	return workspace, nil
}

func (s *SurrealStoreCBOR) UpdateWorkspace(ctx context.Context, workspace *models.Workspace) error {
	rid := workspace.ID.RecordID()
	workspace.UpdatedAt = time.Now()

	// Update with the model directly - foreign keys marshal to RecordIDs
	_, err := surrealdb.Update[models.Workspace](ctx, s.db, rid, workspace)
	if err != nil {
		return fmt.Errorf("failed to update workspace: %w", err)
	}
	return nil
}

func (s *SurrealStoreCBOR) DeleteWorkspace(ctx context.Context, id models.WorkspaceID) error {
	rid := id.RecordID()
	_, err := surrealdb.Delete[models.Workspace](ctx, s.db, rid)
	return err
}

func (s *SurrealStoreCBOR) ListWorkspaces(ctx context.Context, ownerID models.UserID) ([]*models.Workspace, error) {
	// Use graph traversal: from user, follow owns relationship to workspaces
	// The .* retrieves all fields of the workspace, not just the ID
	query := "SELECT ->owns->workspace.* FROM $user"
	params := map[string]any{
		"user": ownerID.RecordID(),
	}

	// Query returns all workspace fields when using .*
	type Result struct {
		Workspaces []*models.Workspace `json:"->owns->workspace"`
	}
	result, err := surrealdb.Query[[]Result](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	var workspaces []*models.Workspace
	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		workspaces = (*result)[0].Result[0].Workspaces
	}
	return workspaces, nil
}

// Page operations
func (s *SurrealStoreCBOR) CreatePage(ctx context.Context, page *models.Page) error {
	if page.ID.IsZero() {
		page.ID = models.NewPageID()
	}

	// Set timestamps if needed
	if page.CreatedAt.IsZero() {
		page.CreatedAt = time.Now()
	}
	if page.UpdatedAt.IsZero() {
		page.UpdatedAt = time.Now()
	}

	// Use models directly - foreign keys marshal to RecordIDs automatically
	_, err := surrealdb.Create[models.Page](ctx, s.db, "pages", page)
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	// Create relationships:
	// 1. workspace->contains->page
	relateQuery := "RELATE $workspace->contains->$page"
	params := map[string]any{
		"workspace": page.WorkspaceID.RecordID(),
		"page":      page.ID.RecordID(),
	}
	if _, err := surrealdb.Query[any](ctx, s.db, relateQuery, params); err != nil {
		return fmt.Errorf("failed to create workspace-page relationship: %w", err)
	}

	// 2. user->created->page
	relateQuery2 := "RELATE $user->created->$page"
	params2 := map[string]any{
		"user": page.CreatedBy.RecordID(),
		"page": page.ID.RecordID(),
	}
	if _, err := surrealdb.Query[any](ctx, s.db, relateQuery2, params2); err != nil {
		return fmt.Errorf("failed to create user-page relationship: %w", err)
	}

	// 3. If there's a parent page: parent->has_child->page
	if page.ParentPageID != nil {
		relateQuery3 := "RELATE $parent->has_child->$page"
		params3 := map[string]any{
			"parent": page.ParentPageID.RecordID(),
			"page":   page.ID.RecordID(),
		}
		if _, err := surrealdb.Query[any](ctx, s.db, relateQuery3, params3); err != nil {
			return fmt.Errorf("failed to create parent-page relationship: %w", err)
		}
	}

	return nil
}

func (s *SurrealStoreCBOR) GetPage(ctx context.Context, id models.PageID) (*models.Page, error) {
	rid := id.RecordID()
	page, err := surrealdb.Select[models.Page](ctx, s.db, rid)
	if err != nil {
		if handleNotFoundCBOR(err) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get page: %w", err)
	}
	if page != nil {
		log.Printf("SurrealDB GetPage: ID=%s, Title=%s, UpdatedAt=%v", page.ID, page.Title, page.UpdatedAt)
	} else {
		log.Printf("SurrealDB GetPage: ID=%s returned nil", id)
	}
	return page, nil
}

func (s *SurrealStoreCBOR) UpdatePage(ctx context.Context, page *models.Page) error {
	rid := page.ID.RecordID()
	page.UpdatedAt = time.Now()

	log.Printf("SurrealDB UpdatePage: ID=%s, Title=%s, UpdatedAt=%v", page.ID, page.Title, page.UpdatedAt)

	// Pass the struct directly - typed IDs automatically marshal to RecordIDs
	// thanks to their MarshalCBOR implementations
	_, err := surrealdb.Update[models.Page](ctx, s.db, rid, page)
	if err != nil {
		return fmt.Errorf("failed to update page: %w", err)
	}
	log.Printf("SurrealDB UpdatePage successful for ID=%s", page.ID)
	return nil
}

func (s *SurrealStoreCBOR) DeletePage(ctx context.Context, id models.PageID) error {
	rid := id.RecordID()
	_, err := surrealdb.Delete[models.Page](ctx, s.db, rid)
	return err
}

func (s *SurrealStoreCBOR) ListPages(ctx context.Context, workspaceID models.WorkspaceID) ([]*models.Page, error) {
	// Use graph traversal: from workspace, follow contains relationship to pages
	query := "SELECT ->contains->page.* FROM $workspace"
	params := map[string]any{
		"workspace": workspaceID.RecordID(),
	}
	// Query returns all page fields when using .*
	type Result struct {
		Pages []*models.Page `json:"->contains->page"`
	}
	result, err := surrealdb.Query[[]Result](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list pages: %w", err)
	}

	var pages []*models.Page
	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		pages = (*result)[0].Result[0].Pages
	}
	return pages, nil
}

func (s *SurrealStoreCBOR) ListChildPages(ctx context.Context, parentPageID models.PageID) ([]*models.Page, error) {
	// Use graph traversal: from parent page, follow has_child relationship to child pages
	query := "SELECT ->has_child->page.* FROM $parent"
	params := map[string]any{
		"parent": parentPageID.RecordID(),
	}
	type Result struct {
		Pages []*models.Page `json:"->has_child->page"`
	}
	result, err := surrealdb.Query[[]Result](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list child pages: %w", err)
	}

	var pages []*models.Page
	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		pages = (*result)[0].Result[0].Pages
	}
	return pages, nil
}

// Block operations
func (s *SurrealStoreCBOR) CreateBlock(ctx context.Context, block *models.Block) error {
	if block.ID.IsZero() {
		block.ID = models.NewBlockID()
	}

	// Set timestamps if needed
	if block.CreatedAt.IsZero() {
		block.CreatedAt = time.Now()
	}
	if block.UpdatedAt.IsZero() {
		block.UpdatedAt = time.Now()
	}

	// Use models directly - typed IDs handle RecordID marshaling automatically
	_, err := surrealdb.Create[models.Block](ctx, s.db, "blocks", block)
	if err != nil {
		return fmt.Errorf("failed to create block: %w", err)
	}

	// Create relationships:
	// 1. page->has_block->block
	relateQuery := "RELATE $page->has_block->$block SET block_order = $order"
	params := map[string]any{
		"page":  block.PageID.RecordID(),
		"block": block.ID.RecordID(),
		"order": block.Order,
	}
	if _, err := surrealdb.Query[any](ctx, s.db, relateQuery, params); err != nil {
		return fmt.Errorf("failed to create page-block relationship: %w", err)
	}

	// 2. If there's a parent block: parent_block->contains->block
	if block.ParentBlockID != nil {
		relateQuery2 := "RELATE $parent->contains->$block"
		params2 := map[string]any{
			"parent": block.ParentBlockID.RecordID(),
			"block":  block.ID.RecordID(),
		}
		if _, err := surrealdb.Query[any](ctx, s.db, relateQuery2, params2); err != nil {
			return fmt.Errorf("failed to create parent-block relationship: %w", err)
		}
	}

	return nil
}

func (s *SurrealStoreCBOR) GetBlock(ctx context.Context, id models.BlockID) (*models.Block, error) {
	rid := id.RecordID()
	block, err := surrealdb.Select[models.Block](ctx, s.db, rid)
	if err != nil {
		if handleNotFoundCBOR(err) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	return block, nil
}

func (s *SurrealStoreCBOR) UpdateBlock(ctx context.Context, block *models.Block) error {
	rid := block.ID.RecordID()
	block.UpdatedAt = time.Now()

	// Pass the struct directly - typed IDs automatically marshal to RecordIDs
	_, err := surrealdb.Update[models.Block](ctx, s.db, rid, block)
	if err != nil {
		return fmt.Errorf("failed to update block: %w", err)
	}
	return nil
}

func (s *SurrealStoreCBOR) DeleteBlock(ctx context.Context, id models.BlockID) error {
	rid := id.RecordID()
	_, err := surrealdb.Delete[models.Block](ctx, s.db, rid)
	return err
}

func (s *SurrealStoreCBOR) ListBlocks(ctx context.Context, pageID models.PageID) ([]*models.Block, error) {
	// Use graph traversal: from page, follow has_block relationship to blocks
	// Also fetch the order from the relationship edge
	query := "SELECT ->has_block->block.* FROM $page"
	params := map[string]any{
		"page": pageID.RecordID(),
	}
	type Result struct {
		Blocks []*models.Block `json:"->has_block->block"`
	}
	result, err := surrealdb.Query[[]Result](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list blocks: %w", err)
	}

	var blocks []*models.Block
	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		blocks = (*result)[0].Result[0].Blocks
	}
	// Note: The blocks may not be ordered by the 'order' field from the relationship
	// In production, you might want to fetch the relationships separately to get the order
	return blocks, nil
}

func (s *SurrealStoreCBOR) ReorderBlocks(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error {
	// Update the order field in the has_block relationship
	for i, blockID := range blockIDs {
		// Update the order in the relationship edge
		query := "UPDATE has_block SET block_order = $order WHERE in = $page AND out = $block"
		params := map[string]any{
			"page":  pageID.RecordID(),
			"block": blockID.RecordID(),
			"order": i,
		}
		if _, err := surrealdb.Query[any](ctx, s.db, query, params); err != nil {
			return fmt.Errorf("failed to update block order: %w", err)
		}
	}
	return nil
}

// User operations
func (s *SurrealStoreCBOR) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID.IsZero() {
		user.ID = models.NewUserID()
	}

	// Set timestamps if needed
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = time.Now()
	}

	// Use models directly - typed IDs handle RecordID marshaling automatically
	_, err := surrealdb.Create[models.User](ctx, s.db, "users", user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (s *SurrealStoreCBOR) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	rid := id.RecordID()
	user, err := surrealdb.Select[models.User](ctx, s.db, rid)
	if err != nil {
		if handleNotFoundCBOR(err) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (s *SurrealStoreCBOR) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	// Use parameterized query for safety
	query := "SELECT * FROM users WHERE email = $email"
	params := map[string]any{
		"email": email,
	}
	result, err := surrealdb.Query[[]models.User](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		return &(*result)[0].Result[0], nil
	}
	return nil, nil
}

func (s *SurrealStoreCBOR) UpdateUser(ctx context.Context, user *models.User) error {
	rid := user.ID.RecordID()
	user.UpdatedAt = time.Now()

	// Pass the struct directly - no need for intermediate maps
	_, err := surrealdb.Update[models.User](ctx, s.db, rid, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (s *SurrealStoreCBOR) DeleteUser(ctx context.Context, id models.UserID) error {
	rid := id.RecordID()
	_, err := surrealdb.Delete[models.User](ctx, s.db, rid)
	return err
}

// Permission operations
func (s *SurrealStoreCBOR) CreatePermission(ctx context.Context, permission *models.Permission) error {
	if permission.ID.IsZero() {
		permission.ID = models.NewPermissionID()
	}

	rid := permission.ID.RecordID()

	// Pass the struct directly - typed IDs marshal to RecordIDs automatically
	// IMPORTANT: ResourceID and UserID have MarshalCBOR that handle RecordID conversion
	_, err := surrealdb.Create[models.Permission](ctx, s.db, rid, permission)
	if err != nil {
		return fmt.Errorf("failed to create permission: %w", err)
	}

	// ID is already set correctly, just set timestamps if needed
	if permission.CreatedAt.IsZero() {
		permission.CreatedAt = time.Now()
	}
	if permission.UpdatedAt.IsZero() {
		permission.UpdatedAt = time.Now()
	}
	return nil
}

func (s *SurrealStoreCBOR) GetPermissions(ctx context.Context, resourceType models.ResourceType, resourceID models.ResourceID) ([]*models.Permission, error) {
	// SECURITY: Always use parameterized queries to prevent SQL injection
	// Pass typed IDs directly - they marshal to RecordIDs automatically
	query := "SELECT * FROM permissions WHERE resource_type = $resource_type AND resource_id = $resource_id"
	vars := map[string]any{
		"resource_type": string(resourceType),
		"resource_id":   resourceID, // ResourceID marshals to RecordID via MarshalCBOR
	}
	result, err := surrealdb.Query[[]models.Permission](ctx, s.db, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	var permissions []*models.Permission
	if result != nil && len(*result) > 0 {
		for i := range (*result)[0].Result {
			permissions = append(permissions, &(*result)[0].Result[i])
		}
	}
	return permissions, nil
}

func (s *SurrealStoreCBOR) GetUserPermissions(ctx context.Context, userID models.UserID) ([]*models.Permission, error) {
	// SECURITY: Use parameterized queries - typed IDs work directly as parameters
	query := "SELECT * FROM permissions WHERE user_id = $user_id"
	vars := map[string]any{
		"user_id": userID, // UserID marshals to RecordID automatically
	}
	result, err := surrealdb.Query[[]models.Permission](ctx, s.db, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	var permissions []*models.Permission
	if result != nil && len(*result) > 0 {
		for i := range (*result)[0].Result {
			permissions = append(permissions, &(*result)[0].Result[i])
		}
	}
	return permissions, nil
}

func (s *SurrealStoreCBOR) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	rid := permission.ID.RecordID()
	permission.UpdatedAt = time.Now()

	// Pass the struct directly - typed IDs marshal to RecordIDs automatically
	_, err := surrealdb.Update[models.Permission](ctx, s.db, rid, permission)
	if err != nil {
		return fmt.Errorf("failed to update permission: %w", err)
	}
	return nil
}

func (s *SurrealStoreCBOR) DeletePermission(ctx context.Context, id models.PermissionID) error {
	rid := id.RecordID()
	_, err := surrealdb.Delete[models.Permission](ctx, s.db, rid)
	return err
}

func (s *SurrealStoreCBOR) CheckPermission(ctx context.Context, userID models.UserID, resourceType models.ResourceType, resourceID models.ResourceID, level models.PermissionLevel) (bool, error) {
	query := `SELECT * FROM permission
		WHERE user_id = $user_id
		AND resource_type = $resource_type
		AND resource_id = $resource_id
		AND permission_level = $permission_level`
	vars := map[string]any{
		"user_id":          userID,
		"resource_type":    string(resourceType),
		"resource_id":      resourceID,
		"permission_level": string(level),
	}

	result, err := surrealdb.Query[[]models.Permission](ctx, s.db, query, vars)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		return true, nil
	}
	return false, nil
}

// Comment operations
func (s *SurrealStoreCBOR) CreateComment(ctx context.Context, comment *models.Comment) error {
	if comment.ID.IsZero() {
		comment.ID = models.NewCommentID()
	}

	rid := comment.ID.RecordID()

	// Pass the struct directly - typed IDs marshal to RecordIDs automatically
	_, err := surrealdb.Create[models.Comment](ctx, s.db, rid, comment)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	// ID is already set correctly, just set timestamps if needed
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now()
	}
	if comment.UpdatedAt.IsZero() {
		comment.UpdatedAt = time.Now()
	}
	return nil
}

func (s *SurrealStoreCBOR) GetComment(ctx context.Context, id models.CommentID) (*models.Comment, error) {
	rid := id.RecordID()
	comment, err := surrealdb.Select[models.Comment](ctx, s.db, rid)
	if err != nil {
		if handleNotFoundCBOR(err) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}
	return comment, nil
}

func (s *SurrealStoreCBOR) ListComments(ctx context.Context, blockID models.BlockID) ([]*models.Comment, error) {
	// SECURITY: Parameterized query prevents SQL injection
	query := "SELECT * FROM comments WHERE block_id = $block_id ORDER BY created_at"
	vars := map[string]any{
		"block_id": blockID, // BlockID marshals to RecordID automatically
	}
	result, err := surrealdb.Query[[]models.Comment](ctx, s.db, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	var comments []*models.Comment
	if result != nil && len(*result) > 0 {
		for i := range (*result)[0].Result {
			comments = append(comments, &(*result)[0].Result[i])
		}
	}
	return comments, nil
}

func (s *SurrealStoreCBOR) UpdateComment(ctx context.Context, comment *models.Comment) error {
	rid := comment.ID.RecordID()
	comment.UpdatedAt = time.Now()

	// Pass the struct directly - typed IDs marshal to RecordIDs automatically
	_, err := surrealdb.Update[models.Comment](ctx, s.db, rid, comment)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}
	return nil
}

func (s *SurrealStoreCBOR) DeleteComment(ctx context.Context, id models.CommentID) error {
	rid := id.RecordID()
	_, err := surrealdb.Delete[models.Comment](ctx, s.db, rid)
	return err
}

func (s *SurrealStoreCBOR) ResolveComment(ctx context.Context, id models.CommentID) error {
	rid := id.RecordID()
	now := time.Now()
	_, err := surrealdb.Merge[models.Comment](ctx, s.db, rid, map[string]any{
		"resolved_at": now,
	})
	return err
}

// Timestamp-based catch-up methods for CQRS consistency

func (s *SurrealStoreCBOR) ListModifiedWorkspaceIDs(ctx context.Context, since, until time.Time) ([]models.WorkspaceID, error) {
	// Use parameterized query to avoid timezone issues
	query := `SELECT id FROM workspaces
		WHERE (created_at >= $since AND created_at <= $until)
		OR (updated_at >= $since AND updated_at <= $until)`
	params := map[string]any{
		"since": since,
		"until": until,
	}

	result, err := surrealdb.Query[[]struct {
		ID models.WorkspaceID `json:"id"`
	}](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modified workspace IDs: %w", err)
	}

	var ids []models.WorkspaceID
	if result != nil && len(*result) > 0 {
		for _, record := range (*result)[0].Result {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

func (s *SurrealStoreCBOR) ListModifiedPageIDs(ctx context.Context, since, until time.Time) ([]models.PageID, error) {
	query := `SELECT id FROM pages
		WHERE (created_at >= $since AND created_at <= $until)
		OR (updated_at >= $since AND updated_at <= $until)`
	params := map[string]any{
		"since": since,
		"until": until,
	}

	result, err := surrealdb.Query[[]struct {
		ID models.PageID `json:"id"`
	}](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modified page IDs: %w", err)
	}

	var ids []models.PageID
	if result != nil && len(*result) > 0 {
		for _, record := range (*result)[0].Result {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

func (s *SurrealStoreCBOR) ListModifiedBlockIDs(ctx context.Context, since, until time.Time) ([]models.BlockID, error) {
	query := `SELECT id FROM blocks
		WHERE (created_at >= $since AND created_at <= $until)
		OR (updated_at >= $since AND updated_at <= $until)`
	params := map[string]any{
		"since": since,
		"until": until,
	}

	result, err := surrealdb.Query[[]struct {
		ID models.BlockID `json:"id"`
	}](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modified block IDs: %w", err)
	}

	var ids []models.BlockID
	if result != nil && len(*result) > 0 {
		for _, record := range (*result)[0].Result {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

func (s *SurrealStoreCBOR) ListModifiedUserIDs(ctx context.Context, since, until time.Time) ([]models.UserID, error) {
	query := `SELECT id FROM users
		WHERE (created_at >= $since AND created_at <= $until)
		OR (updated_at >= $since AND updated_at <= $until)`
	params := map[string]any{
		"since": since,
		"until": until,
	}

	result, err := surrealdb.Query[[]struct {
		ID models.UserID `json:"id"`
	}](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modified user IDs: %w", err)
	}

	var ids []models.UserID
	if result != nil && len(*result) > 0 {
		for _, record := range (*result)[0].Result {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

func (s *SurrealStoreCBOR) ListModifiedCommentIDs(ctx context.Context, since, until time.Time) ([]models.CommentID, error) {
	query := `SELECT id FROM comments
		WHERE (created_at >= $since AND created_at <= $until)
		OR (updated_at >= $since AND updated_at <= $until)`
	params := map[string]any{
		"since": since,
		"until": until,
	}

	result, err := surrealdb.Query[[]struct {
		ID models.CommentID `json:"id"`
	}](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modified comment IDs: %w", err)
	}

	var ids []models.CommentID
	if result != nil && len(*result) > 0 {
		for _, record := range (*result)[0].Result {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

func (s *SurrealStoreCBOR) ListModifiedPermissionIDs(ctx context.Context, since, until time.Time) ([]models.PermissionID, error) {
	query := `SELECT id FROM permissions
		WHERE (created_at >= $since AND created_at <= $until)
		OR (updated_at >= $since AND updated_at <= $until)`
	params := map[string]any{
		"since": since,
		"until": until,
	}

	result, err := surrealdb.Query[[]struct {
		ID models.PermissionID `json:"id"`
	}](ctx, s.db, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list modified permission IDs: %w", err)
	}

	var ids []models.PermissionID
	if result != nil && len(*result) > 0 {
		for _, record := range (*result)[0].Result {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

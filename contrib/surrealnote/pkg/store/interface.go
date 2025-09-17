// Package store provides the data persistence layer abstraction for the surrealnote application.
//
// This package defines the [Store] interface which enables the application to work with
// different database backends while maintaining a unified API. The design demonstrates
// how to build applications that can migrate between PostgreSQL (with ORM) and SurrealDB
// (without ORM) while preserving data consistency and application functionality.
//
// # Architecture Pattern
//
// The [Store] interface implements the Repository pattern, abstracting data operations
// for a complete note-taking application domain. While this interface is intentionally
// large for demonstration purposes, production systems should consider splitting it
// following the Interface Segregation Principle into smaller, focused interfaces
// like WorkspaceStore, PageStore, BlockStore, etc.
//
// # Implementation Strategies
//
// Three different implementation approaches showcase different database interaction patterns:
//
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/postgres.PostgresStore]: Uses GORM ORM for traditional relational database operations
//     with ACID transactions and immediate consistency guarantees
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/surrealdb.SurrealStoreCBOR]: Uses native SurrealQL without ORM for schema-flexible
//     operations with eventual consistency within single queries
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs.CQRSStore]: Coordinates dual writes between PostgreSQL and SurrealDB for
//     zero-downtime migration with timestamp-based eventual consistency
//
// # Data Model and Relationships
//
// The store interface supports a hierarchical data model for collaborative documents:
//
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Workspace]: Top-level organizational containers owned by users
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Page]: Documents within workspaces with parent-child relationships
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Block]: Individual content elements (text, headings, images) within pages
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.User]: Account entities with authentication and profile information
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Permission]: Access control rules for workspaces and pages with inheritance
//   - [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.Comment]: Collaborative annotations on content blocks
//
// Entity relationships maintain referential integrity across different database backends
// through careful design of foreign key relationships and cascade operations.
//
// # CQRS and Migration Support
//
// The interface includes specialized methods for Command Query Responsibility Segregation
// (CQRS) patterns that enable zero-downtime database migration:
//
//   - Dual-write operations: Write to both PostgreSQL and SurrealDB simultaneously
//   - Timestamp-based change tracking: Identify records modified during migration windows
//   - Eventual consistency: Synchronize data between stores using catch-up operations
//   - Migration mode support: Switch read/write behavior during migration phases
//
// These methods (ListModified*IDs) work with the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs.CQRSStore] implementation to
// maintain data consistency during database backend transitions.
//
// # Consistency Models
//
// Different store implementations provide different consistency guarantees:
//
//   - PostgreSQL: ACID transactions with immediate consistency and strong isolation
//   - SurrealDB: Eventual consistency within individual query operations
//   - CQRS: Eventual consistency with configurable time windows for synchronization
//
// Applications must understand these trade-offs when choosing between single-store
// and CQRS deployment modes.
//
// # Production Considerations
//
// This demonstration interface should be enhanced for production use with:
//   - Connection pooling and resource management
//   - Caching layers (Redis, in-memory) for performance optimization
//   - Repository patterns with Unit of Work for complex transactions
//   - Batch operations for bulk data processing
//   - Retry logic with exponential backoff for transient failures
//   - Circuit breakers for fault tolerance
//   - Metrics and observability for monitoring
//   - Comprehensive error types for different failure scenarios
//
// # Usage Patterns
//
// Store implementations are typically used through dependency injection:
//
//	// Single store mode
//	store, err := postgres.NewPostgresStore(dsn)
//	app := surrealnote.NewApp(store, config)
//
//	// CQRS migration mode
//	primary, _ := postgres.NewPostgresStore(postgresDSN)
//	secondary, _ := surrealdb.NewSurrealStoreCBOR(surrealURL, ns, db, user, pass)
//	cqrsStore := cqrs.NewCQRSStore(primary, secondary, cqrs.ModeDualWrite)
//	app := surrealnote.NewApp(cqrsStore, config)
package store

import (
	"context"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// Store defines the complete data persistence interface for the surrealnote application.
//
// Store abstracts database operations across different backend implementations:
//   - PostgreSQL store: Uses GORM for ORM-based operations with ACID transactions
//   - SurrealDB store: Uses native SurrealQL without ORM for schema-flexible operations
//   - CQRS store: Coordinates dual writes and synchronization between the above stores
//
// The interface supports the complete application domain including workspace management,
// hierarchical document structure, content blocks, user authentication, access control,
// and collaborative features. Each method group serves specific application requirements:
//
// Entity Operations:
// Standard CRUD operations follow consistent patterns across all entity types.
// Create methods expect entities with generated IDs or will auto-generate them.
// Get methods return nil without error for missing entities.
// Update methods perform full entity replacement (not partial updates).
// Delete methods use soft deletion where supported by the entity model.
// List methods return empty slices for no results, never nil.
//
// Error Handling:
// Methods return errors for:
//   - Database connection failures and timeouts
//   - Constraint violations and validation failures
//   - Permission denied (where applicable)
//   - Resource not found (for Update and Delete operations)
//   - Concurrent modification conflicts
//
// Context Handling:
// All methods accept context.Context for cancellation, timeouts, and request tracing.
// Implementations should respect context deadlines and cancellation signals.
// Long-running operations should check context periodically.
//
// Performance Characteristics:
// Individual operations are optimized for single-entity access patterns.
// List operations use appropriate indexes and may require pagination in production.
// Batch operations are not provided in this interface but would improve performance
// for bulk data operations and migrations.
//
// This interface intentionally violates Interface Segregation Principle for demonstration
// purposes. Production systems should split this into smaller, cohesive interfaces.
type Store interface {
	// Workspace Operations
	//
	// Workspaces represent the top-level organizational units in the application,
	// serving as containers for related pages and collaboration spaces. Each workspace
	// is owned by a single user but can be shared with other users through permissions.

	// CreateWorkspace persists a new workspace to the store.
	//
	// The workspace entity should have its Name and OwnerID populated. If the ID field
	// is zero, a new UUID will be generated automatically. CreatedAt and UpdatedAt
	// timestamps are managed by the store implementation.
	//
	// Returns an error if:
	//   - The workspace name is empty or invalid
	//   - The owner user does not exist
	//   - Database constraints are violated (e.g., name uniqueness per owner)
	//   - Database connection or transaction fails
	//
	// This operation is atomic and will either fully succeed or leave no traces.
	CreateWorkspace(ctx context.Context, workspace *models.Workspace) error

	// GetWorkspace retrieves a workspace by its unique identifier.
	//
	// Returns the workspace entity if found, or nil if no workspace exists with the
	// given ID. The returned workspace may include related entities (Owner) depending
	// on the store implementation's eager loading strategy.
	//
	// Returns an error only for database connection issues or query execution problems,
	// not for missing records. Use the nil return value to detect non-existent workspaces.
	//
	// This is a read-only operation suitable for caching and concurrent access.
	GetWorkspace(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error)

	// UpdateWorkspace replaces an existing workspace with the provided entity.
	//
	// The workspace must have a valid ID that matches an existing record. All fields
	// except ID, CreatedAt, and OwnerID can be modified. The UpdatedAt timestamp is
	// automatically updated by the store implementation.
	//
	// Returns an error if:
	//   - No workspace exists with the given ID
	//   - The workspace name violates uniqueness constraints
	//   - Attempting to change immutable fields like OwnerID
	//   - Database connection or transaction fails
	//   - Concurrent modifications conflict (optimistic locking)
	//
	// This operation replaces the entire entity, not just changed fields.
	UpdateWorkspace(ctx context.Context, workspace *models.Workspace) error

	// DeleteWorkspace removes a workspace and all its related data.
	//
	// This operation cascades to delete all pages, blocks, comments, and permissions
	// associated with the workspace. The deletion is permanent and cannot be undone.
	// Some implementations may use soft deletion for audit trails.
	//
	// Returns an error if:
	//   - No workspace exists with the given ID
	//   - The workspace contains pages that cannot be deleted
	//   - Database connection or transaction fails
	//   - Foreign key constraints prevent deletion
	//
	// Warning: This is a destructive operation that affects multiple entity types.
	// Consider implementing authorization checks and user confirmation workflows.
	DeleteWorkspace(ctx context.Context, id models.WorkspaceID) error

	// ListWorkspaces returns all workspaces owned by the specified user.
	//
	// Results are typically ordered by creation time or name, depending on the
	// implementation. The returned workspaces may include related entities (Owner)
	// for display purposes.
	//
	// Returns an empty slice if the user owns no workspaces. Returns an error only
	// for database connection issues, not for users with no workspaces.
	//
	// Performance note: This query is optimized with indexes on OwnerID. Consider
	// adding pagination parameters for users with many workspaces.
	ListWorkspaces(ctx context.Context, ownerID models.UserID) ([]*models.Workspace, error)

	// Page Operations
	//
	// Pages represent documents within workspaces and support hierarchical organization
	// through parent-child relationships. Pages contain blocks (content elements) and
	// serve as the primary unit of content organization in the note-taking application.

	// CreatePage persists a new page to the store.
	//
	// The page entity must have WorkspaceID and CreatedBy populated. Title is required
	// and should be non-empty. ParentPageID can be nil for root-level pages or set to
	// create a child page. Properties field stores additional metadata as JSON.
	//
	// Returns an error if:
	//   - The workspace does not exist or is not accessible
	//   - The parent page does not exist or is in a different workspace
	//   - The creating user lacks write permissions to the workspace
	//   - Title is empty or violates constraints
	//   - Circular parent-child relationships would be created
	//
	// Parent-child relationships must remain within the same workspace for security
	// and organizational consistency.
	CreatePage(ctx context.Context, page *models.Page) error

	// GetPage retrieves a page by its unique identifier.
	//
	// Returns the page entity with basic fields populated. Related entities (Workspace,
	// ParentPage, Creator) may be included depending on implementation. Use separate
	// queries for blocks and child pages as they are not automatically loaded.
	//
	// Returns nil if no page exists with the given ID. This method does not enforce
	// access control - authorization should be handled at the application layer.
	//
	// Performance note: This operation is indexed on the primary key and executes
	// efficiently regardless of workspace size or page hierarchy depth.
	GetPage(ctx context.Context, id models.PageID) (*models.Page, error)

	// UpdatePage replaces an existing page with the provided entity.
	//
	// The page ID must match an existing record. WorkspaceID and CreatedBy are
	// immutable after creation. Title, Icon, CoverImage, ParentPageID, and Properties
	// can be modified. UpdatedAt timestamp is managed automatically.
	//
	// Returns an error if:
	//   - No page exists with the given ID
	//   - Attempting to change immutable fields (WorkspaceID, CreatedBy)
	//   - Moving page to non-existent parent or different workspace
	//   - Creating circular parent-child relationships
	//   - Title becomes empty or violates constraints
	//
	// Parent-child relationship changes are validated to prevent cycles and maintain
	// workspace boundaries. Consider implementing optimistic locking for concurrent edits.
	UpdatePage(ctx context.Context, page *models.Page) error

	// DeletePage removes a page and all its related data.
	//
	// This operation cascades to delete all blocks, comments, and permissions associated
	// with the page. Child pages become orphaned (ParentPageID set to nil) rather than
	// being deleted, preserving content while removing the hierarchical relationship.
	//
	// Returns an error if:
	//   - No page exists with the given ID
	//   - Database connection or transaction fails
	//   - Cascade deletion of related entities fails
	//
	// Child pages are preserved to prevent accidental data loss. Applications should
	// provide UI warnings about orphaned pages and allow users to reorganize them.
	DeletePage(ctx context.Context, id models.PageID) error

	// ListPages returns all pages within the specified workspace.
	//
	// Results include both root-level pages (ParentPageID is nil) and all nested pages
	// within the workspace hierarchy. Pages are typically ordered by creation time,
	// but specific ordering depends on the implementation.
	//
	// Returns an empty slice for workspaces with no pages. This method does not
	// enforce access control - authorization should be handled at the application layer.
	//
	// Performance note: Large workspaces may return significant result sets. Consider
	// implementing pagination, filtering, or lazy loading for production systems.
	ListPages(ctx context.Context, workspaceID models.WorkspaceID) ([]*models.Page, error)

	// ListChildPages returns direct children of the specified parent page.
	//
	// This method supports building hierarchical page trees by fetching one level
	// at a time. Only direct children are returned - grandchildren and deeper
	// descendants require separate queries.
	//
	// Returns an empty slice if the parent page has no children. The parent page
	// itself must exist, but this is not validated by this method.
	//
	// Use this method for lazy loading page hierarchies, building navigation trees,
	// and implementing expandable/collapsible page organization in user interfaces.
	ListChildPages(ctx context.Context, parentPageID models.PageID) ([]*models.Page, error)

	// Block Operations
	//
	// Blocks represent individual content elements within pages (text, headings, lists,
	// images, tables, etc.). Blocks support hierarchical nesting and ordering within
	// pages. The Content field stores type-specific data as flexible JSON.

	// CreateBlock persists a new content block to the store.
	//
	// The block entity must have PageID, Type, and Content populated. Order determines
	// the block's position within the page. ParentBlockID can be set for nested blocks
	// (e.g., list items, nested content). Content structure varies by block type.
	//
	// Block Content field structure by type:
	//   - text/heading: {"text": "string", "formatting": {...}}
	//   - list: {"items": [...], "style": "bullet|numbered"}
	//   - code: {"code": "string", "language": "string"}
	//   - image: {"url": "string", "alt": "string", "caption": "string"}
	//   - table: {"rows": [[...]], "headers": [...]}
	//   - todo: {"text": "string", "checked": boolean}
	//
	// Returns an error if:
	//   - The page does not exist or is not accessible
	//   - Block type is not supported
	//   - Content structure is invalid for the block type
	//   - Parent block does not exist or creates circular reference
	//   - Order conflicts with existing blocks (implementation-specific)
	//
	// Order management varies by implementation - some auto-assign, others require
	// explicit values. Use ReorderBlocks for precise positioning control.
	CreateBlock(ctx context.Context, block *models.Block) error

	// GetBlock retrieves a content block by its unique identifier.
	//
	// Returns the block entity with all fields populated, including the flexible
	// Content field containing type-specific data. Related entities (Page, ParentBlock)
	// may be included depending on eager loading configuration.
	//
	// Returns nil if no block exists with the given ID. This method does not validate
	// page access permissions - authorization should be handled at the application layer.
	//
	// The Content field must be type-asserted based on the Type field for proper
	// handling of type-specific data structures and validation.
	GetBlock(ctx context.Context, id models.BlockID) (*models.Block, error)

	// UpdateBlock replaces an existing content block with the provided entity.
	//
	// The block ID must match an existing record. PageID is immutable after creation
	// to maintain data integrity. Type, Content, Order, and ParentBlockID can be
	// modified. Content structure must be valid for the block type.
	//
	// Returns an error if:
	//   - No block exists with the given ID
	//   - Attempting to change PageID (not allowed)
	//   - Content structure is invalid for the block type
	//   - Parent block creates circular reference or belongs to different page
	//   - Order conflicts cause positioning issues
	//
	// Changing block types may require content transformation to match the new type's
	// expected structure. Consider validating content before type changes.
	UpdateBlock(ctx context.Context, block *models.Block) error

	// DeleteBlock removes a content block and its associated data.
	//
	// This operation cascades to delete any child blocks (nested content) and all
	// comments associated with the block. The deletion permanently removes content
	// and cannot be undone through the store interface.
	//
	// Returns an error if:
	//   - No block exists with the given ID
	//   - Database connection or transaction fails
	//   - Cascade deletion of child blocks or comments fails
	//
	// Child block deletion maintains content integrity by preventing orphaned nested
	// content. Applications should warn users about nested content removal.
	DeleteBlock(ctx context.Context, id models.BlockID) error

	// ListBlocks returns all content blocks within the specified page.
	//
	// Results include both root-level blocks and nested child blocks, typically
	// ordered by the Order field for proper content sequence. Complex hierarchical
	// structures may require client-side tree building from the flat result set.
	//
	// Returns an empty slice for pages with no content blocks. All blocks include
	// their complete Content field for immediate use without additional queries.
	//
	// Performance note: Pages with many blocks may return large result sets due to
	// flexible JSON content. Consider implementing pagination or lazy loading for
	// content-heavy pages in production systems.
	ListBlocks(ctx context.Context, pageID models.PageID) ([]*models.Block, error)

	// ReorderBlocks updates the sequence of blocks within a page.
	//
	// The blockIDs slice defines the new ordering for the specified blocks. All
	// provided block IDs must exist within the same page. Blocks not included in
	// the slice retain their current positions relative to the reordered blocks.
	//
	// This operation is commonly used for drag-and-drop interfaces, content
	// reorganization, and maintaining visual layout consistency across clients.
	//
	// Returns an error if:
	//   - Any block ID does not exist
	//   - Blocks belong to different pages
	//   - The blockIDs slice is empty
	//   - Database transaction fails during bulk update
	//
	// Implementation typically updates Order fields atomically to prevent inconsistent
	// states. Consider implementing optimistic locking for concurrent reordering operations.
	ReorderBlocks(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error

	// User Operations
	//
	// Users represent account entities with authentication and profile information.
	// User management supports account creation, profile updates, and authentication
	// lookups. Email addresses serve as unique identifiers for login purposes.

	// CreateUser persists a new user account to the store.
	//
	// The user entity must have Name and Email populated. Email addresses must be
	// unique across all users and are validated for basic format compliance.
	// AvatarURL is optional and can store profile image references.
	//
	// Returns an error if:
	//   - Email address is empty, malformed, or already exists
	//   - Name is empty or contains invalid characters
	//   - Database constraints are violated
	//   - Database connection or transaction fails
	//
	// This operation is typically part of user registration workflows. Consider
	// implementing additional validation, password handling, and email verification
	// in the application layer before calling this method.
	CreateUser(ctx context.Context, user *models.User) error

	// GetUser retrieves a user account by its unique identifier.
	//
	// Returns the user entity with all profile fields populated if found, or nil
	// if no user exists with the given ID. This method is suitable for internal
	// user lookups and profile display where the user ID is already known.
	//
	// Returns an error only for database connection issues, not for missing users.
	// This method does not include sensitive information like passwords or tokens.
	//
	// Use GetUserByEmail for authentication and login workflows where only the
	// email address is available.
	GetUser(ctx context.Context, id models.UserID) (*models.User, error)

	// GetUserByEmail retrieves a user account by email address.
	//
	// This method is primarily used for authentication workflows, login processes,
	// and user lookup by email identifier. Email comparison is case-insensitive
	// to match common user expectations and reduce login friction.
	//
	// Returns the user entity if found, or nil if no user has the specified email
	// address. Returns an error only for database connection issues, not for
	// missing users or invalid email formats.
	//
	// Performance note: This query is optimized with a unique index on the email
	// field for fast authentication lookups.
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// UpdateUser replaces an existing user account with the provided entity.
	//
	// The user ID must match an existing record. Email addresses must remain unique
	// across all users. Name, Email, and AvatarURL can be modified. Timestamps are
	// managed automatically by the store implementation.
	//
	// Returns an error if:
	//   - No user exists with the given ID
	//   - Email address is already used by another user
	//   - Name is empty or contains invalid characters
	//   - Database connection or transaction fails
	//
	// Email address changes should be carefully handled with proper verification
	// workflows to prevent account takeover and ensure user intent.
	UpdateUser(ctx context.Context, user *models.User) error

	// DeleteUser removes a user account and handles related data cleanup.
	//
	// This operation affects related entities throughout the system. Workspaces
	// owned by the user may be deleted or transferred to other users depending
	// on implementation. Pages, comments, and permissions created by the user
	// are typically preserved but marked as created by a deleted user.
	//
	// Returns an error if:
	//   - No user exists with the given ID
	//   - The user owns workspaces that cannot be automatically handled
	//   - Database connection or transaction fails
	//   - Foreign key constraints prevent deletion
	//
	// Consider implementing soft deletion to preserve data integrity and audit
	// trails. Hard deletion should include comprehensive cleanup of related entities
	// and may require batch operations for users with significant data.
	DeleteUser(ctx context.Context, id models.UserID) error

	// Permission Operations
	//
	// Permissions represent access control rules for workspaces and pages. The system
	// supports hierarchical permissions where workspace permissions apply to all pages
	// within that workspace unless overridden by page-specific permissions.

	// CreatePermission grants a user access to a resource at the specified level.
	//
	// The permission entity must have ResourceType, ResourceID, UserID, and
	// PermissionLevel populated. ResourceType determines which entity the permission
	// applies to (workspace or page). ResourceID must reference an existing resource
	// of the specified type.
	//
	// Permission levels form a hierarchy:
	//   - read: View content and navigate structure
	//   - write: Create, edit, and delete content (includes read)
	//   - admin: Manage permissions and settings (includes write and read)
	//
	// Returns an error if:
	//   - The resource does not exist or is not accessible
	//   - The user does not exist
	//   - Permission level is invalid or not supported
	//   - Duplicate permission exists (same user, resource, and type)
	//   - Database connection or transaction fails
	//
	// Workspace permissions apply to all pages within the workspace unless overridden
	// by page-specific permissions. Higher permission levels include lower levels.
	CreatePermission(ctx context.Context, permission *models.Permission) error

	// GetPermissions returns all permission records for the specified resource.
	//
	// This method retrieves all users who have been explicitly granted access to
	// the resource, along with their permission levels. Results typically include
	// related user information for display in permission management interfaces.
	//
	// Returns an empty slice for resources with no explicit permissions. The resource
	// owner may have implicit permissions not reflected in explicit permission records.
	//
	// Use this method for displaying permission lists, audit trails, and managing
	// resource sharing settings in administrative interfaces.
	GetPermissions(ctx context.Context, resourceType models.ResourceType, resourceID models.ResourceID) ([]*models.Permission, error)

	// GetUserPermissions returns all permissions granted to the specified user.
	//
	// This method provides a comprehensive view of all resources the user can access,
	// including workspaces and pages with their respective permission levels. Results
	// are useful for user dashboards, navigation filtering, and access audit trails.
	//
	// Returns an empty slice for users with no explicit permissions. Users may still
	// have access to resources they own, which are not represented as explicit
	// permission records.
	//
	// Performance note: Users with access to many resources may return large result
	// sets. Consider implementing pagination or filtering for production systems.
	GetUserPermissions(ctx context.Context, userID models.UserID) ([]*models.Permission, error)

	// UpdatePermission modifies an existing permission record.
	//
	// The permission ID must match an existing record. Typically only the
	// PermissionLevel field is modified to upgrade or downgrade user access.
	// ResourceType, ResourceID, and UserID are immutable after creation.
	//
	// Returns an error if:
	//   - No permission exists with the given ID
	//   - Attempting to change immutable fields
	//   - New permission level is invalid
	//   - Database connection or transaction fails
	//
	// Use DeletePermission and CreatePermission to change resource or user assignments.
	// Consider implementing permission change audit logs for security compliance.
	UpdatePermission(ctx context.Context, permission *models.Permission) error

	// DeletePermission revokes a user's access to a resource.
	//
	// This operation removes the explicit permission record, effectively denying
	// the user access to the resource unless they have access through other means
	// (e.g., ownership or workspace-level permissions for page access).
	//
	// Returns an error if:
	//   - No permission exists with the given ID
	//   - Database connection or transaction fails
	//
	// Revoking permissions does not affect data created by the user. Consider the
	// implications of removing access to collaborative content and provide appropriate
	// user notifications.
	DeletePermission(ctx context.Context, id models.PermissionID) error

	// CheckPermission verifies if a user has the required access level for a resource.
	//
	// This method implements the complete permission resolution logic, considering:
	//   - Direct resource permissions (exact resource and type match)
	//   - Inherited workspace permissions (for page resources)
	//   - Resource ownership (owners have implicit admin permissions)
	//   - Permission level hierarchy (admin includes write and read)
	//
	// Returns true if the user has the required permission level or higher.
	// Returns false for insufficient permissions or non-existent resources.
	// Returns an error only for database connection issues.
	//
	// Permission resolution order:
	//   1. Check resource ownership (automatic admin level)
	//   2. Check direct resource permissions
	//   3. For pages, check workspace permissions (inherited)
	//   4. Apply permission level hierarchy rules
	//
	// This method is essential for authorization checks throughout the application
	// and should be called before any operation that modifies or accesses resources.
	CheckPermission(ctx context.Context, userID models.UserID, resourceType models.ResourceType, resourceID models.ResourceID, level models.PermissionLevel) (bool, error)

	// Comment Operations
	//
	// Comments provide collaborative annotation and discussion features for content
	// blocks. Comments support threaded discussions, resolution tracking, and user
	// attribution for collaborative workflows and content review processes.

	// CreateComment adds a new comment to a content block.
	//
	// The comment entity must have BlockID, UserID, and Content populated. Comments
	// are associated with specific blocks to provide contextual discussions about
	// particular content elements rather than entire pages.
	//
	// Returns an error if:
	//   - The block does not exist or is not accessible
	//   - The user does not exist
	//   - Content is empty or violates length constraints
	//   - Database connection or transaction fails
	//
	// Access control should be enforced at the application layer by verifying the
	// user has read access to the page containing the block before allowing comments.
	CreateComment(ctx context.Context, comment *models.Comment) error

	// GetComment retrieves a comment by its unique identifier.
	//
	// Returns the comment entity with all fields populated, including related
	// entities (Block, User) depending on eager loading configuration. The
	// ResolvedAt field indicates if the comment has been marked as resolved.
	//
	// Returns nil if no comment exists with the given ID. This method does not
	// validate access permissions - authorization should be handled at the
	// application layer by checking page access rights.
	//
	// Use this method for displaying individual comments, handling replies,
	// and implementing comment-specific operations like resolution.
	GetComment(ctx context.Context, id models.CommentID) (*models.Comment, error)

	// ListComments returns all comments associated with a specific content block.
	//
	// Results are typically ordered by creation time to show discussion flow.
	// Comments include user information for proper attribution and may include
	// resolution status for workflow management.
	//
	// Returns an empty slice for blocks with no comments. Both resolved and
	// unresolved comments are included - use client-side filtering or additional
	// query parameters to distinguish between them.
	//
	// Use this method for displaying comment threads, building discussion interfaces,
	// and implementing collaborative review workflows.
	ListComments(ctx context.Context, blockID models.BlockID) ([]*models.Comment, error)

	// UpdateComment modifies an existing comment's content.
	//
	// The comment ID must match an existing record. Only the Content field can be
	// modified - BlockID and UserID are immutable to maintain discussion integrity.
	// UpdatedAt timestamp is automatically managed by the store implementation.
	//
	// Returns an error if:
	//   - No comment exists with the given ID
	//   - Attempting to change immutable fields (BlockID, UserID)
	//   - Content becomes empty or violates constraints
	//   - Database connection or transaction fails
	//
	// Comment editing should include appropriate authorization checks to ensure
	// only comment authors or administrators can modify comment content.
	UpdateComment(ctx context.Context, comment *models.Comment) error

	// DeleteComment removes a comment permanently from the discussion.
	//
	// This operation completely removes the comment and cannot be undone. Consider
	// implementing soft deletion to preserve discussion context and audit trails,
	// especially for collaborative environments.
	//
	// Returns an error if:
	//   - No comment exists with the given ID
	//   - Database connection or transaction fails
	//
	// Comment deletion should be carefully controlled with proper authorization.
	// Removing comments from active discussions may disrupt collaborative workflows
	// and should include appropriate user notifications.
	DeleteComment(ctx context.Context, id models.CommentID) error

	// ResolveComment marks a comment as resolved in collaborative workflows.
	//
	// This operation sets the ResolvedAt timestamp to indicate that the comment's
	// concerns, questions, or suggestions have been addressed. Resolved comments
	// may be filtered out of active discussion views while remaining available
	// for audit and reference purposes.
	//
	// Returns an error if:
	//   - No comment exists with the given ID
	//   - The comment is already resolved (implementation-specific)
	//   - Database connection or transaction fails
	//
	// Comment resolution is typically used in content review, editorial workflows,
	// and collaborative editing processes. Consider implementing unresolve functionality
	// for cases where discussions need to be reopened.
	ResolveComment(ctx context.Context, id models.CommentID) error

	// Transaction and Schema Management
	//
	// Note: Traditional transaction methods (BeginTx, Commit, Rollback) are intentionally
	// omitted from this interface. Different store implementations handle consistency differently:
	//
	// Consistency Models by Store Type:
	//   - PostgreSQL: ACID transactions with immediate consistency within single operations
	//   - SurrealDB: Transactions must be composed within single Query RPC calls
	//   - CQRS: Eventual consistency using timestamp-based catch-up synchronization
	//
	// The CQRS store coordinates between PostgreSQL and SurrealDB, handling consistency
	// through dual-write patterns with recovery mechanisms for failed operations.
	// See pkg/store/cqrs/ implementation for detailed consistency guarantees.

	// Migrate initializes or updates the database schema to support the application's data model.
	//
	// This method handles schema migration (structure) rather than data migration (content).
	// It creates or updates tables, indexes, constraints, and other database schema elements
	// required for the surrealnote application to function properly.
	//
	// Implementation behavior by store type:
	//   - PostgreSQL: Uses GORM auto-migration to create/update tables, foreign keys,
	//     indexes, and constraints based on model struct tags
	//   - SurrealDB: Minimal schema setup as SurrealDB is schema-flexible, may create
	//     indexes for performance optimization
	//   - CQRS: Coordinates migration across both underlying stores, ensuring schema
	//     consistency between PostgreSQL and SurrealDB backends
	//
	// Migration Safety:
	// Migrate is idempotent and safe to run multiple times without data loss. It only
	// creates missing schema elements and updates existing ones when necessary. Data
	// is preserved during schema changes when possible.
	//
	// Usage Patterns:
	//   - Call during application startup to ensure schema is current
	//   - Run after model changes in development environments
	//   - Include in deployment scripts for production schema updates
	//   - Execute before running tests to ensure test database schema
	//
	// Returns an error if:
	//   - Database connection fails or times out
	//   - Schema creation/update operations fail
	//   - Insufficient database permissions for schema changes
	//   - Incompatible schema changes conflict with existing data
	//
	// Consider implementing schema versioning and rollback mechanisms for production
	// systems that require more sophisticated migration management.
	Migrate(ctx context.Context) error

	// Close releases database connections and cleans up resources.
	//
	// This method should be called when the store is no longer needed, typically
	// during application shutdown or when switching store implementations. It ensures
	// proper cleanup of database connection pools, prepared statements, and other
	// resources allocated by the store implementation.
	//
	// Implementation behavior:
	//   - PostgreSQL: Closes GORM database connections and connection pools
	//   - SurrealDB: Closes WebSocket connections to SurrealDB instances
	//   - CQRS: Coordinates shutdown of both underlying stores
	//
	// Returns an error if resource cleanup fails, but the store should be considered
	// unusable regardless of the return value. Multiple calls to Close are safe.
	Close() error

	// CQRS Synchronization and Consistency Support
	//
	// The following methods enable Command Query Responsibility Segregation (CQRS) patterns
	// by providing timestamp-based change tracking. These methods are critical for maintaining
	// eventual consistency between different database backends during migration phases and
	// for recovering from partial failures in dual-write scenarios.
	//
	// CQRS Use Cases:
	//   - Zero-downtime migration between PostgreSQL and SurrealDB
	//   - Catch-up synchronization after network partitions or outages
	//   - Data consistency validation between primary and secondary stores
	//   - Audit trails and change tracking for compliance purposes
	//   - Incremental backup and replication workflows
	//
	// Synchronization Strategy:
	// The CQRS store maintains dual writes to both PostgreSQL and SurrealDB. When the
	// secondary store fails, operations continue on the primary store. These methods
	// enable identification and synchronization of changes that were missed by the
	// secondary store during failure periods.
	//
	// Timestamp Semantics:
	//   - since: Inclusive start time (records modified >= this time are included)
	//   - until: Exclusive end time (records modified < this time are included)
	//   - Time comparison uses MAX(CreatedAt, UpdatedAt) for accurate change detection
	//   - Timezone handling maintains consistency across store implementations
	//   - Clock skew between application and database servers is handled gracefully
	//
	// Performance and Scalability:
	//   - All timestamp queries are optimized with compound indexes on (CreatedAt, UpdatedAt)
	//   - Large time ranges may return substantial result sets requiring pagination
	//   - Query performance scales with time range size, not total table size
	//   - Consider implementing batch processing for large synchronization operations
	//   - Network latency dominates execution time for distributed deployments
	//
	// Error Handling:
	// These methods return errors only for database connectivity issues or query failures,
	// never for empty result sets. Empty slices indicate no changes in the time range,
	// which is a valid result for synchronization operations.

	// ListModifiedWorkspaceIDs returns workspace identifiers for records modified within the time range.
	//
	// Workspaces represent top-level organizational containers and collaboration spaces.
	// This method identifies workspace changes including metadata updates, ownership transfers,
	// and configuration modifications that require synchronization between stores.
	//
	// Synchronization Priority: Medium
	// Workspaces change infrequently but are fundamental to the application hierarchy.
	// Workspace inconsistencies affect user access to all contained pages and blocks.
	//
	// Typical Workflow:
	//   1. Call this method to identify changed workspace IDs
	//   2. Use GetWorkspace to retrieve full workspace entities
	//   3. Apply changes to the destination store using CreateWorkspace or UpdateWorkspace
	//   4. Verify synchronization success with permission checks
	//
	// Performance: Low volume, high impact operations requiring careful ordering during sync.
	ListModifiedWorkspaceIDs(ctx context.Context, since, until time.Time) ([]models.WorkspaceID, error)

	// ListModifiedPageIDs returns page identifiers for records modified within the time range.
	//
	// Pages represent documents within workspaces, supporting hierarchical parent-child
	// relationships. This method captures changes to page metadata, titles, organizational
	// structure, and hierarchical relationships that require synchronization.
	//
	// Synchronization Priority: High
	// Page structure changes affect navigation, access control inheritance, and content
	// organization. Inconsistent page hierarchies disrupt user workflows and collaboration.
	//
	// Typical Workflow:
	//   1. Call this method to identify changed page IDs
	//   2. Use GetPage to retrieve full page entities with relationships
	//   3. Synchronize parent-child relationships before creating/updating pages
	//   4. Validate workspace membership and access control inheritance
	//
	// Performance: Medium volume operations requiring dependency ordering for hierarchical consistency.
	ListModifiedPageIDs(ctx context.Context, since, until time.Time) ([]models.PageID, error)

	// ListModifiedBlockIDs returns block identifiers for records modified within the time range.
	//
	// Blocks represent individual content elements within pages (text, headings, lists,
	// images, tables, etc.). This method captures the highest frequency changes as users
	// actively edit document content, modify block types, and reorganize content structure.
	//
	// Synchronization Priority: Critical
	// Block changes represent actual user content and are the most frequent modifications.
	// Content inconsistencies directly impact user experience and collaborative editing workflows.
	//
	// Typical Workflow:
	//   1. Call this method to identify changed block IDs (expect high volume)
	//   2. Use GetBlock to retrieve full block entities with content
	//   3. Batch synchronization operations for performance optimization
	//   4. Handle block ordering and parent-child relationships carefully
	//
	// Performance: Highest volume operations requiring batch processing and efficient content serialization.
	ListModifiedBlockIDs(ctx context.Context, since, until time.Time) ([]models.BlockID, error)

	// ListModifiedUserIDs returns user identifiers for records modified within the time range.
	//
	// Users represent account entities with authentication data, profile information, and
	// application preferences. This method captures changes to user profiles, authentication
	// credentials, avatar updates, and preference modifications.
	//
	// Synchronization Priority: High
	// User data affects authentication, authorization, and personalization across the application.
	// Inconsistent user data can prevent login or cause authorization failures.
	//
	// Typical Workflow:
	//   1. Call this method to identify changed user IDs
	//   2. Use GetUser to retrieve full user profile data
	//   3. Synchronize authentication-related changes carefully
	//   4. Validate email uniqueness constraints during synchronization
	//
	// Performance: Low to medium volume, but critical for application access and security.
	ListModifiedUserIDs(ctx context.Context, since, until time.Time) ([]models.UserID, error)

	// ListModifiedCommentIDs returns comment identifiers for records modified within the time range.
	//
	// Comments represent collaborative discussions, annotations, and feedback on content blocks.
	// This method captures changes to comment content, resolution status, and collaborative
	// interactions that enhance the note-taking and review workflows.
	//
	// Synchronization Priority: Medium
	// Comments support collaborative features but don't affect core content or access control.
	// Comment inconsistencies may disrupt discussions but don't block primary workflows.
	//
	// Typical Workflow:
	//   1. Call this method to identify changed comment IDs
	//   2. Use GetComment to retrieve full comment data with user attribution
	//   3. Synchronize comment resolution status and threading relationships
	//   4. Ensure block associations remain valid during synchronization
	//
	// Performance: Medium volume operations with dependencies on blocks and users.
	ListModifiedCommentIDs(ctx context.Context, since, until time.Time) ([]models.CommentID, error)

	// ListModifiedPermissionIDs returns permission identifiers for records modified within the time range.
	//
	// Permissions represent access control rules, sharing configurations, and security policies
	// for workspaces and pages. This method captures changes to user permissions, access rights,
	// and sharing settings that affect authorization throughout the application.
	//
	// Synchronization Priority: Critical
	// Permission inconsistencies can cause security vulnerabilities, unauthorized access,
	// or prevent legitimate users from accessing their content. Security-critical operations
	// require immediate and accurate synchronization.
	//
	// Typical Workflow:
	//   1. Call this method to identify changed permission IDs
	//   2. Use GetPermissions or implement GetPermission to retrieve full permission data
	//   3. Synchronize permission changes with proper authorization validation
	//   4. Verify permission hierarchies and inheritance rules remain consistent
	//
	// Performance: Low to medium volume but requires immediate synchronization for security.
	// Note: Consider adding GetPermission(ctx, id) method to support individual permission retrieval.
	ListModifiedPermissionIDs(ctx context.Context, since, until time.Time) ([]models.PermissionID, error)
}

// Package models defines the domain entities and business logic for the hierarchical note-taking application.
//
// This package provides the core data model for hierarchical document organization,
// demonstrating how to build flexible document hierarchies with user management, access
// control, and collaborative features. The models are designed to work consistently across
// both PostgreSQL and SurrealDB backends through careful design of relationships and constraints.
//
// # Domain Model Architecture
//
// The application implements a hierarchical data structure inspired by modern collaborative
// document platforms. Each entity serves a specific purpose in the content organization hierarchy:
//
//   - [Workspace]: Top-level containers that organize all content, owned by users and
//     serving as the primary organizational unit for documents and collaboration
//   - [Page]: Core content units that support nesting through parent-child relationships,
//     enabling flexible document hierarchies within workspaces
//   - [Block]: Building blocks of content within pages including text, headings, lists,
//     code blocks, images, tables, and todo items for rich document composition
//   - [User]: System users with authentication credentials, profile information, and
//     ownership relationships to workspaces and content
//   - [Permission]: Fine-grained access control for resources, supporting read, write,
//     and admin permission levels with inheritance from workspace to pages
//   - [Comment]: Discussion threads attached to specific blocks, enabling collaborative
//     annotations, feedback, and team communication on content
//   - Revision Support: Version history tracking for pages to enable rollback and change
//     auditing (planned feature for production implementations)
//
// This structure enables flexible content organization with clear ownership boundaries,
// collaborative features, and fine-grained access control across different database backends.
//
// # Typed IDs
//
// This package defines strongly-typed identifiers for each entity: [UserID], [WorkspaceID],
// [PageID], [BlockID], [PermissionID], [CommentID], and [ResourceID] (a polymorphic ID for
// permissions). These typed IDs provide compile-time type safety while enabling seamless
// operation across PostgreSQL and SurrealDB through automatic format conversion.
//
// Each typed ID wraps a UUID but knows its associated database table at compile time.
// In PostgreSQL, they store and retrieve standard UUIDs for foreign key relationships.
// In SurrealDB, they automatically marshal to and from RecordID through custom CBOR encoding.
// RecordID is a SurrealDB data type that identifies a record within a specific table - it's
// not a formatted string but a structured type containing the table name and record identifier,
// enabling the document-graph model without requiring separate model definitions for each database.
//
// This approach eliminates runtime configuration errors and prevents type mismatches - the
// compiler ensures you cannot accidentally use a UserID where a WorkspaceID is expected.
// The single model definition works for both databases, making it ideal for CQRS patterns
// and zero-downtime migrations between database backends.
//
// Beyond type safety, these IDs handle all serialization concerns automatically. They provide
// proper JSON marshaling for API responses, CBOR marshaling for SurrealDB communication, and
// SQL driver integration for PostgreSQL operations. Each ID type includes an IsZero() method
// for validation, ensuring uninitialized IDs are properly detected. The [ResourceID] type
// deserves special mention as a polymorphic identifier that can reference either workspaces
// or pages, enabling flexible permission systems across different resource types.
//
// # Entity Relationships and Constraints
//
// The domain model enforces these business rules through database constraints:
//
// Ownership Hierarchy:
//   - Users own Workspaces (User.ID → Workspace.OwnerID)
//   - Users create Pages within Workspaces (User.ID → Page.CreatedBy)
//   - All content belongs to a Workspace for access control
//
// Content Hierarchy:
//   - Pages can have parent Pages within the same Workspace (Page.ParentPageID)
//   - Blocks belong to Pages and can have parent Blocks for nesting (Block.ParentBlockID)
//   - Hierarchical relationships enable tree-like content organization
//
// Access Control:
//   - Permissions reference either Workspaces or Pages through polymorphic ResourceID
//   - Workspace permissions inherit to all contained Pages
//   - Page-specific permissions can override Workspace permissions
//   - Users have implicit admin permissions on resources they own
//
// Collaboration:
//   - Comments attach to specific Blocks for contextual discussion
//   - Users can be granted read, write, or admin permissions on resources
//   - Permission levels form a hierarchy (admin > write > read)
//
// # Content Model and Block Types
//
// The flexible content model supports various types of information:
//
// Block Types ([BlockType]):
//   - text: Plain text content with optional formatting
//   - heading: Hierarchical headings (H1, H2, H3, etc.)
//   - list: Ordered or unordered lists with nested items
//   - code: Code blocks with syntax highlighting
//   - image: Embedded images with captions and alt text
//   - table: Structured tabular data
//   - todo: Task items with completion status
//
// Content Storage:
// Block content uses [JSONMap], a flexible key-value structure that stores dynamic
// data in each database's native format while maintaining queryability. See [JSONMap]
// for implementation details and alternative storage strategies considered.
//
// # Permission Model and Access Control
//
// The permission system implements role-based access control:
//
// Permission Levels ([PermissionLevel]):
//   - read: View content and navigate structure
//   - write: Create, edit, and delete content (includes read)
//   - admin: Manage permissions and settings (includes write and read)
//
// Resource Types ([ResourceType]):
//   - workspace: Permissions apply to entire workspace and contained pages
//   - page: Permissions apply to specific page and its content blocks
//
// Permission Inheritance:
// Workspace permissions apply to all pages within that workspace unless
// overridden by page-specific permissions, enabling efficient permission
// management at scale.
//
// # Database-Specific Features
//
// The models are designed to work identically across different database backends:
//
// PostgreSQL Compatibility:
//   - GORM struct tags define foreign keys, indexes, and constraints
//   - Relational integrity enforced through database constraints
//   - JSONMap fields use PostgreSQL's JSONB for efficient querying
//   - Soft deletion support through gorm.DeletedAt
//
// SurrealDB Compatibility:
//   - Models marshal to SurrealDB records through CBOR encoding
//   - Typed IDs automatically convert to SurrealDB RecordIDs
//   - Flexible schema allows for document-style storage
//   - Relationships preserved through RecordID references
//
// # Audit Trail and Timestamps
//
// All entities include automatic timestamp management:
//   - CreatedAt: Set automatically when entity is first created
//   - UpdatedAt: Updated automatically on every modification
//   - DeletedAt: Used for soft deletion when supported (PostgreSQL)
//
// These timestamps enable the CQRS migration system to track changes
// and synchronize data between different database backends.
//
// # Production Enhancements
//
// For production use, consider these enhancements:
//   - Validation tags using go-playground/validator for input validation
//   - Domain-driven design aggregates for complex business operations
//   - Value objects for business rules (email validation, content limits)
//   - Event sourcing for complete audit trails of all changes
//   - Versioning support for content revision history
//   - Rich text support with structured formatting information
//   - File attachment handling for images and documents
//   - Search indexing for full-text content discovery
//
// # Usage Examples
//
//	// Create a workspace with typed IDs
//	workspace := &models.Workspace{
//		ID:      models.NewWorkspaceID(),
//		Name:    "My Workspace",
//		OwnerID: userID,
//	}
//
//	// Create a page with hierarchical relationship
//	page := &models.Page{
//		ID:           models.NewPageID(),
//		WorkspaceID:  workspace.ID,
//		ParentPageID: &parentPageID, // Optional: nil for root pages
//		Title:        "Meeting Notes",
//		CreatedBy:    userID,
//	}
//
//	// Create content blocks
//	block := &models.Block{
//		ID:      models.NewBlockID(),
//		PageID:  page.ID,
//		Type:    models.BlockTypeText,
//		Content: models.JSONMap{"text": "Meeting agenda"},
//		Order:   0,
//	}
package models

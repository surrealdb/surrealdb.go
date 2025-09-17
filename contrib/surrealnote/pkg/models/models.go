package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// BlockType represents the type of content block
type BlockType string

const (
	BlockTypeText    BlockType = "text"
	BlockTypeHeading BlockType = "heading"
	BlockTypeList    BlockType = "list"
	BlockTypeCode    BlockType = "code"
	BlockTypeImage   BlockType = "image"
	BlockTypeTable   BlockType = "table"
	BlockTypeTodo    BlockType = "todo"
)

// PermissionLevel represents the access level for a resource
type PermissionLevel string

const (
	PermissionRead  PermissionLevel = "read"
	PermissionWrite PermissionLevel = "write"
	PermissionAdmin PermissionLevel = "admin"
)

// ResourceType represents the type of resource for permissions
type ResourceType string

const (
	ResourceWorkspace ResourceType = "workspace"
	ResourcePage      ResourceType = "page"
)

// JSONMap is a flexible key-value map for storing dynamic content data across different database backends.
// Despite its name suggesting JSON-only usage, it serves as a universal structured data container that
// adapts to each database's native format: PostgreSQL's JSONB for efficient querying and indexing,
// and SurrealDB's object type for nested document storage.
//
// This type is used for Block content where the structure varies by block type (text blocks might
// have "text" and "format" fields, while image blocks have "url" and "caption" fields). The flexible
// schema allows rich content without requiring separate tables or rigid field definitions, and the
// content remains queryable in both databases (e.g., searching text within blocks).
//
// Alternative storage strategies considered:
//   - Polymorphic tables: Separate TextBlock, ImageBlock, TodoBlock tables with foreign keys back
//     to blocks table. Pros: type safety, referential integrity. Cons: complex joins, migration overhead.
//   - Protocol buffers: Encode content as protobuf, store as bytea/bytes. Pros: schema versioning,
//     compact storage, backward compatibility. Cons: loses queryability, requires deserialization.
//   - Single table inheritance: All block types in one table with nullable columns. Pros: simple
//     queries. Cons: sparse data, many NULL columns, schema bloat.
type JSONMap map[string]any

// Value implements the driver.Valuer interface for database storage
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for database retrieval
func (j *JSONMap) Scan(value any) error {
	if value == nil {
		*j = make(map[string]any)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		bytes = []byte(value.(string))
	}
	return json.Unmarshal(bytes, j)
}

// Workspace represents a top-level container using typed IDs.
// A production system would validate name length, implement workspace limits per user,
// add billing/subscription fields, and include workspace settings/preferences as JSONB.
type Workspace struct {
	ID        WorkspaceID    `gorm:"type:uuid;primary_key" json:"id"`
	Name      string         `gorm:"not null" json:"name"` // Should include length validation and sanitization
	OwnerID   UserID         `gorm:"type:uuid;not null" json:"owner_id"`
	Owner     *User          `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	// Missing fields for production: Settings, Subscription, MemberCount, StorageUsed
}

// BeforeCreate hook to generate ID if not set
func (w *Workspace) BeforeCreate(tx *gorm.DB) error {
	if w.ID.IsZero() {
		w.ID = NewWorkspaceID()
	}
	return nil
}

// Page represents a core content unit using typed IDs
type Page struct {
	ID           PageID         `gorm:"type:uuid;primary_key" json:"id"`
	WorkspaceID  WorkspaceID    `gorm:"type:uuid;not null" json:"workspace_id"`
	Workspace    *Workspace     `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	ParentPageID *PageID        `gorm:"type:uuid" json:"parent_page_id,omitempty"`
	ParentPage   *Page          `gorm:"foreignKey:ParentPageID" json:"parent_page,omitempty"`
	Title        string         `gorm:"not null" json:"title"`
	Icon         string         `json:"icon,omitempty"`
	CoverImage   string         `json:"cover_image,omitempty"`
	CreatedBy    UserID         `gorm:"type:uuid;not null" json:"created_by"`
	Creator      *User          `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Properties   JSONMap        `gorm:"type:jsonb" json:"properties,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate hook to generate ID if not set
func (p *Page) BeforeCreate(tx *gorm.DB) error {
	if p.ID.IsZero() {
		p.ID = NewPageID()
	}
	return nil
}

// Block represents a building block of content using typed IDs
type Block struct {
	ID            BlockID        `gorm:"type:uuid;primary_key" json:"id"`
	PageID        PageID         `gorm:"type:uuid;not null" json:"page_id"`
	Page          *Page          `gorm:"foreignKey:PageID" json:"page,omitempty"`
	Type          BlockType      `gorm:"not null" json:"type"`
	Content       JSONMap        `gorm:"type:jsonb" json:"content"`
	Order         int            `gorm:"not null" json:"order"`
	ParentBlockID *BlockID       `gorm:"type:uuid" json:"parent_block_id,omitempty"`
	ParentBlock   *Block         `gorm:"foreignKey:ParentBlockID" json:"parent_block,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate hook to generate ID if not set
func (b *Block) BeforeCreate(tx *gorm.DB) error {
	if b.ID.IsZero() {
		b.ID = NewBlockID()
	}
	return nil
}

// User represents a user account using typed IDs
type User struct {
	ID        UserID         `gorm:"type:uuid;primary_key" json:"id"`
	Email     string         `gorm:"unique;not null" json:"email"`
	Name      string         `gorm:"not null" json:"name"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate hook to generate ID if not set
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID.IsZero() {
		u.ID = NewUserID()
	}
	return nil
}

// Permission represents access control using typed IDs
type Permission struct {
	ID              PermissionID    `gorm:"type:uuid;primary_key" json:"id"`
	ResourceType    ResourceType    `gorm:"not null" json:"resource_type"`
	ResourceID      ResourceID      `gorm:"type:uuid;not null" json:"resource_id"`
	UserID          UserID          `gorm:"type:uuid;not null" json:"user_id"`
	User            *User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	PermissionLevel PermissionLevel `gorm:"not null" json:"permission_level"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// BeforeCreate hook to generate ID and set resource table
func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID.IsZero() {
		p.ID = NewPermissionID()
	}
	// Set ResourceID table based on ResourceType
	p.ResourceID.SetTableForResourceType(p.ResourceType)
	return nil
}

// Comment represents a comment on a block using typed IDs
type Comment struct {
	ID         CommentID      `gorm:"type:uuid;primary_key" json:"id"`
	BlockID    BlockID        `gorm:"type:uuid;not null" json:"block_id"`
	Block      *Block         `gorm:"foreignKey:BlockID" json:"block,omitempty"`
	UserID     UserID         `gorm:"type:uuid;not null" json:"user_id"`
	User       *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	ResolvedAt *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate hook to generate ID if not set
func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	if c.ID.IsZero() {
		c.ID = NewCommentID()
	}
	return nil
}

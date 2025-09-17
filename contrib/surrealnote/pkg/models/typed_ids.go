package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
	surrealdb_models "github.com/surrealdb/surrealdb.go/pkg/models"
)

// WorkspaceID is a typed ID for workspaces
type WorkspaceID struct {
	uuid uuid.UUID
}

func NewWorkspaceID() WorkspaceID {
	return WorkspaceID{uuid: uuid.New()}
}

func NewWorkspaceIDFromUUID(id uuid.UUID) WorkspaceID {
	return WorkspaceID{uuid: id}
}

func ParseWorkspaceID(s string) (WorkspaceID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return WorkspaceID{}, fmt.Errorf("invalid workspace ID: %w", err)
	}
	return WorkspaceID{uuid: id}, nil
}

func (w WorkspaceID) UUID() uuid.UUID { return w.uuid }
func (w WorkspaceID) String() string  { return w.uuid.String() }
func (w WorkspaceID) IsZero() bool    { return w.uuid == uuid.Nil }

func (w WorkspaceID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: "workspaces",
		ID:    w.uuid.String(),
	}
}

func (w WorkspaceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.uuid.String())
}

func (w *WorkspaceID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	w.uuid = id
	return nil
}

func (w WorkspaceID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{"workspaces", w.uuid.String()},
	})
}

func (w *WorkspaceID) UnmarshalCBOR(data []byte) error {
	return unmarshalCBORID(data, "workspaces", &w.uuid)
}

func (w WorkspaceID) Value() (driver.Value, error) {
	if w.IsZero() {
		return nil, nil
	}
	return w.uuid.String(), nil
}

func (w *WorkspaceID) Scan(value any) error {
	return scanUUID(value, &w.uuid)
}

func (WorkspaceID) GormDataType() string { return "uuid" }

// PageID is a typed ID for pages
type PageID struct {
	uuid uuid.UUID
}

func NewPageID() PageID {
	return PageID{uuid: uuid.New()}
}

func NewPageIDFromUUID(id uuid.UUID) PageID {
	return PageID{uuid: id}
}

func ParsePageID(s string) (PageID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return PageID{}, fmt.Errorf("invalid page ID: %w", err)
	}
	return PageID{uuid: id}, nil
}

func (p PageID) UUID() uuid.UUID { return p.uuid }
func (p PageID) String() string  { return p.uuid.String() }
func (p PageID) IsZero() bool    { return p.uuid == uuid.Nil }

func (p PageID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: "pages",
		ID:    p.uuid.String(),
	}
}

func (p PageID) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.uuid.String())
}

func (p *PageID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	p.uuid = id
	return nil
}

func (p PageID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{"pages", p.uuid.String()},
	})
}

func (p *PageID) UnmarshalCBOR(data []byte) error {
	return unmarshalCBORID(data, "pages", &p.uuid)
}

func (p PageID) Value() (driver.Value, error) {
	if p.IsZero() {
		return nil, nil
	}
	return p.uuid.String(), nil
}

func (p *PageID) Scan(value any) error {
	return scanUUID(value, &p.uuid)
}

func (PageID) GormDataType() string { return "uuid" }

// BlockID is a typed ID for blocks
type BlockID struct {
	uuid uuid.UUID
}

func NewBlockID() BlockID {
	return BlockID{uuid: uuid.New()}
}

func NewBlockIDFromUUID(id uuid.UUID) BlockID {
	return BlockID{uuid: id}
}

func ParseBlockID(s string) (BlockID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return BlockID{}, fmt.Errorf("invalid block ID: %w", err)
	}
	return BlockID{uuid: id}, nil
}

func (b BlockID) UUID() uuid.UUID { return b.uuid }
func (b BlockID) String() string  { return b.uuid.String() }
func (b BlockID) IsZero() bool    { return b.uuid == uuid.Nil }

func (b BlockID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: "blocks",
		ID:    b.uuid.String(),
	}
}

func (b BlockID) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.uuid.String())
}

func (b *BlockID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	b.uuid = id
	return nil
}

func (b BlockID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{"blocks", b.uuid.String()},
	})
}

func (b *BlockID) UnmarshalCBOR(data []byte) error {
	return unmarshalCBORID(data, "blocks", &b.uuid)
}

func (b BlockID) Value() (driver.Value, error) {
	if b.IsZero() {
		return nil, nil
	}
	return b.uuid.String(), nil
}

func (b *BlockID) Scan(value any) error {
	return scanUUID(value, &b.uuid)
}

func (BlockID) GormDataType() string { return "uuid" }

// UserID is a typed ID for users
type UserID struct {
	uuid uuid.UUID
}

func NewUserID() UserID {
	return UserID{uuid: uuid.New()}
}

func NewUserIDFromUUID(id uuid.UUID) UserID {
	return UserID{uuid: id}
}

func ParseUserID(s string) (UserID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UserID{}, fmt.Errorf("invalid user ID: %w", err)
	}
	return UserID{uuid: id}, nil
}

func (u UserID) UUID() uuid.UUID { return u.uuid }
func (u UserID) String() string  { return u.uuid.String() }
func (u UserID) IsZero() bool    { return u.uuid == uuid.Nil }

func (u UserID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: "users",
		ID:    u.uuid.String(),
	}
}

func (u UserID) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.uuid.String())
}

func (u *UserID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	u.uuid = id
	return nil
}

func (u UserID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{"users", u.uuid.String()},
	})
}

func (u *UserID) UnmarshalCBOR(data []byte) error {
	return unmarshalCBORID(data, "users", &u.uuid)
}

func (u UserID) Value() (driver.Value, error) {
	if u.IsZero() {
		return nil, nil
	}
	return u.uuid.String(), nil
}

func (u *UserID) Scan(value any) error {
	return scanUUID(value, &u.uuid)
}

func (UserID) GormDataType() string { return "uuid" }

// PermissionID is a typed ID for permissions
type PermissionID struct {
	uuid uuid.UUID
}

func NewPermissionID() PermissionID {
	return PermissionID{uuid: uuid.New()}
}

func NewPermissionIDFromUUID(id uuid.UUID) PermissionID {
	return PermissionID{uuid: id}
}

func ParsePermissionID(s string) (PermissionID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return PermissionID{}, fmt.Errorf("invalid permission ID: %w", err)
	}
	return PermissionID{uuid: id}, nil
}

func (p PermissionID) UUID() uuid.UUID { return p.uuid }
func (p PermissionID) String() string  { return p.uuid.String() }
func (p PermissionID) IsZero() bool    { return p.uuid == uuid.Nil }

func (p PermissionID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: "permissions",
		ID:    p.uuid.String(),
	}
}

func (p PermissionID) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.uuid.String())
}

func (p *PermissionID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	p.uuid = id
	return nil
}

func (p PermissionID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{"permissions", p.uuid.String()},
	})
}

func (p *PermissionID) UnmarshalCBOR(data []byte) error {
	return unmarshalCBORID(data, "permissions", &p.uuid)
}

func (p PermissionID) Value() (driver.Value, error) {
	if p.IsZero() {
		return nil, nil
	}
	return p.uuid.String(), nil
}

func (p *PermissionID) Scan(value any) error {
	return scanUUID(value, &p.uuid)
}

func (PermissionID) GormDataType() string { return "uuid" }

// CommentID is a typed ID for comments
type CommentID struct {
	uuid uuid.UUID
}

func NewCommentID() CommentID {
	return CommentID{uuid: uuid.New()}
}

func NewCommentIDFromUUID(id uuid.UUID) CommentID {
	return CommentID{uuid: id}
}

func ParseCommentID(s string) (CommentID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return CommentID{}, fmt.Errorf("invalid comment ID: %w", err)
	}
	return CommentID{uuid: id}, nil
}

func (c CommentID) UUID() uuid.UUID { return c.uuid }
func (c CommentID) String() string  { return c.uuid.String() }
func (c CommentID) IsZero() bool    { return c.uuid == uuid.Nil }

func (c CommentID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: "comments",
		ID:    c.uuid.String(),
	}
}

func (c CommentID) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.uuid.String())
}

func (c *CommentID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	c.uuid = id
	return nil
}

func (c CommentID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{"comments", c.uuid.String()},
	})
}

func (c *CommentID) UnmarshalCBOR(data []byte) error {
	return unmarshalCBORID(data, "comments", &c.uuid)
}

func (c CommentID) Value() (driver.Value, error) {
	if c.IsZero() {
		return nil, nil
	}
	return c.uuid.String(), nil
}

func (c *CommentID) Scan(value any) error {
	return scanUUID(value, &c.uuid)
}

func (CommentID) GormDataType() string { return "uuid" }

// ResourceID is a generic ID that can represent either a workspace or page
// Used in Permission model for flexible resource references
type ResourceID struct {
	uuid      uuid.UUID
	tableName string
}

func NewResourceIDForWorkspace(id WorkspaceID) ResourceID {
	return ResourceID{uuid: id.UUID(), tableName: "workspaces"}
}

func NewResourceIDForPage(id PageID) ResourceID {
	return ResourceID{uuid: id.UUID(), tableName: "pages"}
}

func (r ResourceID) UUID() uuid.UUID { return r.uuid }
func (r ResourceID) String() string  { return r.uuid.String() }
func (r ResourceID) IsZero() bool    { return r.uuid == uuid.Nil }

func (r ResourceID) RecordID() surrealdb_models.RecordID {
	return surrealdb_models.RecordID{
		Table: r.tableName,
		ID:    r.uuid.String(),
	}
}

func (r ResourceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.uuid.String())
}

func (r *ResourceID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	r.uuid = id
	return nil
}

func (r ResourceID) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  8,
		Content: []any{r.tableName, r.uuid.String()},
	})
}

func (r *ResourceID) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty CBOR data")
	}

	majorType := data[0] >> 5
	if majorType == 6 {
		var tag cbor.Tag
		if err := cbor.Unmarshal(data, &tag); err != nil {
			return err
		}

		if tag.Number != 8 {
			return fmt.Errorf("expected RecordID tag (8), got %d", tag.Number)
		}

		if arr, ok := tag.Content.([]any); ok && len(arr) == 2 {
			if table, ok := arr[0].(string); ok {
				r.tableName = table
				if idStr, ok := arr[1].(string); ok {
					parsedUUID, err := uuid.Parse(idStr)
					if err != nil {
						return fmt.Errorf("invalid UUID in RecordID: %w", err)
					}
					r.uuid = parsedUUID
					return nil
				}
			}
		}
		return fmt.Errorf("invalid RecordID content format")
	}

	var uuidStr string
	if err := cbor.Unmarshal(data, &uuidStr); err != nil {
		return err
	}
	parsedUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		return err
	}
	r.uuid = parsedUUID
	return nil
}

func (r ResourceID) Value() (driver.Value, error) {
	if r.IsZero() {
		return nil, nil
	}
	return r.uuid.String(), nil
}

func (r *ResourceID) Scan(value any) error {
	return scanUUID(value, &r.uuid)
}

func (ResourceID) GormDataType() string { return "uuid" }

func (r *ResourceID) SetTableForResourceType(resourceType ResourceType) {
	switch resourceType {
	case ResourceWorkspace:
		r.tableName = "workspaces"
	case ResourcePage:
		r.tableName = "pages"
	}
}

// Helper functions

// scanUUID is a helper for implementing sql.Scanner interface for PostgreSQL/GORM
func scanUUID(value any, target *uuid.UUID) error {
	if value == nil {
		*target = uuid.Nil
		return nil
	}

	switch v := value.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return err
		}
		*target = id
	case []byte:
		id, err := uuid.ParseBytes(v)
		if err != nil {
			return err
		}
		*target = id
	default:
		return fmt.Errorf("cannot scan type %T into UUID", value)
	}
	return nil
}

// unmarshalCBORID is a helper for unmarshaling SurrealDB RecordID from CBOR.
// SurrealDB uses CBOR tag 8 to identify RecordID types in its binary protocol.
// The RecordID is encoded as [table_name, id_string] within the tag.
func unmarshalCBORID(data []byte, expectedTable string, target *uuid.UUID) error {
	if len(data) == 0 {
		return fmt.Errorf("empty CBOR data")
	}

	// Check if this is a CBOR tag (major type 6)
	majorType := data[0] >> 5
	if majorType != 6 {
		return fmt.Errorf("expected CBOR tag for RecordID, got major type %d", majorType)
	}

	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return fmt.Errorf("failed to unmarshal CBOR tag: %w", err)
	}

	// SurrealDB uses tag 8 for RecordID
	if tag.Number != 8 {
		return fmt.Errorf("expected RecordID tag (8), got %d", tag.Number)
	}

	arr, ok := tag.Content.([]any)
	if !ok || len(arr) != 2 {
		return fmt.Errorf("invalid RecordID format: expected [table, id] array")
	}

	table, ok := arr[0].(string)
	if !ok {
		return fmt.Errorf("invalid RecordID format: table name must be string")
	}

	if table != expectedTable {
		return fmt.Errorf("expected table %s, got %s", expectedTable, table)
	}

	idStr, ok := arr[1].(string)
	if !ok {
		return fmt.Errorf("invalid RecordID format: ID must be string")
	}

	parsedUUID, err := uuid.Parse(idStr)
	if err != nil {
		return fmt.Errorf("invalid UUID in RecordID: %w", err)
	}

	*target = parsedUUID
	return nil
}

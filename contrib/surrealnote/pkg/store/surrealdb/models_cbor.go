package surrealdb

// SurrealDB Store Design Evolution
// =================================
//
// HISTORICAL CONTEXT:
// -------------------
// Early implementations created separate SDB types (UserSDB, WorkspaceSDB, etc.) as a
// translation layer between application models and SurrealDB. This seemed necessary because:
//
// 1. RecordID Handling: SurrealDB uses RecordID format (table:id) for all IDs
// 2. Foreign Key Storage: We stored foreign keys as plain strings to enable WHERE queries
// 3. Nil Handling: Concerns about nil pointer handling in CBOR unmarshaling
// 4. Field Filtering: Wanted to exclude GORM-specific fields like DeletedAt
//
// WHY SDB TYPES WERE UNNECESSARY:
// --------------------------------
// After implementing typed IDs (UserID, WorkspaceID, etc.) with proper CBOR marshaling,
// we discovered that SDB types were actually NOT needed:
//
// 1. Typed IDs handle RecordID conversion automatically via MarshalCBOR/UnmarshalCBOR
// 2. Foreign keys can be stored as RecordIDs AND queried with parameters
// 3. IDs are never nil (always generated if zero)
// 4. Extra GORM fields don't hurt SurrealDB
//
// CURRENT APPROACH (Graph + Direct Models):
// ------------------------------------------
// We now use application models directly with SurrealDB by:
//
// 1. Storing foreign key fields as RecordIDs (automatic via typed ID marshaling)
// 2. Creating graph relationships with RELATE for efficient traversal
// 3. Using graph queries (->relationship->) instead of WHERE clauses
// 4. Supporting both approaches for maximum flexibility
//
// Example:
//   workspace.OwnerID -> marshals to RecordID automatically
//   RELATE user->owns->workspace -> creates graph edge
//   Query: SELECT ->owns->workspace FROM $user (graph traversal)
//   Or: SELECT * FROM workspaces WHERE owner_id = $owner (with RecordID param)
//
// COMMON MISTAKES TO AVOID:
// -------------------------
// 1. DON'T create wrapper types just for RecordID - typed IDs handle this
// 2. DON'T store foreign keys as strings - use typed IDs that marshal to RecordID
// 3. DON'T use string interpolation in queries - use parameterized queries
// 4. DON'T forget to create RELATE relationships for graph traversal
// 5. DON'T assume you need different models for different databases
//
// KEY INSIGHT:
// ------------
// The same models work for both PostgreSQL and SurrealDB when:
// - Typed IDs implement proper CBOR marshaling for RecordID format
// - Foreign keys are stored (for PostgreSQL) AND graph relationships created (for SurrealDB)
// - Queries use parameters with RecordID objects instead of string interpolation
//
// This unified model approach is perfect for CQRS patterns and database migration scenarios
// where you need to maintain compatibility between different database systems.

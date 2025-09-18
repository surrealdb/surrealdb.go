# Migration Guide to v1

This guide helps you migrate from surrealdb.go v0.x to v1.0. It covers all breaking changes and provides migration examples for each change.

## Breaking Changes

### GeometryPoint Constructor Removal

**Introduced in:** v0.11.0

**What changed:** `NewGeometryPoint` has been removed to prevent silent breaking changes from parameter reordering. This affects users upgrading from v0.10.0 or earlier.

**Why:** The previous parameter order (latitude, longitude) conflicted with SurrealDB's expectation based on the GeoJSON specification, which requires longitude to appear first. Simply reversing the parameters would cause existing code to compile but produce incorrect results, potentially causing data corruption or incorrect geospatial queries without any compile-time errors.

**Before (v0.x):**
```go
point := models.NewGeometryPoint(37.7749, -122.4194) // latitude, longitude
```

**After (v1.0):**
```go
// Use explicit field initialization with correct GeoJSON order
point := models.GeometryPoint{
    Longitude: -122.4194,
    Latitude:  37.7749,
}
```

**Migration steps:**
1. Find all usages of `NewGeometryPoint` in your codebase
2. Replace with explicit struct initialization using field names
3. Consider placing longitude before latitude in your struct initialization (not strictly necessary when using field names, but recommended for consistency with GeoJSON [longitude, latitude] convention and to help readers familiar with GeoJSON immediately recognize the coordinate pattern)
4. Update any tests that use `NewGeometryPoint`
5. Verify geospatial queries work as expected after migration

**Related issue:** #223

### Default CBOR Implementation Change

**Introduced in:** v0.11.0

**What changed:** The default CBOR implementation has changed from `fxamacker/cbor` to `surrealcbor`. This change provides better compatibility with SurrealDB's CBOR protocol, particularly for handling the NONE tag (tag 6) which is properly unmarshaled to Go `nil` values.

**Why:** `surrealcbor` is specifically designed for SurrealDB's CBOR protocol and provides:
- Proper handling of SurrealDB's NONE tag (unmarshals to Go `nil`)
- Better performance for SurrealDB-specific data types
- Native support for all SurrealDB CBOR tags (UUIDs, Record IDs, Geometry types, etc.)
- More robust handling of SurrealDB's custom datetime and duration formats

**Before (v0.x):**
```go
// Default configuration automatically used fxamacker/cbor
conf := connection.NewConfig(u)
// conf.Marshaler was &models.CborMarshaler{}
// conf.Unmarshaler was &models.CborUnmarshaler{}
```

**After (v1.0):**
```go
// Default configuration now automatically uses surrealcbor
conf := connection.NewConfig(u)
// conf.Marshaler is surrealcbor.New()
// conf.Unmarshaler is surrealcbor.New()

// To explicitly use the legacy fxamacker/cbor implementation:
conf.Marshaler = &models.CborMarshaler{}
conf.Unmarshaler = &models.CborUnmarshaler{}
```

**Environment variable support:**
```bash
# Use surrealcbor (default, no need to set)
# SURREALDB_CBOR_IMPL=""

# Explicitly use surrealcbor
SURREALDB_CBOR_IMPL="surrealcbor"

# Use legacy fxamacker/cbor implementation
SURREALDB_CBOR_IMPL="fxamackercbor"
```

**Key behavioral difference:**

When selecting non-existent records, the two implementations behave differently:

- **fxamacker/cbor**: Returns a struct with nil fields (e.g., `user.ID == nil`)
- **surrealcbor**: Returns nil for the entire struct (e.g., `user == nil`)

**Code that may need updating:**
```go
// Old code that worked with fxamacker/cbor default:
user, err := surrealdb.Select[User](ctx, db, recordID)
if err != nil {
    return err
}
if user.ID == nil {
    // Handle non-existent record
}

// Updated code for surrealcbor default:
user, err := surrealdb.Select[User](ctx, db, recordID)
if err != nil {
    return err
}
if user == nil {
    // Handle non-existent record
}
```

**Migration steps:**
1. **No action required for most users** - the new default provides better compatibility
2. If you were explicitly setting surrealcbor before, you can remove the explicit configuration as it's now the default
3. Update any code that depends on fxamacker/cbor-specific behavior to work with surrealcbor
4. **Update tests and code that check for non-existent records** - change from checking field nullability to checking struct nullability
5. Test your application thoroughly, paying attention to:
   - NONE/null value handling
   - Non-existent record detection
   - Custom datetime parsing
   - UUID and Record ID serialization
   - Geometry type handling

**Note:** `models.CborMarshaler` and `models.CborUnmarshaler` are deprecated in v0.11.0 and will be removed in v1. Use `surrealcbor.New()` instead.

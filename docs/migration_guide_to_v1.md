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

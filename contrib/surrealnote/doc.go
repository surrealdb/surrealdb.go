// Package surrealnote demonstrates building a hierarchical note-taking application
// with dual database backend support (PostgreSQL and SurrealDB) and zero-downtime migration capabilities.
//
// This package serves as a practical example of how to build production-ready applications using
// the SurrealDB Go SDK while maintaining the flexibility to support multiple database backends.
// The implementation showcases real-world patterns for database migrations, CQRS architecture,
// and building maintainable APIs with different persistence layers.
//
// # Features
//
//   - Dual Database Support: Seamlessly work with PostgreSQL (using GORM ORM) and SurrealDB (using surrealql library)
//   - CQRS Pattern: Implements Command Query Responsibility Segregation without dual-write complexity during migration
//   - Zero-Downtime Migration: Switch between databases without service interruption using background synchronization
//   - Hierarchical Data Model: Pages, blocks, workspaces with parent-children relationships
//   - RESTful API: Complete CRUD operations for all entities
//   - Authentication Stubs: Basic user and permission models for demonstration (not production-ready)
//   - Permission Models: Simplified permission tracking without enforcement (does not use SurrealDB's built-in RBAC)
//
// # Architecture Overview
//
// The application demonstrates three key architectural patterns:
//
//   - Multi-Backend Support: Use [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] interface to abstract PostgreSQL (with GORM ORM)
//     and SurrealDB (without ORM) implementations
//   - CQRS Migration: Implement [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs.CQRSStore] for zero-downtime database migrations
//     with background synchronization and read-only switchover
//   - Command Pattern: Use [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote.Command] interface to organize application operations
//     (run, migrate, sync) with their specific configurations
//
// # Data Model
//
// The application implements a hierarchical data structure for collaborative document
// management with workspaces, pages, blocks, and rich permission controls. All entities
// use typed IDs for type safety and seamless operation across both PostgreSQL and SurrealDB.
//
// For detailed information about the domain model, entity relationships, and typed ID system,
// see [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models].
//
// # Migration Strategy
//
// The application demonstrates a safe approach for zero-downtime database migration:
//
//   - Single-write pattern: Writes go to one database at a time, avoiding partial failures
//   - Background synchronization: Data is synced between databases in batches
//   - Read-only switchover: Brief read-only period ensures data consistency during the final cutover
//   - Two sync strategies: Timestamp-based (using CreatedAt/UpdatedAt) or change tracking tables
//
// This approach maintains data consistency and allows rollback at any point during migration,
// while avoiding the complexity and risks of writing to multiple databases simultaneously.
//
// For detailed information about migration modes, synchronization strategies, and
// operational procedures, see [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs].
//
// # Package Organization
//
// For detailed information about sub-packages and their specific functionality,
// see [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg].
//
// # Getting Started
//
// For command-line usage, quick start examples, and application configuration,
// see [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote].
//
// # API Integration
//
// The [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client] package provides a Go HTTP client for programmatic access to the surrealnote API.
// The [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnotetesting] package includes utilities for load testing and virtual user simulation.
//
// For testing and development, see the end-to-end tests that demonstrate migration scenarios
// and data consistency validation across different database backends.
package surrealnote

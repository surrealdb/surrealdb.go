// Package pkg contains all the sub-packages for the surrealnote application.
//
// This package serves as a central namespace for organizing the application's core functionality
// into focused, single-purpose packages that work together to provide a complete note-taking
// application with multi-database support and zero-downtime migration capabilities.
//
// # Package Architecture
//
// The sub-packages are organized in three main layers:
//
// # Application Layer
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote] - Core application logic, command orchestration, and HTTP handlers.
// Contains the main application entry points and coordinates interactions between other packages.
// Use this package when implementing new commands or extending the HTTP API.
//
// # Domain Layer
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models] - Domain entities, business rules, and typed IDs for the note-taking system.
// Defines the core data structures that represent workspaces, pages, blocks, users, and permissions.
// Use this package when working with data models or implementing new entity types.
//
// # Infrastructure Layer
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store] - Data persistence layer abstraction with the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] interface.
// Provides a unified interface for database operations across different backend implementations.
// Use this package when implementing new persistence layers or database operations.
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/postgres] - PostgreSQL implementation using GORM ORM for relational data operations.
// Demonstrates how to implement the [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store.Store] interface with traditional SQL databases.
// Use this package as a reference for implementing other ORM-based stores.
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/surrealdb] - SurrealDB implementation using native SurrealQL without ORM abstractions.
// Shows how to work directly with SurrealDB's flexible document-graph model.
// Use this package as a reference for implementing other schema-flexible stores.
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/cqrs] - CQRS implementation coordinating dual writes between PostgreSQL and SurrealDB.
// Enables zero-downtime migration through dual-write patterns and eventual consistency.
// Use this package when implementing migration strategies or consistency patterns.
//
// # Integration Layer
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client] - HTTP client library for programmatic access to the surrealnote API.
// Provides strongly-typed methods for all API endpoints with proper error handling.
// Use this package when building integrations, testing tools, or client applications.
//
// [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnotetesting] - Testing utilities including virtual user simulations and load testing.
// Contains tools for comprehensive testing of multi-database scenarios and migration workflows.
// Use this package when implementing end-to-end tests or performance validation.
//
// # Package Dependencies
//
// The packages follow these dependency relationships:
//
//	surrealnote → store, models, client
//	store → models
//	store/postgres → store, models
//	store/surrealdb → store, models
//	store/cqrs → store, models
//	client → models
//	surrealnotetesting → client, models
//
// This design ensures clean separation of concerns while enabling focused testing
// and independent development of each layer.
package pkg

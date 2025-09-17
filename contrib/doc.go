// Package contrib provides additional functionality and utilities
// for the SurrealDB Go SDK.
//
// Everything in this package is intended to extend the core
// SurrealDB Go SDK with features that are not part of the core library.
// This includes example applications, testing utilities, experimental features,
// and other contributions that enhance the usability and functionality of the SDK.
//
// Note that this package is outside of the backward compatibility guarantees
// provided by the core SurrealDB Go SDK. Changes to this package may
// introduce breaking changes without following semantic versioning.
//
// The contrib directory includes [github.com/surrealdb/surrealdb.go/contrib/surrealnote], an example application that
// demonstrates zero-downtime migration from PostgreSQL to SurrealDB using a Notion-like
// hierarchical note-taking system. For building queries, [github.com/surrealdb/surrealdb.go/contrib/surrealql] provides a type-safe
// query builder. The [github.com/surrealdb/surrealdb.go/contrib/rews] package offers a reconnecting WebSocket implementation with
// automatic session restoration. Database management tools include [github.com/surrealdb/surrealdb.go/contrib/surrealdump] and
// [github.com/surrealdb/surrealdb.go/contrib/surrealrestore] for backup and recovery operations.
package contrib

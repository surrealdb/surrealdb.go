// The [surrealdb] package implements [SurrealDB RPC Protocol] in the Go way.
//
// # Connection Engines
//
// There are 2 different connection engines, WebSocket and HTTP, you can use to connect to SurrealDB backend.
//
// Provide a proper SurrealDB endpoint URL to [FromEndpointURLString] so that it chooses the right backend for you.
//
// # Data Models
//
// The [surrealdb] package facilitates communication between client and the backend service using the Concise
// Binary Object Representation (CBOR) format.
//
// For more information on CBOR and how it relates to SurrealDB's
// data models, please refer to the [github.com/surrealdb/surrealdb.go/pkg/models] package.
//
// # Use Query for most use cases
//
// For most use cases, you can use the [Query] function to execute SurrealQL statements.
//
// [Query] is recommended for both simple and complex queries, transactions, and when you need full control over your database operations.
//
// To ease writing queries for [Query] with more type-safety, you can use the [github.com/surrealdb/surrealdb.go/contrib/surrealql] package.
//
// # Use Send for low-level control
//
// [Send] is used internally by all data manipulation methods.
//
// Use it directly when you want to create requests yourself.
//
// [SurrealDB RPC Protcol]: https://surrealdb.com/docs/surrealdb/integration/rpc
package surrealdb

// Package surrealql provides a type-safe query builder for SurrealDB's SurrealQL language.
//
// This package allows you to construct SurrealQL queries programmatically with a fluent interface,
// ensuring type safety and preventing SurrealQL injection through proper parameter binding.
//
// The query builder supports the following SurrealQL operations:
//   - SELECT with WHERE, ORDER BY, LIMIT, GROUP BY, etc.
//   - COUNT and other aggregate functions
//   - CREATE with CONTENT
//   - UPDATE with SET
//   - DELETE
//   - RELATE for creating relationships
//   - RETURN clauses (NONE, DIFF, BEFORE, AFTER)
package surrealql

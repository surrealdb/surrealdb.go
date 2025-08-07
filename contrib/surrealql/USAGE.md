
# Usage

This document outlines the basic usage of the `surrealql` library
so that you can go through it and get started with the library quickly.

> [!IMPORTANT] Read the testable examples for more information!
>
> This is just an outline for basic usage- Please refer to the testable examples named `example_*_test.go` for more comprehensive and detailed documentation.
>
> They are considered part of the documentation, because they strive to
cover all the features and are verified as a part of our CI.

## Table of Contents

- [SELECT Query](#select-query)
- [WHERE Conditions](#where-conditions)
- [COUNT Queries](#count-queries)
- [Ordering and Pagination](#ordering-and-pagination)
- [Group By Clause](#group-by-clause)
- [RETURN Clauses](#return-clauses)
- [CREATE Queries](#create-queries)
- [UPDATE Queries](#update-queries)
- [DELETE Queries](#delete-queries)
- [RELATE Queries](#relate-queries)
- [Fetch Clause](#fetch-clause)
- [Parallel Clause](#parallel-clause)
- [Explain Clause](#explain-clause)
- [Integration with surrealdb.Query](#integration-with-surrealdbquery)
- [Using Aggregate Functions](#using-aggregate-functions)
- [Raw Queries](#raw-queries)

## SELECT Query

```go
// Simple select all
query := surrealql.Select("*").FromTable("users")
sql, vars := query.Build()
// SurrealQL: "SELECT * FROM users"

// Select specific fields
query := surrealql.Select("id", "name", "email").FromTable("users")
// SurrealQL: "SELECT id, name, email FROM users"
```

## WHERE Conditions

```go
// Using WhereEq for equality
query := surrealql.Select("*").FromTable("users").WhereEq("active", true)
// SurrealQL: "SELECT * FROM users WHERE active = $active_1"

// Using WhereNull to find null values
query := surrealql.Select("*").FromTable("users").WhereNull("deleted_at")
// SurrealQL: "SELECT * FROM users WHERE deleted_at IS NULL"

// Using WhereNotNull to find non-null values
query := surrealql.Select("*").FromTable("users").WhereNotNull("email")
// SurrealQL: "SELECT * FROM users WHERE email IS NOT NULL"

// Using WhereIn for multiple values
query := surrealql.Select("*").From("orders").
    WhereIn("status", "pending", "processing", "shipped")
// SurrealQL: "SELECT * FROM orders WHERE status IN ($status_1, $status_2, $status_3)"

// Using Where for complex conditions with placeholders
query := surrealql.Select("*").FromTable("products").
    Where("price BETWEEN ? AND ?", 10, 100)
// SurrealQL: "SELECT * FROM products WHERE price BETWEEN $param_1 AND $param_2"

// Combining multiple WHERE conditions
query := surrealql.Select("*").FromTable("users").
    WhereEq("active", true).
    WhereNotNull("email").
    WhereIn("role", "admin", "moderator").
    Where("created_at > ?", "2024-01-01")
// All conditions are combined with AND
```

## COUNT Queries

```go
// Count all records
query := surrealql.Select("count() as count").FromTable("users").GroupAll()
// SurrealQL: "SELECT count() FROM users GROUP ALL"

// Count specific field
query := surrealql.Select("count(id) as id_count").FromTable("orders").GroupAll()
// SurrealQL: "SELECT count(id) as id_count FROM orders GROUP ALL"

// Count with grouping
query := surrealql.Select("category", "count() as count").FromTable("products").GroupBy("category").OrderByDesc("count")
// SurrealQL: "SELECT category, count() AS count FROM products GROUP BY category ORDER BY count DESC"
```

## Ordering and Pagination

```go
query := surrealql.Select("*").FromTable("posts").
    OrderByDesc("created_at").
    Limit(10).
    Start(20)
// SurrealQL: "SELECT * FROM posts ORDER BY created_at DESC LIMIT 10 START 20"
```

## Group By Clause

```go
// Group by with aggregates
query := surrealql.Select("category", "count() AS total", "avg(price) AS avg_price").
    FromTable("products").
    GroupBy("category").
    OrderByDesc("total")
```

## RETURN Clauses

```go
// Return none - useful for write operations where you don't need the result
query := surrealql.Create("users").
    Set("name", "John").
    ReturnNone()
// SurrealQL: "CREATE users CONTENT $content_1 RETURN NONE"

// Return diff - useful for updates
query := surrealql.Update("users", "123").
    Set("name", "Jane").
    ReturnDiff()
// SurrealQL: "UPDATE users:123 SET $set_1 RETURN DIFF"
```

## CREATE Queries

```go
// Create with individual fields
query := surrealql.Create("users").
    Set("name", "John Doe").
    Set("email", "john@example.com").
    Set("active", true)

// Create with content map
query := surrealql.Create("users").Content(map[string]any{
    "name":  "John Doe",
    "email": "john@example.com",
    "roles": []string{"user", "admin"},
})
```

## UPDATE Queries

```go
// Update all records
query := surrealql.Update("users").
    Set("last_seen", time.Now())

// Update specific record
query := surrealql.Update("users", "123").
    Set("name", "Jane Doe")

// Update with conditions
query := surrealql.Update("users").
    Set("active", false).
    Where("last_login < ?", "2024-01-01")
```

## DELETE Queries

```go
// Delete all records
query := surrealql.Delete("users")

// Delete specific record
query := surrealql.Delete("users:123")

// Delete with conditions
query := surrealql.Delete("sessions").
    Where("expires_at < ?", time.Now())
```

## RELATE Queries

```go
// Create a relation
query := surrealql.Relate("users:123", "likes", "posts:456")

// Create a relation with properties
query := surrealql.Relate("users:123", "purchased", "products:789").
    Set("quantity", 2).
    Set("price", 29.99).
    Set("purchased_at", time.Now())
```

## Fetch Clause

```go
// Fetch related records
query := surrealql.Select("*").FromTable("posts").
    Fetch("author", "comments", "comments.author")
```

References:

- [FETCH clause | SurrealQL](https://surrealdb.com/docs/surrealql/clauses/fetch)

## Parallel Clause

Several SurrealQL statements support the `PARALLEL` clause:

```go
query := surrealql.Select("*").FromTable("large_table").
    WhereEq("processed", false).
    Parallel()
```

References:

- [SELECT statement | SurrealQL](https://surrealdb.com/docs/surrealql/statements/select#the-parallel-clause)
- [CREATE statement | SurrealQL](https://surrealdb.com/docs/surrealql/statements/create#parallel)

## Explain Clause

```go
query := surrealql.Select("*").FromTable("users").
    WhereEq("email", "user@example.com").
    Explain()
```

References:

- [EXPLAIN clause | SurrealQL](https://surrealdb.com/docs/surrealql/clauses/explain)

## Integration with surrealdb.Query

```go
import (
    "context"
    "github.com/surrealdb/surrealdb.go"
    "github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

// Build your query
query := surrealql.Select("id", "name").
    FromTable("users").
    WhereEq("active", true).
    OrderBy("created_at").
    Limit(10)

// Get SurrealQL and parameters
sql, vars := query.Build()

// Execute with surrealdb
ctx := context.Background()
result, err := surrealdb.Query[User](ctx, db, sql, vars)
```

## Using Aggregate Functions

In SurrealDB, aggregate functions work differently than traditional SQL. To aggregate values from a table, use aggregate functions with either `GROUP ALL` or `GROUP BY`.

More specifically:

- Use `SELECT math::sum(field)` with `GROUP [BY|ALL]` instead of `SELECT SUM(col)`
- Use `SELECT math::avg(field)` with `GROUP [BY|ALL]` instead of `SELECT AVG(col)`
- Use `SELECT math::max(field)` with `GROUP [BY|ALL]` instead of `SELECT MAX(col)`
- Use `SELECT math::min(field)` with `GROUP [BY|ALL]` instead of `SELECT MIN(col)`

```go
// Sum all values from a table
query := surrealql.Select(surrealql.Fn("math::sum").ArgFromField("amount")).
			FromTable("orders").
			GroupAll()
// Produces: SELECT math::sum(amount) FROM orders GROUP ALL

// Average ratings
query := surrealql.Select(surrealql.Fn("math::mean").ArgFromField("rating")).
			FromTable("reviews").
			GroupAll()

// Min/Max prices
minQuery := surrealql.Select(surrealql.Fn("math::min").ArgFromField("price")).
			FromTable("products").
			GroupAll()
maxQuery := surrealql.Select(surrealql.Fn("math::max").ArgFromField("price")).
			FromTable("products").
			GroupAll()

// For use in SELECT with GROUP BY, use the raw functions:
query := surrealql.Select("category", "math::sum(price) AS total").
    FromTable("products").
    GroupBy("category")
```

## Raw Queries

For cases where you need complete control:

```go
query := surrealql.Raw(
    "SELECT * FROM users WHERE created_at > $date",
    map[string]any{"date": "2024-01-01"},
)
```

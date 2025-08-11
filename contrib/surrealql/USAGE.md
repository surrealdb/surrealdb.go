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
- [CREATE Queries](#create-queries)
- [UPDATE Queries](#update-queries)
- [UPSERT Queries](#upsert-queries)
- [DELETE Queries](#delete-queries)
- [RELATE Queries](#relate-queries)
- [Transactions](#transactions)
- [Variables](#variables)
- [Integration with surrealdb.Query](#integration-with-surrealdbquery)

## SELECT Query

```go
// Simple select all from a table
query := surrealql.Select("users")
sql, vars := query.Build()
// SurrealQL: "SELECT * FROM users"

// Select specific fields
query := surrealql.Select("users").Fields("id", "name", "email")
// SurrealQL: "SELECT id, name, email FROM users"

// Select with WHERE conditions
query := surrealql.Select("users").
    Fields("id", "name").
    Where("age > ?", 18).
    Where("active = ?", true)

// Select with ordering and pagination
query := surrealql.Select("posts").
    OrderBy("created_at DESC").
    Limit(10).
    Start(20)

// Count queries with GROUP ALL
query := surrealql.Select("users").
    Field("count()").
    GroupAll()

// Select from multiple tables
query := surrealql.Select("users", "products")

// Select VALUE for single field value
query := surrealql.SelectValue("users.name")
```

## CREATE Queries

```go
// Create with individual fields using Set
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

// Create specific record ID
query := surrealql.Create("users:123").
    Set("name", "Alice").
    ReturnAfter()  // Returns the record after creation

// Create with RETURN NONE for better performance
query := surrealql.Create("logs").
    Set("message", "User logged in").
    ReturnNone()
```

## UPDATE Queries

```go
// Update all records in a table
query := surrealql.Update("users").
    Set("active", true).
    Where("last_login < ?", time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC))

// Update specific record
query := surrealql.Update("users:123").
    Set("name", "Jane Doe").
    Set("email", "jane.doe@example.com")

// Update with compound operations
query := surrealql.Update("products").
    Set("stock -= ?", 5).               // Decrement
    Set("sales_count += ?", 1).         // Increment
    Set("last_sold", "2024-01-01T00:00:00Z")

// Update with RETURN DIFF to see changes
query := surrealql.Update("users:123").
    Set("name", "Jane Doe").
    ReturnDiff()

// Update using RecordID
recordID := surrealql.Thing("users", 123)
query := surrealql.Update(recordID).
    Set("name", "Alice")
```

## UPSERT Queries

UPSERT creates a record if it doesn't exist, or updates it if it does.

```go
// Basic UPSERT with SET
query := surrealql.Upsert("product:laptop").
    Set("name", "Laptop Pro").
    Set("price", 1299)

// UPSERT with CONTENT (replaces entire record)
query := surrealql.Upsert("product:tablet").
    Content(map[string]any{
        "name":  "Tablet Pro",
        "price": 899,
    }).
    ReturnAfter()

// UPSERT with MERGE (updates specific fields)
query := surrealql.Upsert("product:headphones").
    Merge(map[string]any{
        "colors": []string{"Black", "White"},
    })

// UPSERT with JSON PATCH operations
query := surrealql.Upsert("product:keyboard").
    Patch([]surrealql.PatchOp{
        {Op: "add", Path: "/features/-", Value: "RGB Lighting"},
        {Op: "replace", Path: "/price", Value: 149},
    })

// UPSERT ONLY returns single record instead of array
query := surrealql.UpsertOnly("product:charger").
    Set("name", "Fast Charger").
    Set("available", true).
    ReturnAfter()

// UPSERT with WHERE condition
query := surrealql.Upsert("product:speaker").
    Set("last_updated", "2024-01-01T00:00:00Z").
    Set("status", "in_stock").
    Where("price >= ?", 100).
    ReturnDiff()

// UPSERT without data modification (creates if doesn't exist)
query := surrealql.Upsert("foo:1")
```

## DELETE Queries

```go
// Delete all records in a table
query := surrealql.Delete("sessions")

// Delete specific record
query := surrealql.Delete("users:123")

// Delete with conditions
query := surrealql.Delete("logs").
    Where("created_at < ?", time.Now().AddDate(0, -1, 0)).
    ReturnNone()
```

## RELATE Queries

```go
// Create a simple relation
query := surrealql.Relate(
    surrealql.Thing("users", 123),
    "purchased",
    surrealql.Thing("products", 456),
)

// Create a relation with properties
query := surrealql.Relate(
    surrealql.Thing("users", 123),
    "likes",
    surrealql.Thing("posts", 789),
).Set("rating", 5).
  Set("created_at", time.Now())

// Create relation with Content
query := surrealql.Relate(
    surrealql.Thing("users", 456),
    "follows",
    surrealql.Thing("users", 789),
).Content(map[string]any{
    "since": time.Now(),
    "mutual": true,
})
```

## Transactions

```go
// Create a transaction with multiple queries
createUser := surrealql.Create("users:123").Set("name", "Alice")
updateUser := surrealql.Update("users:123").Set("email", "alice@example.com")

tx := surrealql.Begin().
    Query(createUser).
    Query(updateUser)

sql, vars := tx.Build()
// Produces:
// BEGIN TRANSACTION;
// CREATE users:123 SET name = $param_1;
// UPDATE users:123 SET email = $param_1;
// COMMIT TRANSACTION;

// Transaction with conditional logic
tx := surrealql.Begin().
    Let("transfer_amount", 300.00).
    Raw("UPDATE account:one SET dollars -= $transfer_amount").
    If("account:one.dollars < 0").
    Then(func(tb *surrealql.ThenBuilder) {
        tb.Throw("Insufficient funds")
    }).
    End()

// Transaction with RETURN value
tx := surrealql.Begin().
    Let("name", "Alice").
    Let("email", "alice@example.com").
    Query(surrealql.Create("person").
        Set("name", surrealql.Var("name")).
        Set("email", surrealql.Var("email"))).
    Return("$name")
```

## Variables

Use `Var()` to reference SurrealQL variables (as opposed to literal values):

```go
// Using Var() for variable reference
query := surrealql.Create("users").
    Set("name", surrealql.Var("name")).  // References the variable $name
    Set("prefix", "$user")                // Literal string "$user"

// In transactions
tx := surrealql.Begin().
    Let("user_id", 123).
    Query(surrealql.Create("users").
        Set("id", surrealql.Var("user_id")))
```

## Integration with the main `surrealdb.go` package

The `surrealql` library integrates seamlessly with the main `surrealdb.go` package. You can use `models.Table` and `models.RecordID` for type-safe table and record targeting.

### Basic Integration

```go
import (
    "context"
    "github.com/surrealdb/surrealdb.go"
    "github.com/surrealdb/surrealdb.go/contrib/surrealql"
    "github.com/surrealdb/surrealdb.go/pkg/models"
)

// Define your model
type User struct {
    ID    models.RecordID `json:"id"`
    Name  string          `json:"name"`
    Email string          `json:"email"`
    Active bool           `json:"active"`
}

// Build your query
query := surrealql.Select("users").
    Fields("id", "name", "email").
    Where("active = ?", true).
    OrderBy("created_at").
    Limit(10)

// Get SurrealQL and parameters
sql, vars := query.Build()

// Execute with surrealdb
ctx := context.Background()
results, err := surrealdb.Query[[]User](ctx, db, sql, vars)
```

### Using models.Table for Dynamic Table Names

When table names come from user input or configuration, use `models.Table` for safe parameterization:

```go
// Safe handling of dynamic table names
func queryTable(ctx context.Context, db *surrealdb.DB, tableName string) ([]map[string]any, error) {
    // models.Table ensures proper CBOR encoding and parameterization
    table := models.Table(tableName)

    query := surrealql.Select(table).
        Fields("id", "name", "created_at").
        Where("active = ?", true)

    sql, vars := query.Build()
    // Produces: SELECT id, name, created_at FROM $from_table_1 WHERE active = $param_1
    // vars contains: from_table_1: tableName, param_1: true

    return surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
}

// Handle special characters in table names
specialTable := models.Table("user-sessions") // Hyphens are safely handled
query := surrealql.Select(specialTable)

// Handle reserved words as table names
reservedTable := models.Table("select") // Reserved word safely parameterized
query := surrealql.Select(reservedTable)
```

### Using models.RecordID for Specific Records

Target specific records using `models.RecordID`:

```go
// Query a specific record
recordID := models.NewRecordID("users", "john")
query := surrealql.Select(recordID).
    Fields("name", "email", "last_login")

sql, vars := query.Build()
// Produces: SELECT name, email, last_login FROM $from_id_1
// vars contains: from_id_1: users:john

// Update a specific record
recordID := models.NewRecordID("products", 12345)
updateQuery := surrealql.Update(recordID).
    Set("stock", 100).
    Set("updated_at", time.Now()).
    ReturnAfter()

sql, vars = updateQuery.Build()
// Produces: UPDATE $id_1 SET stock = $param_1, updated_at = $param_2 RETURN AFTER
// vars contains the record ID and parameters

// Delete specific records
record1 := models.NewRecordID("sessions", "abc123")
record2 := models.NewRecordID("sessions", "def456")
deleteQuery := surrealql.Delete(record1, record2)

sql, vars = deleteQuery.Build()
// Produces: DELETE $from_id_1, $from_id_2
```

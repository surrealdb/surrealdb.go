package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleUpdate_allInTable() {
	// Update all records in a table
	query := surrealql.Update("users").
		Set("active", true).
		Where("last_login < ?", time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE users SET active = $param_1 WHERE last_login < $param_2
	// Var param_1: true
	// Var param_2: 2022-10-01 00:00:00 +0000 UTC
}

func ExampleUpdateOnly() {
	// Update only one record in a table
	query := surrealql.UpdateOnly(surrealql.Thing("users", 123)).
		Set("name", "Alice")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: UPDATE ONLY $id_1 SET name = $param_1
	// Vars:
	//   id_1: users:123
	//   param_1: Alice
}

func ExampleUpdate_allInMultipleTables() {
	// Update all records in multiple tables
	query := surrealql.Update("users", "products").
		Set("active", true).
		Where("last_updated < ?", time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE users, products SET active = $param_1 WHERE last_updated < $param_2
	// Var param_1: true
	// Var param_2: 2022-10-01 00:00:00 +0000 UTC
}

func ExampleUpdate_specificRecord() {
	// Update a specific record by ID
	query := surrealql.Update("users:123").
		Set("name", "Jane Doe").
		Set("email", "jane.doe@example.com")
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}
	// Output:
	// SurrealQL: UPDATE users:123 SET name = $param_1, email = $param_2
	// Var param_1: Jane Doe
	// Var param_2: jane.doe@example.com
}

func ExampleUpdate_specificRecordsAcrossMultipleTables() {
	// Update specific records across multiple tables
	query := surrealql.Update("users:123", "products:456").
		Set("active", false).
		Where("last_login < ?", time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE users:123, products:456 SET active = $param_1 WHERE last_login < $param_2
	// Var param_1: false
	// Var param_2: 2022-10-01 00:00:00 +0000 UTC
}

func ExampleUpdate_thing() {
	// Update a record using a Thing
	query := surrealql.Update(surrealql.Thing("users", 123)).
		Set("name", "Alice Smith").
		Set("email", "alice.smith@example.com")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE $id_1 SET name = $param_1, email = $param_2
	// Var id_1: users:123
	// Var param_1: Alice Smith
	// Var param_2: alice.smith@example.com
}

func ExampleUpdate_table() {
	// Update records in a table using Table function
	query := surrealql.Update(surrealql.Table("users")).
		Set("active", true).
		Where("last_login < ?", time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE $table_1 SET active = $param_1 WHERE last_login < $param_2
	// Var param_1: true
	// Var param_2: 2022-10-01 00:00:00 +0000 UTC
	// Var table_1: users
}

func ExampleUpdate_thingAndTable() {
	// Update a record using a Thing and a Table
	query := surrealql.Update(
		surrealql.Expr(surrealql.Thing("users", 123)),
		surrealql.Expr(surrealql.Table("products")),
	).
		Set("name", "Bob").
		Set("email", "bob@example.com")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE $id_1, $table_1 SET name = $param_1, email = $param_2
	// Var id_1: users:123
	// Var param_1: Bob
	// Var param_2: bob@example.com
	// Var table_1: products
}

func ExampleUpdate_compoundOperations() {
	// UPDATE with compound operations using the Set function
	sql, vars := surrealql.Update("products").
		Set("stock -= ?", 5).                     // Decrement stock
		Set("sales_count += ?", 1).               // Increment sales counter
		Set("last_sold", "2024-01-01T00:00:00Z"). // Simple assignment
		Where("stock > ?", 0).
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPDATE products SET stock -= $param_1, sales_count += $param_2, last_sold = $param_3 WHERE stock > $param_4
	// Variables: map[param_1:5 param_2:1 param_3:2024-01-01T00:00:00Z param_4:0]
}

func ExampleUpdate_arrayOperations() {
	// UPDATE with array operations
	sql, vars := surrealql.Update("products:laptop").
		Set("tags += ?", []string{"featured", "sale"}). // Append to array
		Set("categories -= ?", "deprecated").           // Remove from array
		Set("stock", 100).                              // Simple assignment
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPDATE products:laptop SET tags += $param_1, categories -= $param_2, stock = $param_3
	// Variables: map[param_1:[featured sale] param_2:deprecated param_3:100]
}

func ExampleUpdate_returnNone() {
	// Use RETURN NONE for better performance when results aren't needed

	// Bulk update without returning results
	updateQuery := surrealql.Update("products").
		Set("on_sale", false).
		Where("sale_ends_at < ?", time.Now()).
		ReturnNone()

	// Bulk delete without returning results
	deleteQuery := surrealql.Delete("logs").
		Where("created_at < ?", time.Now().AddDate(0, -1, 0)).
		ReturnNone()

	fmt.Println("Update:", updateQuery.String())
	fmt.Println("Delete:", deleteQuery.String())

	// Output:
	// Update: UPDATE products SET on_sale = $param_1 WHERE sale_ends_at < $param_2 RETURN NONE
	// Delete: DELETE logs WHERE created_at < $param_1 RETURN NONE
}

func ExampleUpdate_withReturnDiff() {
	// Update user and return changes
	query := surrealql.Update("users:123").
		Set("name", "Jane Doe").
		Set("updated_at", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)).
		ReturnDiff()

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// SurrealQL: UPDATE users:123 SET name = $param_1, updated_at = $param_2 RETURN DIFF
	// Var param_1: Jane Doe
	// Var param_2: 2023-10-01 12:00:00 +0000 UTC
}

func ExampleUpdate_recordID() {
	// Update a record using its ID
	recordID := surrealql.Thing("users", 123)
	query := surrealql.Update(recordID).
		Set("name", "Alice").
		Set("email", "alice@example.com")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: UPDATE $id_1 SET name = $param_1, email = $param_2
	// Vars:
	//   id_1: users:123
	//   param_1: Alice
	//   param_2: alice@example.com
}

func ExampleUpdate_recordID_multi() {
	// Update multiple records using their IDs
	recordIDs := []*models.RecordID{
		surrealql.Thing("users", 123),
		surrealql.Thing("users", 456),
	}

	query := surrealql.Update(recordIDs...).
		Set("name", "Alice").
		Set("email", "alice@example.com")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: UPDATE $id_1, $id_2 SET name = $param_1, email = $param_2
	// Vars:
	//   id_1: users:123
	//   id_2: users:456
	//   param_1: Alice
	//   param_2: alice@example.com
}

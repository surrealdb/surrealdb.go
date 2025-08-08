package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
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
	// SurrealQL: UPDATE users SET active = $active_1 WHERE last_login < $param_1
	// Var active_1: true
	// Var param_1: 2022-10-01 00:00:00 +0000 UTC
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
	// SurrealQL: UPDATE users, products SET active = $active_1 WHERE last_updated < $param_1
	// Var active_1: true
	// Var param_1: 2022-10-01 00:00:00 +0000 UTC
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
	// SurrealQL: UPDATE users:123 SET email = $email_1, name = $name_1
	// Var email_1: jane.doe@example.com
	// Var name_1: Jane Doe
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
	// SurrealQL: UPDATE users:123, products:456 SET active = $active_1 WHERE last_login < $param_1
	// Var active_1: false
	// Var param_1: 2022-10-01 00:00:00 +0000 UTC
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
	// SurrealQL: UPDATE $id_1 SET email = $email_1, name = $name_1
	// Var email_1: alice.smith@example.com
	// Var id_1: {users 123}
	// Var name_1: Alice Smith
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
	// SurrealQL: UPDATE type::table($tb_1) SET active = $active_1 WHERE last_login < $param_1
	// Var active_1: true
	// Var param_1: 2022-10-01 00:00:00 +0000 UTC
	// Var tb_1: users
}

func ExampleUpdate_thingAndTable() {
	// Update a record using a Thing and a Table
	query := surrealql.Update(surrealql.Thing("users", 123), surrealql.Table("products")).
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
	// SurrealQL: UPDATE $id_1, type::table($tb_1) SET email = $email_1, name = $name_1
	// Var email_1: bob@example.com
	// Var id_1: {users 123}
	// Var name_1: Bob
	// Var tb_1: products
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
	// UPDATE products SET last_sold = $last_sold_1, stock -= $param_1, sales_count += $param_2 WHERE stock > $param_3
	// Variables: map[last_sold_1:2024-01-01T00:00:00Z param_1:5 param_2:1 param_3:0]
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
	// UPDATE products:laptop SET stock = $stock_1, tags += $param_1, categories -= $param_2
	// Variables: map[param_1:[featured sale] param_2:deprecated stock_1:100]
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
	// Update: UPDATE products SET on_sale = $on_sale_1 WHERE sale_ends_at < $param_1 RETURN NONE
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
	// SurrealQL: UPDATE users:123 SET name = $name_1, updated_at = $updated_at_1 RETURN DIFF
	// Var name_1: Jane Doe
	// Var updated_at_1: 2023-10-01 12:00:00 +0000 UTC
}

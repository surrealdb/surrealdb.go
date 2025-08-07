package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleInsert() {
	// Simple insert with data
	q := surrealql.Insert("company").Value(map[string]any{
		"name":    "SurrealDB",
		"founded": "2021-09-10",
		"tags":    []string{"big data", "database"},
	})

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// INSERT INTO company $insert_data_1
}

func ExampleInsert_valueArray() {
	// Insert multiple records with an array of data
	data := []map[string]any{
		{"name": "Company A"},
		{"name": "Company B"},
	}

	q := surrealql.Insert("company").Value(data)

	sql, vars := q.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// INSERT INTO company $insert_data_1
	// Var insert_data_1: [map[name:Company A] map[name:Company B]]
}

func ExampleInsert_valueQuery() {
	// Insert using a SELECT query
	selectQuery := surrealql.Select("name", "founded").
		FromTable("companies").
		Where("active = ?", true)

	q := surrealql.Insert("company").ValueQuery(selectQuery)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// INSERT INTO company (SELECT name, founded FROM companies WHERE active = $param_1)
}

func ExampleInsert_fields() {
	// Insert with fields and values
	q := surrealql.Insert("company").
		Fields("name", "founded").
		Values("SurrealDB", "2021-09-10").
		Values("Another Company", "2022-01-01")

	sql, vars := q.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// INSERT INTO company (name, founded) VALUES ($insert_0_0_1, $insert_0_1_1), ($insert_1_0_1, $insert_1_1_1)
	// Var insert_0_0_1: SurrealDB
	// Var insert_0_1_1: 2021-09-10
	// Var insert_1_0_1: Another Company
	// Var insert_1_1_1: 2022-01-01
}

func ExampleInsert_onDuplicate() {
	// Insert with ON DUPLICATE KEY UPDATE
	q := surrealql.Insert("user").
		Fields("id", "name", "email").
		Values("user:1", "John", "john@example.com").
		OnDuplicateKeyUpdateSet("name", "John Updated").
		OnDuplicateKeyUpdateSet("email", "john.updated@example.com")

	sql, vars := q.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// INSERT INTO user (id, name, email) VALUES ($insert_0_0_1, $insert_0_1_1, $insert_0_2_1) ON DUPLICATE KEY UPDATE email = $dup_email_1, name = $dup_name_1
	// Var dup_email_1: john.updated@example.com
	// Var dup_name_1: John Updated
	// Var insert_0_0_1: user:1
	// Var insert_0_1_1: John
	// Var insert_0_2_1: john@example.com
}

func ExampleInsert_onDuplicateRaw() {
	// Insert with ON DUPLICATE KEY UPDATE using raw SQL
	q := surrealql.Insert("user").
		Fields("id", "name", "times_updated", "last_edited").
		Values("user:1", "John", 0).
		Values("user:2", "Jane", 0).
		OnDuplicateKeyUpdateRaw("times_updated += 1").
		OnDuplicateKeyUpdateRaw("last_edited = time::now()")

	sql, vars := q.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// INSERT INTO user (id, name, times_updated, last_edited) VALUES ($insert_0_0_1, $insert_0_1_1, $insert_0_2_1), ($insert_1_0_1, $insert_1_1_1, $insert_1_2_1) ON DUPLICATE KEY UPDATE times_updated += 1, last_edited = time::now()
	// Var insert_0_0_1: user:1
	// Var insert_0_1_1: John
	// Var insert_0_2_1: 0
	// Var insert_1_0_1: user:2
	// Var insert_1_1_1: Jane
	// Var insert_1_2_1: 0
}

func ExampleInsert_returnOptions() {
	// Insert with different return options
	q1 := surrealql.Insert("person").Value(map[string]any{"name": "Alice"}).ReturnNone()
	q2 := surrealql.Insert("person").Value(map[string]any{"name": "Bob"}).ReturnAfter()
	q3 := surrealql.Insert("person").Value(map[string]any{"name": "Charlie"}).ReturnDiff()

	sql1, _ := q1.Build()
	sql2, _ := q2.Build()
	sql3, _ := q3.Build()

	fmt.Println(sql1)
	fmt.Println(sql2)
	fmt.Println(sql3)
	// Output:
	// INSERT INTO person $insert_data_1 RETURN NONE
	// INSERT INTO person $insert_data_1 RETURN AFTER
	// INSERT INTO person $insert_data_1 RETURN DIFF
}

func ExampleInsert_ignore() {
	// Insert with IGNORE flag
	q := surrealql.Insert("user").
		Ignore().
		Fields("id", "email").
		Values("user:1", "existing@example.com")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// INSERT IGNORE INTO user (id, email) VALUES ($insert_0_0_1, $insert_0_1_1)
}

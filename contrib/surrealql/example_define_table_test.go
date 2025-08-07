package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleDefineTable() {
	// Simple table definition
	q := surrealql.DefineTable("user")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE user
}

func ExampleDefineTable_changefeed() {
	// Table with changefeed
	q := surrealql.DefineTable("reading").Changefeed("3d")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE reading CHANGEFEED 3d
}

func ExampleDefineTable_changefeedDuration() {
	// Table with changefeed using duration
	q := surrealql.DefineTable("reading").ChangefeedDuration(72 * time.Hour)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE reading CHANGEFEED 72h0m0s
}

func ExampleDefineTable_changefeedWithOriginal() {
	// Table with changefeed including original
	q := surrealql.DefineTable("events").ChangefeedWithOriginal("7d")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE events CHANGEFEED 7d INCLUDE ORIGINAL
}

func ExampleDefineTable_changefeedDurationWithOriginal() {
	// Table with changefeed including original using duration
	q := surrealql.DefineTable("events").ChangefeedDurationWithOriginal(24 * time.Hour)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE events CHANGEFEED 24h0m0s INCLUDE ORIGINAL
}

func ExampleDefineTable_schemafull() {
	// Schemafull table with changefeed
	q := surrealql.DefineTable("product").
		Schemafull().
		Changefeed("30d")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE product SCHEMAFULL CHANGEFEED 30d
}

func ExampleDefineTable_permissions() {
	// Table with permissions
	q := surrealql.DefineTable("secure_data").
		Permissions("select", "WHERE user = $auth.id").
		Permissions("create", "WHERE user = $auth.id").
		Permissions("update", "NONE").
		Permissions("delete", "NONE")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE TABLE secure_data PERMISSIONS SELECT WHERE user = $auth.id CREATE WHERE user = $auth.id UPDATE NONE DELETE NONE
}

func ExampleDefineField() {
	// Define a field with type and default
	q := surrealql.DefineField("email", "user").
		Type("string").
		Assert("$value != NONE AND string::is::email($value)").
		Default("\"no-email@example.com\"")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE FIELD email ON TABLE user TYPE string ASSERT $value != NONE AND string::is::email($value) DEFAULT "no-email@example.com"
}

func ExampleDefineField_value() {
	// Define a computed field
	q := surrealql.DefineField("full_name", "person").
		Type("string").
		Value("string::concat(first_name, ' ', last_name)")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// DEFINE FIELD full_name ON TABLE person TYPE string VALUE string::concat(first_name, ' ', last_name)
}

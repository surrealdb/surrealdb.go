package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleShowChangesForTable_sinceRawDate() {
	// Show changes from a specific timestamp
	q := surrealql.ShowChangesForTable("reading").
		Since("d\"2023-09-07T01:23:52Z\"").
		Limit(10)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// SHOW CHANGES FOR TABLE reading SINCE d"2023-09-07T01:23:52Z" LIMIT 10
}

func ExampleShowChangesForTable_sinceTime() {
	// Show changes since a specific time
	since := time.Date(2023, 9, 7, 1, 23, 52, 0, time.UTC)
	q := surrealql.ShowChangesForTable("reading").
		SinceTime(&since).
		Limit(10)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// SHOW CHANGES FOR TABLE reading SINCE d"2023-09-07T01:23:52Z" LIMIT 10
}

func ExampleShowChangesForTable_sinceRawVersionstamp() {
	// Show changes from version 0
	q := surrealql.ShowChangesForTable("events").
		Since("0").
		Limit(50)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// SHOW CHANGES FOR TABLE events SINCE 0 LIMIT 50
}

func ExampleShowChangesForTable_sinceVersionstamp() {
	// Show changes since a specific versionstamp
	// Note: If you pass a versionstamp from a previous SHOW CHANGES result (e.g., 6553600),
	// this method automatically adjusts it by shifting right 16 bits (to 100 in this case)
	// to get the logical version for the SINCE clause
	q := surrealql.ShowChangesForTable("events").
		SinceVersionstamp(6553600). // This will be automatically adjusted to 100
		Limit(50)

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// SHOW CHANGES FOR TABLE events SINCE 100 LIMIT 50
}

func ExampleShowChangesForTable_noLimit() {
	// Show all changes since a version
	q := surrealql.ShowChangesForTable("users").Since("100")

	sql, _ := q.Build()
	fmt.Println(sql)
	// Output:
	// SHOW CHANGES FOR TABLE users SINCE 100
}

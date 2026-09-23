package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_recordIDRange shows how to select a range of records.
//
// Vars print via Range.String for debugging; that text is not SurrealQL.
func ExampleSelect_recordIDRange() {
	rr := models.RecordID{
		Table: "person",
		ID:    surrealql.RangeBeginInclusiveEndInclusive(1, 1000),
	}
	sql, vars := surrealql.Select(rr).Build()

	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person 1..=1000}
}

// ExampleSelect_recordIDRange_open shows open-ended ranges.
//
// Use RangeOpenBegin* / RangeOpenEnd* when one side has no limit, or
// RangeOpen() for both sides open. The Build output is a parameterized SELECT
// plus the Go bounds in vars — not a SurrealQL literal.
//
// Do not pass models.None as a scalar begin/end — SurrealDB rejects NONE as a
// scalar record id. Use None inside array-style ids
// (ExampleSelect_recordIDRange_compositeID). Live proof:
// ExampleSelect_recordRange_omitVsNone_begin and
// ExampleSelect_recordRange_omitVsNone_end in this package (and the matching
// Select Examples in package surrealdb).
func ExampleSelect_recordIDRange_open() {
	cases := []models.RecordID{
		{Table: "person", ID: surrealql.RangeOpenBeginInclusive(1)},
		{Table: "person", ID: surrealql.RangeOpenEndInclusive(10)},
		{Table: "person", ID: surrealql.RangeOpenEndExclusive(10)},
		{Table: "person", ID: surrealql.RangeOpen()},
	}
	for _, rr := range cases {
		sql, vars := surrealql.Select(rr).Build()
		fmt.Println(sql)
		dumpVars(vars)
	}
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person 1..}
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person ..=10}
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person ..10}
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person ..}
}

// ExampleSelect_recordIDRange_nullBound shows NULL as a bound value.
//
// RangeBeginInclusiveEndInclusive(1, nil) stores NULL on the end side. That is not "leave the side
// open" — use RangeOpenBeginInclusive when the end has no limit.
func ExampleSelect_recordIDRange_nullBound() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "person",
		ID:    surrealql.RangeBeginInclusiveEndInclusive(1, nil),
	}).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person 1..=<nil>}
}

// ExampleSelect_recordIDRange_nestedOpenRange shows open `..` as a bound value.
func ExampleSelect_recordIDRange_nestedOpenRange() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "person",
		ID:    surrealql.RangeBeginInclusiveEndInclusive(1, surrealql.RangeOpen()),
	}).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {person 1..=..}
}

// ExampleSelect_recordIDRange_compositeID shows a range over array-style record ids.
//
// Array ids cannot omit a part, so London ranges write NONE and `..` as values
// inside []any{...}. This is the supported place for models.None — not as a
// scalar outer bound like person:NONE..=10 (that form is rejected live; leave
// the side unset instead).
func ExampleSelect_recordIDRange_compositeID() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "temp",
		ID: surrealql.RangeBeginInclusiveEndInclusive(
			[]any{"London", models.None},
			[]any{"London", surrealql.RangeOpen()},
		),
	}).Build()
	fmt.Println(sql)
	dumpVars(vars)

	sql, vars = surrealql.Select(models.RecordID{
		Table: "temp",
		ID: surrealql.RangeBeginInclusiveEndInclusive(
			[]any{"London", models.None, models.None},
			[]any{"London", surrealql.RangeOpen(), surrealql.RangeOpen()},
		),
	}).Build()
	fmt.Println(sql)
	dumpVars(vars)

	sql, vars = surrealql.Select(models.RecordID{
		Table: "temp",
		ID: surrealql.RangeBeginInclusiveEndInclusive(
			[]any{"London", "West", models.None},
			[]any{"London", "West", surrealql.RangeOpen()},
		),
	}).Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {temp [London {}]..=[London ..]}
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {temp [London {} {}]..=[London .. ..]}
	// SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {temp [London West {}]..=[London West ..]}
}

package surrealql_test

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// formatBound describes a range begin/end for Example output.
// It intentionally avoids Range.String() / dumpVars %v, which look like
// SurrealQL but are not guaranteed to be valid SurrealQL.
func formatBound(v any) (kind, inner string) {
	if v == nil {
		return "", ""
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", ""
		}
		rv = rv.Elem()
	}
	name := rv.Type().Name()
	field := rv.FieldByName("Value")
	if !field.IsValid() {
		return "", fmt.Sprintf("%T(%v)", v, v)
	}
	inner = formatBoundValue(field.Interface())
	switch {
	case strings.HasPrefix(name, "BoundIncluded"):
		return "inclusive", inner
	case strings.HasPrefix(name, "BoundExcluded"):
		return "exclusive", inner
	default:
		return "", fmt.Sprintf("%T(%v)", v, v)
	}
}

func formatBoundValue(v any) string {
	if v == nil {
		return "nil"
	}
	switch x := v.(type) {
	case models.CustomNil:
		return "None"
	case models.RecordID:
		if isModelsRange(x.ID) {
			return fmt.Sprintf("RecordID{Table:%q, ID:%s}", x.Table, formatRange(x.ID))
		}
		return fmt.Sprintf("RecordID{Table:%q, ID:%s}", x.Table, formatBoundValue(x.ID))
	case string:
		return x
	default:
		if isModelsRange(v) {
			return formatRange(v)
		}
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice {
			parts := make([]string, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				parts[i] = formatBoundValue(rv.Index(i).Interface())
			}
			return "[]any{" + strings.Join(parts, ", ") + "}"
		}
		return fmt.Sprintf("%#v", v)
	}
}

func isModelsRange(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	name := rv.Type().Name()
	return (name == "Range" || strings.HasPrefix(name, "Range[")) &&
		rv.FieldByName("Begin").IsValid() && rv.FieldByName("End").IsValid()
}

func formatRange(v any) string {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	beginKind, beginInner := formatBound(rv.FieldByName("Begin").Interface())
	endKind, endInner := formatBound(rv.FieldByName("End").Interface())

	key := beginKind + "|" + endKind
	switch key {
	case "|":
		return "RangeOpen()"
	case "inclusive|":
		return "RangeOpenBeginInclusive(" + beginInner + ")"
	case "exclusive|":
		return "RangeOpenBeginExclusive(" + beginInner + ")"
	case "|inclusive":
		return "RangeOpenEndInclusive(" + endInner + ")"
	case "|exclusive":
		return "RangeOpenEndExclusive(" + endInner + ")"
	case "inclusive|inclusive":
		return "RangeClosed(" + beginInner + ", " + endInner + ")"
	case "inclusive|exclusive":
		return "RangeClosedEndExclusive(" + beginInner + ", " + endInner + ")"
	default:
		return fmt.Sprintf("Range{Begin:%s(%s), End:%s(%s)}", beginKind, beginInner, endKind, endInner)
	}
}

// printRecordRangeBuild shows the parameterized SurrealQL and the Go values
// bound into vars — what the SDK actually sends.
func printRecordRangeBuild(sql string, vars map[string]any) {
	fmt.Printf("SurrealQL: %s\n", sql)
	keys := slices.Collect(maps.Keys(vars))
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %s\n", key, formatBoundValue(vars[key]))
	}
}

// ExampleSelect_recordIDRange shows how to select a range of records.
func ExampleSelect_recordIDRange() {
	rr := models.RecordID{
		Table: "person",
		ID:    surrealql.RangeClosed(1, 1000),
	}
	sql, vars := surrealql.Select(rr).Build()

	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeClosed(1, 1000)}
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
		printRecordRangeBuild(sql, vars)
	}
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeOpenBeginInclusive(1)}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeOpenEndInclusive(10)}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeOpenEndExclusive(10)}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeOpen()}
}

// ExampleSelect_recordIDRange_nullBound shows NULL as a bound value.
//
// RangeClosed(1, nil) stores NULL on the end side. That is not "leave the side
// open" — use RangeOpenBeginInclusive when the end has no limit.
func ExampleSelect_recordIDRange_nullBound() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "person",
		ID:    surrealql.RangeClosed(1, nil),
	}).Build()
	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeClosed(1, nil)}
}

// ExampleSelect_recordIDRange_nestedOpenRange shows open `..` as a bound value.
func ExampleSelect_recordIDRange_nestedOpenRange() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "person",
		ID:    surrealql.RangeClosed(1, surrealql.RangeOpen()),
	}).Build()
	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:RangeClosed(1, RangeOpen())}
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
		ID: surrealql.RangeClosed(
			[]any{"London", models.None},
			[]any{"London", surrealql.RangeOpen()},
		),
	}).Build()
	printRecordRangeBuild(sql, vars)

	sql, vars = surrealql.Select(models.RecordID{
		Table: "temp",
		ID: surrealql.RangeClosed(
			[]any{"London", models.None, models.None},
			[]any{"London", surrealql.RangeOpen(), surrealql.RangeOpen()},
		),
	}).Build()
	printRecordRangeBuild(sql, vars)

	sql, vars = surrealql.Select(models.RecordID{
		Table: "temp",
		ID: surrealql.RangeClosed(
			[]any{"London", "West", models.None},
			[]any{"London", "West", surrealql.RangeOpen()},
		),
	}).Build()
	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"temp", ID:RangeClosed([]any{London, None}, []any{London, RangeOpen()})}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"temp", ID:RangeClosed([]any{London, None, None}, []any{London, RangeOpen(), RangeOpen()})}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"temp", ID:RangeClosed([]any{London, West, None}, []any{London, West, RangeOpen()})}
}

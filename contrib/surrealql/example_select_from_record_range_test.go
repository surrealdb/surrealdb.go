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

// formatBound describes a range begin/end as an OpenRange builder call.
// Avoids RangeValue.String() / dumpVars %v, which look like SurrealQL but are
// not guaranteed to be valid SurrealQL.
func formatBound(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	name := rv.Type().Name()
	field := rv.FieldByName("Value")
	if !field.IsValid() {
		return fmt.Sprintf("%T(%v)", v, v)
	}
	inner := formatBoundValue(field.Interface())
	switch {
	case strings.HasPrefix(name, "BoundIncluded"):
		return "Inclusive(" + inner + ")"
	case strings.HasPrefix(name, "BoundExcluded"):
		return "Exclusive(" + inner + ")"
	default:
		return fmt.Sprintf("%T(%v)", v, v)
	}
}

func formatBoundValue(v any) string {
	if v == nil {
		return "nil"
	}
	switch x := v.(type) {
	case models.CustomNil:
		return "None"
	case models.RangeValue:
		return formatRangeValue(x)
	case models.RecordID:
		id, ok := x.ID.(models.RangeValue)
		if ok {
			return fmt.Sprintf("RecordID{Table:%q, ID:%s}", x.Table, formatRangeValue(id))
		}
		return fmt.Sprintf("RecordID{Table:%q, ID:%s}", x.Table, formatBoundValue(x.ID))
	case string:
		return x
	default:
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

func formatRangeValue(r models.RangeValue) string {
	s := "OpenRange()"
	if r.Begin != nil {
		b := formatBound(r.Begin)
		switch {
		case strings.HasPrefix(b, "Inclusive("):
			s += ".BeginInclusive(" + strings.TrimSuffix(strings.TrimPrefix(b, "Inclusive("), ")") + ")"
		case strings.HasPrefix(b, "Exclusive("):
			s += ".BeginExclusive(" + strings.TrimSuffix(strings.TrimPrefix(b, "Exclusive("), ")") + ")"
		}
	}
	if r.End != nil {
		e := formatBound(r.End)
		switch {
		case strings.HasPrefix(e, "Inclusive("):
			s += ".EndInclusive(" + strings.TrimSuffix(strings.TrimPrefix(e, "Inclusive("), ")") + ")"
		case strings.HasPrefix(e, "Exclusive("):
			s += ".EndExclusive(" + strings.TrimSuffix(strings.TrimPrefix(e, "Exclusive("), ")") + ")"
		}
	}
	return s
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
		ID:    models.OpenRange().BeginInclusive(1).EndInclusive(1000),
	}
	sql, vars := surrealql.Select(rr).Build()

	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange().BeginInclusive(1).EndInclusive(1000)}
}

// ExampleSelect_recordIDRange_open shows open-ended ranges by leaving a side unset.
//
// Start from OpenRange() and chain Begin*/End* methods. The Build output is a
// parameterized SELECT plus the Go bounds in vars — not a SurrealQL literal.
//
// Do not pass models.None as a scalar begin/end — SurrealDB rejects NONE as a
// scalar record id. Use None inside array-style ids
// (ExampleSelect_recordIDRange_compositeID). Live proof:
// ExampleSelect_recordRange_omitVsNone_begin and
// ExampleSelect_recordRange_omitVsNone_end in this package (and the matching
// Select Examples in package surrealdb).
func ExampleSelect_recordIDRange_open() {
	cases := []models.RecordID{
		{Table: "person", ID: models.OpenRange().BeginInclusive(1)},
		{Table: "person", ID: models.OpenRange().EndInclusive(10)},
		{Table: "person", ID: models.OpenRange().EndExclusive(10)},
		{Table: "person", ID: models.OpenRange()},
	}
	for _, rr := range cases {
		sql, vars := surrealql.Select(rr).Build()
		printRecordRangeBuild(sql, vars)
	}
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange().BeginInclusive(1)}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange().EndInclusive(10)}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange().EndExclusive(10)}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange()}
}

// ExampleSelect_recordIDRange_nullBound shows NULL as a bound value.
//
// EndInclusive(nil) is NULL on that side — not an open delimiter. Leave End
// unset when the side should have no limit.
func ExampleSelect_recordIDRange_nullBound() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "person",
		ID:    models.OpenRange().BeginInclusive(1).EndInclusive(nil),
	}).Build()
	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange().BeginInclusive(1).EndInclusive(nil)}
}

// ExampleSelect_recordIDRange_nestedOpenRange shows open `..` as a bound value.
func ExampleSelect_recordIDRange_nestedOpenRange() {
	sql, vars := surrealql.Select(models.RecordID{
		Table: "person",
		ID:    models.OpenRange().BeginInclusive(1).EndInclusive(models.OpenRange()),
	}).Build()
	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"person", ID:OpenRange().BeginInclusive(1).EndInclusive(OpenRange())}
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
		ID: models.OpenRange().
			BeginInclusive([]any{"London", models.None}).
			EndInclusive([]any{"London", models.OpenRange()}),
	}).Build()
	printRecordRangeBuild(sql, vars)

	sql, vars = surrealql.Select(models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive([]any{"London", models.None, models.None}).
			EndInclusive([]any{"London", models.OpenRange(), models.OpenRange()}),
	}).Build()
	printRecordRangeBuild(sql, vars)

	sql, vars = surrealql.Select(models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive([]any{"London", "West", models.None}).
			EndInclusive([]any{"London", "West", models.OpenRange()}),
	}).Build()
	printRecordRangeBuild(sql, vars)
	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"temp", ID:OpenRange().BeginInclusive([]any{London, None}).EndInclusive([]any{London, OpenRange()})}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"temp", ID:OpenRange().BeginInclusive([]any{London, None, None}).EndInclusive([]any{London, OpenRange(), OpenRange()})}
	// SurrealQL: SELECT * FROM $from_id_1
	// Var from_id_1: RecordID{Table:"temp", ID:OpenRange().BeginInclusive([]any{London, West, None}).EndInclusive([]any{London, West, OpenRange()})}
}

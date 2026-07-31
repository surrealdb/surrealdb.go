package models_test

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// formatBound describes a range begin/end as an OpenRange builder call.
// It intentionally avoids RangeValue.String() / %v, which look like SurrealQL
// but are not guaranteed to be valid SurrealQL.
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

func printRecordIDRange(rr models.RecordID) {
	id, ok := rr.ID.(models.RangeValue)
	if !ok {
		fmt.Printf("table=%q id=%s\n", rr.Table, formatBoundValue(rr.ID))
		return
	}
	fmt.Printf("table=%q id=%s\n", rr.Table, formatRangeValue(id))
}

// ExampleOpenRange shows open-ended ranges by leaving a side unset.
//
// Start from OpenRange() and chain Begin*/End* methods. An unset side means
// that side has no limit. The values below are the Go bounds the SDK stores —
// not a SurrealQL string.
//
// Do not pass models.None as a scalar begin/end — SurrealDB rejects NONE as a
// scalar record id. Use None inside array-style ids (see ExampleOpenRange_arrayID).
func ExampleOpenRange() {
	printRecordIDRange(models.RecordID{Table: "person", ID: models.OpenRange().BeginInclusive(1)})
	printRecordIDRange(models.RecordID{Table: "person", ID: models.OpenRange().EndInclusive(10)})
	printRecordIDRange(models.RecordID{Table: "person", ID: models.OpenRange().EndExclusive(10)})
	printRecordIDRange(models.RecordID{Table: "person", ID: models.OpenRange()})
	// Output:
	// table="person" id=OpenRange().BeginInclusive(1)
	// table="person" id=OpenRange().EndInclusive(10)
	// table="person" id=OpenRange().EndExclusive(10)
	// table="person" id=OpenRange()
}

// ExampleOpenRange_nullBound shows NULL as a bound value.
//
// EndInclusive(nil) stores NULL on that side. That is not "leave the side
// open" — leave End unset when the side has no limit.
func ExampleOpenRange_nullBound() {
	printRecordIDRange(models.RecordID{
		Table: "person",
		ID:    models.OpenRange().BeginInclusive(1).EndInclusive(nil),
	})
	// Output:
	// table="person" id=OpenRange().BeginInclusive(1).EndInclusive(nil)
}

// ExampleOpenRange_nested shows an open `..` used as a bound value.
//
// Bare OpenRange() is the `..` value (both sides open). Prefer it where a
// value must appear, such as the high end of an array-style id.
func ExampleOpenRange_nested() {
	printRecordIDRange(models.RecordID{
		Table: "person",
		ID:    models.OpenRange().BeginInclusive(1).EndInclusive(models.OpenRange()),
	})
	fmt.Printf("%s\n", formatRangeValue(models.OpenRange()))
	// Output:
	// table="person" id=OpenRange().BeginInclusive(1).EndInclusive(OpenRange())
	// OpenRange()
}

// ExampleOpenRange_arrayID shows why array-style ids write NONE and `..`
// instead of omitting a part.
//
// An array id always has a fixed number of parts, so you cannot leave a part
// blank. Use models.None (low sentinel) and OpenRange() (high open end)
// inside []any{...} — not as a scalar outer bound.
func ExampleOpenRange_arrayID() {
	printRecordIDRange(models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive([]any{"London", models.None}).
			EndInclusive([]any{"London", models.OpenRange()}),
	})
	printRecordIDRange(models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive([]any{"London", models.None, models.None}).
			EndInclusive([]any{"London", models.OpenRange(), models.OpenRange()}),
	})
	printRecordIDRange(models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive([]any{"London", "West", models.None}).
			EndInclusive([]any{"London", "West", models.OpenRange()}),
	})
	// Output:
	// table="temp" id=OpenRange().BeginInclusive([]any{London, None}).EndInclusive([]any{London, OpenRange()})
	// table="temp" id=OpenRange().BeginInclusive([]any{London, None, None}).EndInclusive([]any{London, OpenRange(), OpenRange()})
	// table="temp" id=OpenRange().BeginInclusive([]any{London, West, None}).EndInclusive([]any{London, West, OpenRange()})
}

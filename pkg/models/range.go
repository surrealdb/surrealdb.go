package models

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

type BoundIncluded[T any] struct {
	Value T
}

func (bi *BoundIncluded[T]) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagBoundIncluded,
		Content: bi.Value,
	})
}

func (bi *BoundIncluded[T]) UnmarshalCBOR(data []byte) error {
	data, err := getTaggedContent(data, TagBoundIncluded)
	if err != nil {
		return fmt.Errorf("BoundIncluded: %w", err)
	}

	var temp T
	if err := cbor.Unmarshal(data, &temp); err != nil {
		return err
	}

	bi.Value = temp
	return nil
}

type BoundExcluded[T any] struct {
	Value T
}

func (be *BoundExcluded[T]) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagBoundExcluded,
		Content: be.Value,
	})
}

func (be *BoundExcluded[T]) UnmarshalCBOR(data []byte) error {
	data, err := getTaggedContent(data, TagBoundExcluded)
	if err != nil {
		return fmt.Errorf("BoundExcluded: %w", err)
	}

	var temp T
	if err := cbor.Unmarshal(data, &temp); err != nil {
		return err
	}

	be.Value = temp
	return nil
}

type Bound[T any] interface {
	BoundIncluded[T] | BoundExcluded[T]
}

// Included marks the start or end of a range as inclusive.
//
//	Included(1)    // like 1 in person:1..=10
//	Included("a")  // like a in users:a..=z
//
// For several parts (an array-style record id), use [IncludedMany].
func Included[T any](v T) *BoundIncluded[T] {
	return &BoundIncluded[T]{Value: v}
}

// IncludedMany marks an inclusive bound whose value is an array of parts.
//
//	IncludedMany(1, 2)                      // [1, 2]
//	IncludedMany[any]("London", None)         // ['London', NONE]
//	IncludedMany[any]("London", OpenRange())  // ['London', ..]
func IncludedMany[T any](first T, rest ...T) *BoundIncluded[[]T] {
	all := make([]T, 0, 1+len(rest))
	all = append(all, first)
	all = append(all, rest...)
	return &BoundIncluded[[]T]{Value: all}
}

// Excluded marks the start or end of a range as exclusive.
//
// For several parts (an array-style record id), use [ExcludedMany].
func Excluded[T any](v T) *BoundExcluded[T] {
	return &BoundExcluded[T]{Value: v}
}

// ExcludedMany marks an exclusive bound whose value is an array of parts.
//
// See [IncludedMany] for examples.
func ExcludedMany[T any](first T, rest ...T) *BoundExcluded[[]T] {
	all := make([]T, 0, 1+len(rest))
	all = append(all, first)
	all = append(all, rest...)
	return &BoundExcluded[[]T]{Value: all}
}

// ID joins parts into an array-style record id.
//
// Prefer [IncludedMany] / [ExcludedMany] for range bounds.
// Use ID when you need the array itself, for example:
//
//	NewRecordID("temp", ID("London", 1))
func ID(parts ...any) []any {
	return parts
}

type Range[T any, TBeg Bound[T], TEnd Bound[T]] struct {
	Begin *TBeg
	End   *TEnd
}

func (r *Range[T, TBeg, TEnd]) GetJoinString() string {
	return joinFromBounds(r.Begin, r.End)
}

func (r *Range[T, TBeg, TEnd]) String() string {
	joinStr := r.GetJoinString()
	beginStr := ""
	endStr := ""

	if r.Begin != nil {
		beginStr = convertToString(r.Begin)
	}
	if r.End != nil {
		endStr = convertToString(r.End)
	}

	return fmt.Sprintf("%s%s%s", beginStr, joinStr, endStr)
}

func (r *Range[T, TBeg, TEnd]) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagRange,
		Content: []interface{}{r.Begin, r.End},
	})
}

func (r *Range[T, TBeg, TEnd]) UnmarshalCBOR(data []byte) error {
	data, err := getTaggedContent(data, TagRange)
	if err != nil {
		return fmt.Errorf("Range: %w", err)
	}

	var temp [2]cbor.RawTag
	if err := cbor.Unmarshal(data, &temp); err != nil {
		return err
	}

	var begin TBeg
	beginEnc, _ := temp[0].MarshalCBOR()
	_ = cbor.Unmarshal(beginEnc, &begin)

	var end TEnd
	endEnc, _ := temp[1].MarshalCBOR()
	_ = cbor.Unmarshal(endEnc, &end)

	r.Begin = &begin
	r.End = &end
	return nil
}

type RecordRangeID[T any, TBeg Bound[T], TEnd Bound[T]] struct {
	Range[T, TBeg, TEnd]
	Table Table
}

func (rr *RecordRangeID[T, TBeg, TEnd]) String() string {
	joinStr := rr.GetJoinString()
	beginStr := ""
	endStr := ""

	if rr.Begin != nil {
		beginStr = convertToString(rr.Begin)
	}
	if rr.End != nil {
		endStr = convertToString(rr.End)
	}

	return fmt.Sprintf("%s:%s%s%s", rr.Table, beginStr, joinStr, endStr)
}

func (rr RecordRangeID[T, TBeg, TEnd]) MarshalCBOR() ([]byte, error) {
	return NewRecordRange(string(rr.Table), rr.Begin, rr.End).MarshalCBOR()
}

// RecordRange is a range of record ids in one table, such as person:1..=1000
// or temp:['London', NONE]..=['London', ..].
//
// Set Begin and End with [Included], [Excluded], or nil when that side has no limit.
//
// Pass a RecordRange to surrealdb.Select, Delete, Update, and similar methods,
// or use it as a query variable. For everyday use prefer RecordRange over the
// more typed [RecordRangeID].
type RecordRange struct {
	Table Table
	Begin any
	End   any
}

// NewRecordRange creates a [RecordRange] for the given table and bounds.
//
// This matches SurrealQL like:
//
//	SELECT * FROM temp:['London', NONE]..=['London', ..]
//
//	models.NewRecordRange("temp",
//	    models.IncludedMany[any]("London", models.None),
//	    models.IncludedMany[any]("London", models.OpenRange()),
//	)
func NewRecordRange(table string, begin, end any) RecordRange {
	return RecordRange{Table: Table(table), Begin: begin, End: end}
}

// OpenRange is an open-ended range (`..`), often used as the upper part of an
// array-style record id, as in ['London', ..].
func OpenRange() RangeValue {
	return RangeValue{}
}

// RangeValue is a plain range value, such as 1..=10 or an open `..`.
// Use the typed [Range] when both ends share one Go type; use RangeValue
// (or [OpenRange]) when the range sits inside a mixed value like an array id.
type RangeValue struct {
	Begin any
	End   any
}

func (r RangeValue) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagRange,
		Content: []any{r.Begin, r.End},
	})
}

func (r RangeValue) String() string {
	return fmt.Sprintf("%s%s%s", boundValueString(r.Begin), joinFromBounds(r.Begin, r.End), boundValueString(r.End))
}

func (rr RecordRange) MarshalCBOR() ([]byte, error) {
	if rr.Table == "" {
		return nil, fmt.Errorf("cannot marshal RecordRange with empty table")
	}
	return cbor.Marshal(cbor.Tag{
		Number: TagRecordID,
		Content: []any{
			string(rr.Table),
			RangeValue{Begin: rr.Begin, End: rr.End},
		},
	})
}

func (rr RecordRange) String() string {
	return fmt.Sprintf("%s:%s%s%s",
		rr.Table,
		boundValueString(rr.Begin),
		joinFromBounds(rr.Begin, rr.End),
		boundValueString(rr.End),
	)
}

// joinFromBounds returns the SurrealQL join between two bounds: .., ..=, >.., or >..=.
func joinFromBounds(begin, end any) string {
	joinStr := ""
	if begin != nil && !isNilBound(begin) && isExcludedBound(begin) {
		joinStr += ">"
	}
	joinStr += ".."
	if end != nil && !isNilBound(end) && isIncludedBound(end) {
		joinStr += "="
	}
	return joinStr
}

func isNilBound(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) && rv.IsNil()
}

func isExcludedBound(v any) bool {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	name := t.Name()
	return name == "BoundExcluded" || strings.HasPrefix(name, "BoundExcluded[")
}

func isIncludedBound(v any) bool {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	name := t.Name()
	return name == "BoundIncluded" || strings.HasPrefix(name, "BoundIncluded[")
}

func boundValueString(v any) string {
	if isNilBound(v) {
		return ""
	}
	return convertToString(v)
}

// convertToString renders the underlying value of a range bound (a
// *BoundIncluded[T] or *BoundExcluded[T]) as a SurrealQL-compatible string.
//
// String values are escaped the same way RecordID.String escapes record IDs
// (see record_id_string.go), wrapping them in angle brackets (⟨⟩) when they
// contain characters that would otherwise be ambiguous or invalid in a bare
// identifier. Other value types fall back to their default string
// representation.
func convertToString(v any) string {
	value := reflect.ValueOf(v)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}

	// v is expected to be a *BoundIncluded[T] or *BoundExcluded[T], both of
	// which have a single exported field named "Value" holding the bound.
	field := value.FieldByName("Value")
	if !field.IsValid() {
		return fmt.Sprintf("%v", v)
	}
	inner := field.Interface()

	if strVal, ok := inner.(string); ok {
		if needsEscaping(strVal) {
			return fmt.Sprintf("⟨%s⟩", escapeString(strVal, '⟩'))
		}
		return strVal
	}

	return fmt.Sprintf("%v", inner)
}

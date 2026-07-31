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
	rid := RecordID{
		Table: string(rr.Table),
		ID:    RangeValue{Begin: rr.Begin, End: rr.End},
	}
	return rid.MarshalCBOR()
}

// OpenRange is an open-ended range (`..`). Both sides start open; chain
// BeginInclusive / BeginExclusive / EndInclusive / EndExclusive to set bounds.
//
// Bare OpenRange() is the `..` value itself — useful inside array-style ids,
// as in ['London', ..].
//
// For a range of record ids, put the result in a [RecordID]:
//
//	models.RecordID{
//		Table: "person",
//		ID:    models.OpenRange().BeginInclusive(1).EndInclusive(10),
//	}
//
// Leave a side unset for an open bound (person:1.. or person:..=10). Do not
// pass [None] as a scalar begin/end — SurrealDB rejects that. Use [None]
// inside array-style ids, for example []any{"London", None}.
func OpenRange() RangeValue {
	return RangeValue{}
}

// RangeValue is a plain range value, such as 1..=10 or an open `..`.
// Prefer building it with [OpenRange] and the Begin*/End* methods.
// Use the typed [Range] when both ends share one Go type.
type RangeValue struct {
	Begin any
	End   any
}

// BeginInclusive sets an inclusive lower bound and returns the updated range.
func (r RangeValue) BeginInclusive(v any) RangeValue {
	r.Begin = &BoundIncluded[any]{Value: v}
	return r
}

// BeginExclusive sets an exclusive lower bound and returns the updated range.
func (r RangeValue) BeginExclusive(v any) RangeValue {
	r.Begin = &BoundExcluded[any]{Value: v}
	return r
}

// EndInclusive sets an inclusive upper bound and returns the updated range.
func (r RangeValue) EndInclusive(v any) RangeValue {
	r.End = &BoundIncluded[any]{Value: v}
	return r
}

// EndExclusive sets an exclusive upper bound and returns the updated range.
func (r RangeValue) EndExclusive(v any) RangeValue {
	r.End = &BoundExcluded[any]{Value: v}
	return r
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

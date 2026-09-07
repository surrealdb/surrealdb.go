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

// Range is a SurrealQL range such as 1..=10 or an open `..`.
//
// A nil Begin or End means that side is open (no limit). Do not pass [None]
// as a scalar begin/end — SurrealDB rejects that. Use [None] inside
// array-style ids, for example []any{"London", None}.
//
// For convenient constructors (open and closed forms), use the helpers in
// github.com/surrealdb/surrealdb.go/contrib/surrealql.
type Range[T any, TBeg Bound[T], TEnd Bound[T]] struct {
	Begin *TBeg
	End   *TEnd
}

// GetJoinString returns the join between the bounds: "..", "..=", ">..", or ">..=".
// Nil Begin/End are treated as open sides and do not panic.
func (r Range[T, TBeg, TEnd]) GetJoinString() string {
	return joinFromBounds(r.Begin, r.End)
}

// String returns a debug rendering of the range (for example "1..=10" or "..").
// It is not guaranteed to be valid SurrealQL; do not send it as a query.
// Open sides (nil Begin/End) render as empty, so both-open is "..".
func (r Range[T, TBeg, TEnd]) String() string {
	return fmt.Sprintf("%s%s%s", boundValueString(r.Begin), r.GetJoinString(), boundValueString(r.End))
}

func (r Range[T, TBeg, TEnd]) MarshalCBOR() ([]byte, error) {
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

func (rr RecordRangeID[T, TBeg, TEnd]) String() string {
	return fmt.Sprintf("%s:%s", rr.Table, rr.Range.String())
}

func (rr RecordRangeID[T, TBeg, TEnd]) MarshalCBOR() ([]byte, error) {
	rid := RecordID{
		Table: string(rr.Table),
		ID:    rr.Range,
	}
	return rid.MarshalCBOR()
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

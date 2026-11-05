package surrealcbor

import (
	"fmt"
	"reflect"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// maxKindHeadLength is the most a tag and a text head can take before the text.
const maxKindHeadLength = 3 + 9

// decodeCBOR2KindTag decodes a date, time, timestamp, interval, bigdecimal or
// regex (tags 100 and 189 to 193, which only cbor.2 carries) whose tagged item
// starts at start. The tag number has been read.
//
// Into an interface it gives the models type. Into any other Go type it
// decodes the form plain cbor carries the kind in, widened as the server
// widens it, so the result is what the same field gets over plain cbor: a
// date into a time.Time is its midnight UTC, and a bigdecimal that fits a
// decimal into a float64 is parsed as a decimal is. A regex, which plain
// cbor cannot carry, goes into a string kind as its pattern.
func (d *decoder) decodeCBOR2KindTag(tagNum uint64, start int, v reflect.Value) error {
	d.pos = start
	if err := d.skipCBORItem(); err != nil {
		return err
	}
	raw := d.data[start:d.pos]
	// A bigdecimal or a regex is text: hold it to the limit other text has.
	if uint64(len(raw)) > MaxCBORStringLength+maxKindHeadLength {
		return fmt.Errorf("CBOR tag %d (%s) of %d bytes exceeds maximum allowed (%d)",
			tagNum, cbor2TagName[tagNum], len(raw), MaxCBORStringLength)
	}
	switch {
	case v.Kind() == reflect.Interface:
		return setKindValue(v, tagNum, raw)
	case tagNum == models.TagRegex:
		return setRegexPattern(v, raw)
	}
	return d.decodeWidened(tagNum, raw, v)
}

// setKindValue decodes raw into its models type and sets the interface v to it.
func setKindValue(v reflect.Value, tagNum uint64, raw []byte) error {
	value, err := kindValue(tagNum, raw)
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(value)
	if !rv.Type().AssignableTo(v.Type()) {
		return fmt.Errorf("cannot decode %s (CBOR tag %d) into Go value of type %v", cbor2TagName[tagNum], tagNum, v.Type())
	}
	v.Set(rv)
	return nil
}

// kindValue decodes raw, the item of tag tagNum, into its models type.
func kindValue(tagNum uint64, raw []byte) (any, error) {
	switch tagNum {
	case models.TagPlainDate:
		return unmarshalKind[models.PlainDate](raw)
	case models.TagPlainTime:
		return unmarshalKind[models.PlainTime](raw)
	case models.TagTimestamp:
		return unmarshalKind[models.Timestamp](raw)
	case models.TagInterval:
		return unmarshalKind[models.Interval](raw)
	case models.TagBigDecimal:
		return unmarshalKind[models.BigDecimal](raw)
	default:
		return unmarshalKind[models.Regex](raw)
	}
}

// unmarshalKind decodes raw into a new T through *T's UnmarshalCBOR.
func unmarshalKind[T any, P interface {
	*T
	Unmarshaler
}](raw []byte) (any, error) {
	var v T
	if err := P(&v).UnmarshalCBOR(raw); err != nil {
		return nil, err
	}
	return v, nil
}

// setRegexPattern decodes a regex into v, which must be a string kind, as its
// pattern.
func setRegexPattern(v reflect.Value, raw []byte) error {
	if v.Kind() != reflect.String {
		return fmt.Errorf("cannot decode regex (CBOR tag %d) into Go value of type %v", models.TagRegex, v.Type())
	}
	var r models.Regex
	if err := r.UnmarshalCBOR(raw); err != nil {
		return err
	}
	v.SetString(r.Pattern)
	return nil
}

// decodeWidened decodes raw into v as plain cbor carries it: widened by
// cborwire, and read by a plain cbor decoder.
func (d *decoder) decodeWidened(tagNum uint64, raw []byte, v reflect.Value) error {
	widened, err := cborwire.Normalize(raw, cborwire.Plain)
	if err == nil {
		plain := &decoder{
			data:           widened,
			defaultMapType: d.defaultMapType,
			fieldResolver:  d.fieldResolver,
			cborVersion:    cborV1,
		}
		err = plain.decodeValue(v)
	}
	if err != nil {
		return fmt.Errorf("cannot decode %s (CBOR tag %d) into Go value of type %v: %w", cbor2TagName[tagNum], tagNum, v.Type(), err)
	}
	return nil
}

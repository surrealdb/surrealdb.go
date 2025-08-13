package surrealcbor

import (
	"fmt"
	"math"
	"reflect"
)

// decodeNegInt decodes a CBOR negative integer (Major Type 1) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.4
func (d *decoder) decodeNegInt(v reflect.Value) error {
	val, err := d.readUint()
	if err != nil {
		return err
	}

	negVal := -1 - int64(val) // #nosec G115 - CBOR spec defines this conversion

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Check for overflow based on the target type
		switch v.Kind() {
		case reflect.Int8:
			if negVal < math.MinInt8 || negVal > math.MaxInt8 {
				return fmt.Errorf("value %d overflows int8", negVal)
			}
		case reflect.Int16:
			if negVal < math.MinInt16 || negVal > math.MaxInt16 {
				return fmt.Errorf("value %d overflows int16", negVal)
			}
		case reflect.Int32:
			if negVal < math.MinInt32 || negVal > math.MaxInt32 {
				return fmt.Errorf("value %d overflows int32", negVal)
			}
		}
		v.SetInt(negVal)
	case reflect.Interface:
		v.Set(reflect.ValueOf(negVal))
	default:
		return fmt.Errorf("cannot decode negative int into %v", v.Type())
	}
	return nil
}

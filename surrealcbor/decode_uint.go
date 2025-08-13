package surrealcbor

import (
	"fmt"
	"math"
	"reflect"
)

// decodeUint decodes a CBOR unsigned integer (Major Type 0) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.2
func (d *decoder) decodeUint(v reflect.Value) error {
	val, err := d.readUint()
	if err != nil {
		return err
	}

	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val > math.MaxInt64 {
			return fmt.Errorf("uint %d overflows int", val)
		}
		v.SetInt(int64(val))
	case reflect.Interface:
		v.Set(reflect.ValueOf(val))
	default:
		return fmt.Errorf("cannot decode uint into %v", v.Type())
	}
	return nil
}

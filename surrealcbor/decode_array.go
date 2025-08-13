package surrealcbor

import (
	"fmt"
	"io"
	"reflect"
)

// MaxCBORArrayLength is used to enforce a maximum
// allowed array length when decoding CBOR arrays
//
// This originates from the following CodeQL finding:
//
//	Slice memory allocation with excessive size value
var MaxCBORArrayLength uint64 = 1000000

// decodeArray decodes a CBOR array (Major Type 4) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.10
func (d *decoder) decodeArray(v reflect.Value) error {
	// Check for indefinite length
	if d.pos >= len(d.data) {
		return io.EOF
	}
	if d.data[d.pos]&0x1f == 31 {
		d.pos++ // Skip the indefinite length marker
		return d.decodeIndefiniteArray(v)
	}

	var length int
	err := d.readLength(&length)
	if err != nil {
		return err
	}

	switch v.Kind() {
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), length, length))
		for i := 0; i < length; i++ {
			if err := d.decodeValue(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Array:
		if v.Len() < length {
			return fmt.Errorf("array too small")
		}
		for i := 0; i < length; i++ {
			if err := d.decodeValue(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Interface:
		arr := make([]any, length)
		for i := 0; i < length; i++ {
			var elem any
			if err := d.decodeValue(reflect.ValueOf(&elem).Elem()); err != nil {
				return err
			}
			arr[i] = elem
		}
		v.Set(reflect.ValueOf(arr))
	default:
		return fmt.Errorf("cannot decode array into %v", v.Type())
	}
	return nil
}

func (d *decoder) readLength(dst *int) error {
	length, err := d.readUint()
	if err != nil {
		return err
	}
	if length > MaxCBORArrayLength {
		return fmt.Errorf("CBOR array length %d exceeds maximum allowed (%d)", length, MaxCBORArrayLength)
	}
	if length > uint64(int(^uint(0)>>1)) { // check for int overflow
		return fmt.Errorf("CBOR array length %d overflows int", length)
	}

	*dst = int(length)
	return nil
}

func (d *decoder) decodeIndefiniteArray(v reflect.Value) error {
	var elements []reflect.Value

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Decode the next element
		elem := reflect.New(v.Type().Elem()).Elem()
		if err := d.decodeValue(elem); err != nil {
			return err
		}
		elements = append(elements, elem)
	}

	// Create the slice with the decoded elements
	slice := reflect.MakeSlice(v.Type(), len(elements), len(elements))
	for i, elem := range elements {
		slice.Index(i).Set(elem)
	}
	v.Set(slice)
	return nil
}

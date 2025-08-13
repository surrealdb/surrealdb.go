package surrealcbor

import (
	"fmt"
	"io"
	"reflect"
)

// decodeBytes decodes a CBOR byte string (Major Type 2) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.6
func (d *decoder) decodeBytes(v reflect.Value) error {
	// Check for indefinite length
	if d.pos >= len(d.data) {
		return io.EOF
	}
	if d.data[d.pos]&0x1f == 31 {
		d.pos++ // Skip the indefinite length marker
		return d.decodeIndefiniteBytes(v)
	}

	length, err := d.readUint()
	if err != nil {
		return err
	}

	remaining := len(d.data) - d.pos
	if remaining < 0 {
		return fmt.Errorf("not enough data to decode bytes, expected %d bytes, got %d", length, remaining)
	}

	if length > uint64(remaining) {
		return io.ErrUnexpectedEOF
	}

	byteLen := int(length) // #nosec G115 - length checked above
	bsData := d.data[d.pos : d.pos+byteLen]
	d.pos += byteLen

	switch v.Kind() {
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			v.SetBytes(bsData)
		} else {
			return fmt.Errorf("cannot decode bytes into %v", v.Type())
		}
	case reflect.Interface:
		v.Set(reflect.ValueOf(bsData))
	default:
		return fmt.Errorf("cannot decode bytes into %v", v.Type())
	}
	return nil
}

func (d *decoder) decodeIndefiniteBytes(v reflect.Value) error {
	var chunks [][]byte

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Each chunk must be a definite-length byte string
		if d.data[d.pos]>>5 != 2 {
			return fmt.Errorf("indefinite byte string chunk must be a byte string")
		}

		var chunk []byte
		if err := d.decodeValue(reflect.ValueOf(&chunk).Elem()); err != nil {
			return err
		}
		chunks = append(chunks, chunk)
	}

	// Concatenate all chunks
	totalLen := 0
	for _, chunk := range chunks {
		totalLen += len(chunk)
	}
	result := make([]byte, 0, totalLen)
	for _, chunk := range chunks {
		result = append(result, chunk...)
	}

	switch v.Kind() {
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			v.SetBytes(result)
		} else {
			return fmt.Errorf("cannot decode bytes into %v", v.Type())
		}
	case reflect.Interface:
		v.Set(reflect.ValueOf(result))
	default:
		return fmt.Errorf("cannot decode bytes into %v", v.Type())
	}
	return nil
}

package surrealcbor

import (
	"fmt"
	"io"
	"reflect"
)

// decodeString decodes a CBOR text string (Major Type 3) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.8
func (d *decoder) decodeString(v reflect.Value) error {
	// Check for indefinite length
	if d.pos >= len(d.data) {
		return io.EOF
	}
	if d.data[d.pos]&0x1f == 31 {
		d.pos++ // Skip the indefinite length marker
		return d.decodeIndefiniteString(v)
	}

	length, err := d.readUint()
	if err != nil {
		return err
	}

	remaining := len(d.data) - d.pos
	if remaining < 0 {
		return fmt.Errorf("not enough data to decode string, expected %d bytes, got %d", length, remaining)
	}

	if length > uint64(remaining) {
		return io.ErrUnexpectedEOF
	}

	strLen := int(length) // #nosec G115 - length checked above
	str := string(d.data[d.pos : d.pos+strLen])
	d.pos += strLen

	switch v.Kind() {
	case reflect.String:
		v.SetString(str)
	case reflect.Interface:
		v.Set(reflect.ValueOf(str))
	default:
		return fmt.Errorf("cannot decode string into %v", v.Type())
	}
	return nil
}

func (d *decoder) decodeIndefiniteString(v reflect.Value) error {
	var chunks []string

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Each chunk must be a definite-length string
		if d.data[d.pos]>>5 != 3 {
			return fmt.Errorf("indefinite string chunk must be a text string")
		}

		var chunk string
		if err := d.decodeValue(reflect.ValueOf(&chunk).Elem()); err != nil {
			return err
		}
		chunks = append(chunks, chunk)
	}

	// Concatenate all chunks
	result := ""
	for _, chunk := range chunks {
		result += chunk
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(result)
	case reflect.Interface:
		v.Set(reflect.ValueOf(result))
	default:
		return fmt.Errorf("cannot decode string into %v", v.Type())
	}
	return nil
}

package surrealcbor

import (
	"fmt"
	"io"
	"reflect"
)

// Maximum allowed string length when decoding CBOR strings
// This prevents excessive memory allocation
var MaxCBORStringLength uint64 = 10000000 // 10MB

// readStringLength reads the length of a string and returns it as an int
// with appropriate bounds checking
func (d *decoder) readStringLength(dst *int) error {
	length, err := d.readUint()
	if err != nil {
		return err
	}
	if length > MaxCBORStringLength {
		return fmt.Errorf("CBOR string length %d exceeds maximum allowed (%d)", length, MaxCBORStringLength)
	}
	if length > uint64(int(^uint(0)>>1)) { // check for int overflow
		return fmt.Errorf("CBOR string length %d overflows int", length)
	}

	*dst = int(length) // #nosec G115 - length checked above
	return nil
}

// decodeStringDirect decodes a CBOR text string directly without using reflect.Value
// This avoids allocations when we just need the string value itself.
//
// That said, this is a better alternative to:
//
//	decodeValue(reflect.ValueOf(&key).Elem())
func (d *decoder) decodeStringDirect() (string, error) {
	// Check major type
	if d.pos >= len(d.data) {
		return "", io.EOF
	}

	majorType := d.data[d.pos] >> 5
	if majorType != 3 { // Text string
		return "", fmt.Errorf("expected text string (major type 3), got major type %d", majorType)
	}

	// Check for indefinite length
	if d.data[d.pos]&0x1f == 31 {
		d.pos++ // Skip the indefinite length marker
		return d.decodeIndefiniteStringDirect()
	}

	var strLen int
	err := d.readStringLength(&strLen)
	if err != nil {
		return "", err
	}

	remaining := len(d.data) - d.pos
	if remaining < strLen {
		return "", io.ErrUnexpectedEOF
	}
	str := string(d.data[d.pos : d.pos+strLen])
	d.pos += strLen

	return str, nil
}

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

	var strLen int
	err := d.readStringLength(&strLen)
	if err != nil {
		return err
	}

	remaining := len(d.data) - d.pos
	if remaining < strLen {
		return io.ErrUnexpectedEOF
	}
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

// decodeIndefiniteStringDirect decodes an indefinite-length string directly
func (d *decoder) decodeIndefiniteStringDirect() (string, error) {
	var chunks []string

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return "", io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Each chunk must be a definite-length string
		if d.data[d.pos]>>5 != 3 {
			return "", fmt.Errorf("indefinite string chunk must be a text string")
		}

		// Check that chunk is not indefinite (avoid recursion)
		if d.data[d.pos]&0x1f == 31 {
			return "", fmt.Errorf("indefinite string chunks cannot be indefinite")
		}

		// Decode definite-length chunk
		var strLen int
		err := d.readStringLength(&strLen)
		if err != nil {
			return "", err
		}

		remaining := len(d.data) - d.pos
		if remaining < strLen {
			return "", io.ErrUnexpectedEOF
		}

		chunk := string(d.data[d.pos : d.pos+strLen])
		d.pos += strLen
		chunks = append(chunks, chunk)
	}

	// Concatenate all chunks
	result := ""
	for _, chunk := range chunks {
		result += chunk
	}

	return result, nil
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

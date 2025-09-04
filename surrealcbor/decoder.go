package surrealcbor

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// errNoUnmarshaler is a sentinel error indicating that no custom unmarshaler was found
// This is not an actual error condition, just a signal to continue with standard decoding
var errNoUnmarshaler = fmt.Errorf("no custom unmarshaler found")

// decoder is our custom CBOR decoder
type decoder struct {
	data           []byte
	pos            int
	defaultMapType reflect.Type // Type of map to create when decoding to interface{}
	fieldResolver  FieldResolver
}

func (d *decoder) decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("unmarshal requires non-nil pointer")
	}

	return d.decodeValue(rv.Elem())
}

func (d *decoder) decodeValue(v reflect.Value) error {
	if d.pos >= len(d.data) {
		return io.EOF
	}

	// Check if target type is cbor.RawMessage first (needs special raw bytes handling)
	// Note: cbor.RawMessage is already a reference type ([]byte), so *cbor.RawMessage
	// is unnecessary and not supported. Users should use cbor.RawMessage directly.
	if v.Type() == reflect.TypeOf(cbor.RawMessage{}) {
		return d.decodeRawMessage(v)
	}

	// Check for special values (null/undefined/None) BEFORE trying UnmarshalCBOR
	// This is important for pointer types - if the value is nil/null, we shouldn't
	// try to unmarshal it
	if isNilValue, err := d.checkAndDecodeNilValue(v); isNilValue {
		return err
	}

	// Try to use UnmarshalCBOR if the type implements it
	if err := d.tryUnmarshaler(v); err != errNoUnmarshaler {
		// errNoUnmarshaler means type doesn't implement Unmarshaler, continue with standard decoding
		// Any other error (including nil) should be returned
		return err
	}

	// Handle pointer types after checking for None/null
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return d.decodeValue(v.Elem())
	}

	// Decode based on major type
	return d.decodeByMajorType(v)
}

// tryUnmarshaler tries to use custom Unmarshaler if the type implements it
// This is only called after canImplementUnmarshaler returns true,
// so we already know it's not a primitive or models package type
func (d *decoder) tryUnmarshaler(v reflect.Value) error {
	// Handle pointer types specially
	if v.Kind() == reflect.Pointer {
		return d.tryUnmarshalerPointer(v)
	}

	// For non-pointer types, check if address implements Unmarshaler
	if v.CanAddr() {
		if unmarshaler, ok := v.Addr().Interface().(Unmarshaler); ok {
			return d.callUnmarshaler(unmarshaler)
		}
	}

	// Check if the value itself implements Unmarshaler (for value receivers)
	if v.CanInterface() {
		// Skip nil pointers as they can't unmarshal themselves
		if v.Kind() != reflect.Pointer || !v.IsNil() {
			if unmarshaler, ok := v.Interface().(Unmarshaler); ok {
				return d.callUnmarshaler(unmarshaler)
			}
		}
	}

	return errNoUnmarshaler // Signal that no unmarshaler was found
}

// tryUnmarshalerPointer handles Unmarshaler for pointer types
// We already filtered out models package types in canImplementUnmarshaler
func (d *decoder) tryUnmarshalerPointer(v reflect.Value) error {
	if v.IsNil() {
		// Check if the pointed-to type implements Unmarshaler
		elemType := v.Type().Elem()
		ptrToElem := reflect.New(elemType)
		if unmarshaler, ok := ptrToElem.Interface().(Unmarshaler); ok {
			// Allocate and decode
			v.Set(ptrToElem)
			return d.callUnmarshaler(unmarshaler)
		}
	} else {
		// Non-nil pointer, check if it implements Unmarshaler
		if unmarshaler, ok := v.Interface().(Unmarshaler); ok {
			return d.callUnmarshaler(unmarshaler)
		}
	}
	return errNoUnmarshaler // Signal that no unmarshaler was found
}

// callUnmarshaler calls the UnmarshalCBOR method with the raw CBOR bytes
func (d *decoder) callUnmarshaler(unmarshaler Unmarshaler) error {
	startPos := d.pos
	if err := d.skipCBORItem(); err != nil {
		return err
	}
	rawBytes := d.data[startPos:d.pos]
	return unmarshaler.UnmarshalCBOR(rawBytes)
}

// checkAndDecodeNilValue checks if the current value is nil/null/undefined/None
// Returns (true, nil) if it was a nil value and was handled
// Returns (false, nil) if it's not a nil value
func (d *decoder) checkAndDecodeNilValue(v reflect.Value) (bool, error) {
	majorType := d.data[d.pos] >> 5
	additionalInfo := d.data[d.pos] & 0x1f

	// Check for null/undefined
	if majorType == 7 && (additionalInfo == 22 || additionalInfo == 23) {
		return true, d.decodeSimple(v, additionalInfo)
	}

	// Check for Tag 6 (None)
	if majorType == 6 {
		tagNum, err := d.peekTag()
		if err == nil && tagNum == models.TagNone {
			return true, d.decodeTag(v)
		}
	}

	return false, nil
}

// decodeByMajorType decodes based on CBOR major type
func (d *decoder) decodeByMajorType(v reflect.Value) error {
	majorType := d.data[d.pos] >> 5
	additionalInfo := d.data[d.pos] & 0x1f

	switch majorType {
	case 0: // Unsigned integer
		return d.decodeUint(v)
	case 1: // Negative integer
		return d.decodeNegInt(v)
	case 2: // Byte string
		return d.decodeBytes(v)
	case 3: // Text string
		return d.decodeString(v)
	case 4: // Array
		return d.decodeArray(v)
	case 5: // Map
		return d.decodeMap(v)
	case 6: // Tag
		return d.decodeTag(v)
	case 7: // Simple/Float
		return d.decodeSimple(v, additionalInfo)
	default:
		return fmt.Errorf("unknown major type %d", majorType)
	}
}

// decodeRawMessage decodes CBOR data into cbor.RawMessage by finding the complete CBOR item
// and copying the raw bytes
func (d *decoder) decodeRawMessage(v reflect.Value) error {
	// Save the starting position
	startPos := d.pos

	// Skip the complete CBOR item to find its end
	if err := d.skipCBORItem(); err != nil {
		return err
	}

	// Now d.pos is at the end of the item
	// Copy the raw bytes from startPos to d.pos
	rawBytes := d.data[startPos:d.pos]

	// cbor.RawMessage is []byte, so we set it directly
	v.SetBytes(append([]byte(nil), rawBytes...))

	return nil
}

// Decoder reads and decodes CBOR values from an input stream
// DefaultReadBufferSize is the default size for reading from the underlying reader
const DefaultReadBufferSize = 4096

type Decoder struct {
	r              io.Reader
	buf            *bytes.Buffer
	readBufSize    int
	defaultMapType reflect.Type // Type of map to create when decoding to interface{}
	fieldResolver  FieldResolver
}

// readMore reads more data from the underlying reader into the buffer
func (dec *Decoder) readMore() (int, error) {
	bufSize := dec.readBufSize
	if bufSize <= 0 {
		bufSize = DefaultReadBufferSize
	}

	temp := make([]byte, bufSize)
	n, err := dec.r.Read(temp)
	if n > 0 {
		dec.buf.Write(temp[:n])
	}
	return n, err
}

// SetDefaultMapType sets the map type to use when decoding to interface{} values.
// The mapSample should be an instance of the desired map type, e.g., map[string]any{}.
func (dec *Decoder) SetDefaultMapType(mapSample any) error {
	if mapSample == nil {
		dec.defaultMapType = reflect.TypeOf(map[string]any{})
		return nil
	}

	mapType := reflect.TypeOf(mapSample)
	if mapType.Kind() != reflect.Map {
		return fmt.Errorf("SetDefaultMapType requires a map type, got %v", mapType)
	}
	dec.defaultMapType = mapType
	return nil
}

// Decode reads CBOR data from the reader and decodes into v
func (dec *Decoder) Decode(v any) error {
	// Initialize field resolver if not set
	if dec.fieldResolver == nil {
		dec.fieldResolver = NewCachedFieldResolver()
	}

	var lastDecodeErr error

	for {
		// If buffer is empty or we had an EOF error, try to read more data
		if dec.buf.Len() == 0 || (lastDecodeErr == io.EOF || lastDecodeErr == io.ErrUnexpectedEOF) {
			n, readErr := dec.readMore()
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			// If we couldn't read more data and buffer is still empty or we had a decode error
			if n == 0 && readErr == io.EOF {
				if lastDecodeErr != nil {
					return lastDecodeErr // Return the decode error that caused us to need more data
				}
				if dec.buf.Len() == 0 {
					return io.EOF // No data at all
				}
			}
		}

		// Try decoding with current buffer
		data := dec.buf.Bytes()
		d := &decoder{
			data:           data,
			pos:            0,
			defaultMapType: dec.defaultMapType,
			fieldResolver:  dec.fieldResolver,
		}
		err := d.decode(v)
		if err == nil {
			// Success! Remove consumed bytes from buffer
			dec.buf.Next(d.pos)
			return nil
		}

		// If we hit EOF during decoding, save error and loop to try reading more
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			lastDecodeErr = err
			continue
		}

		// For other errors, return immediately
		return err
	}
}

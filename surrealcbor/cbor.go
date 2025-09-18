// Package surrealcbor provides CBOR (Concise Binary Object Representation) encoding and decoding for SurrealDB.
//
// # SurrealDB CBOR Protocol
//
// The SDK uses this package for handling the SurrealDB CBOR Protocol.
//
// This implementation is optimized for SurrealDB's custom CBOR tags and provides proper handling of SurrealDB-specific
// data types like NONE values, UUIDs, Record IDs, and Geometry types.
//
// For advanced CBOR handling and custom marshaling, refer to [Marshaler] and [Unmarshaler], and the `CustomUnmarshalCBOR` example.
//
// The SurrealDB CBOR protocol is documented at https://surrealdb.com/docs/surrealdb/integration/cbor.
//
// # Custom CBOR Tags
//
// The protocol uses the following CBOR tags for custom data types:
//
//	| Tag | Value               | Description                          |
//	|-----|---------------------|--------------------------------------|
//	| 0   | Datetime (ISO 8601) | ISO 8601 formatted datetime string   |
//	| 6   | NONE                | Represents absence of value          |
//	| 7   | Table name          | Database table identifier            |
//	| 8   | Record ID           | Unique record identifier             |
//	| 9   | UUID (string)       | UUID in string representation        |
//	| 10  | Decimal (string)    | High-precision decimal as string     |
//	| 12  | Datetime (compact)  | Compact binary datetime format       |
//	| 13  | Duration (string)   | Duration in string format            |
//	| 14  | Duration (compact)  | Compact binary duration format       |
//	| 15  | Future (compact)    | Compact future value representation  |
//	| 37  | UUID (binary)       | UUID in binary representation        |
//	| 49  | Range               | Range type for intervals             |
//	| 50  | Included Bound      | Inclusive range boundary             |
//	| 51  | Excluded Bound      | Exclusive range boundary             |
//	| 88  | Geometry Point      | Geographic point coordinates         |
//	| 89  | Geometry Line       | Geographic line string               |
//	| 90  | Geometry Polygon    | Geographic polygon shape             |
//	| 91  | Geometry MultiPoint | Collection of geographic points      |
//	| 92  | Geometry MultiLine  | Collection of geographic lines       |
//	| 93  | Geometry MultiPolygon | Collection of geographic polygons  |
//	| 94  | Geometry Collection | Mixed collection of geometries       |
//
// # Why our own CBOR implementation?
//
// We wanted to unmarshal SurrealDB `NONE` into Go `nil`.
//
// We had been a happy user of https://github.com/fxamacker/cbor for a long time.
// However, the fact that SurrealDB uses a custom CBOR tag for `NONE` value representation
// forced us to implement our own CBOR encoder/decoder.
//
// With fxamacker/cbor, SurrealDB `None` couldn't be unmarshaled into Go `nil`, because to be unmarshaled into `nil`,
// the CBOR value must be a simple `null` or `undefined` value.
//
// Relevant issue on the SurrealDB Go SDK:
// https://github.com/surrealdb/surrealdb.go/issues/291
//
// Relevant code lines in fxamacker/cbor:
// https://github.com/fxamacker/cbor/blob/ead1106a21cc5955543938c5fd3a23e544263f4a/decode.go#L1406-L1411
//
// Do note that we rely on fxamacker/cbor where possible for the actual encoding/decoding logic.
// More concretely, we use fxamacker/cbor for marshaling anything.
package surrealcbor

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
)

// globalFieldResolver is a shared field resolver used by Unmarshal functions
var globalFieldResolver = NewCachedFieldResolver()

func New() *Codec {
	return &Codec{
		encMode: getEncMode(),
	}
}

// UnmarshalOptions contains options for unmarshaling CBOR data
type UnmarshalOptions struct {
	// DefaultMapType specifies a sample map to use as template when decoding CBOR maps to interface{} values.
	// For example, pass map[string]any{} to decode maps with string keys,
	// or map[any]any{} to decode maps with any key type.
	// If nil, defaults to map[string]any{} for backward compatibility.
	DefaultMapType any
}

// Unmarshal is our custom implementation that handles Tag 6 as nil
func Unmarshal(data []byte, v any) error {
	d := &decoder{
		data:           data,
		pos:            0,
		defaultMapType: reflect.TypeOf(map[string]any{}), // Default to string keys for backward compatibility
		fieldResolver:  globalFieldResolver,
	}
	return d.decode(v)
}

// UnmarshalWithOptions unmarshals CBOR data with custom options
func UnmarshalWithOptions(data []byte, v any, opts UnmarshalOptions) error {
	var mapType reflect.Type
	if opts.DefaultMapType != nil {
		mapType = reflect.TypeOf(opts.DefaultMapType)
		if mapType.Kind() != reflect.Map {
			return fmt.Errorf("DefaultMapType must be a map type, got %v", mapType)
		}
	} else {
		mapType = reflect.TypeOf(map[string]any{})
	}

	d := &decoder{
		data:           data,
		pos:            0,
		defaultMapType: mapType,
		fieldResolver:  globalFieldResolver,
	}
	return d.decode(v)
}

// NewDecoder creates a decoder that reads from r with default buffer size
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:              r,
		buf:            &bytes.Buffer{},
		readBufSize:    0,                                // Will use DefaultReadBufferSize
		defaultMapType: reflect.TypeOf(map[string]any{}), // Default to string keys for backward compatibility
	}
}

// NewDecoderWithBufferSize creates a decoder with a custom read buffer size
func NewDecoderWithBufferSize(r io.Reader, bufSize int) *Decoder {
	return &Decoder{
		r:              r,
		buf:            &bytes.Buffer{},
		readBufSize:    bufSize,
		defaultMapType: reflect.TypeOf(map[string]any{}), // Default to string keys for backward compatibility
	}
}

// Marshal uses fxamacker/cbor for marshaling with proper tag registration
func Marshal(v any) ([]byte, error) {
	em := getEncMode()
	return em.Marshal(v)
}

// NewEncoder creates an encoder that writes to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:  w,
		em: getEncMode(),
	}
}

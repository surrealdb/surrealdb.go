// Package surrealcbor provides CBOR encoding and decoding with special handling
// for SurrealDB-specific types and custom marshaling/unmarshaling support.
package surrealcbor

// Unmarshaler is the interface implemented by types that can unmarshal a CBOR
// representation of themselves. UnmarshalCBOR must copy the CBOR data if it
// needs to use it after returning.
//
// This interface is intentionally identical to github.com/fxamacker/cbor/v2.Unmarshaler
// to ensure compatibility. Types implementing this interface will work with both
// surrealcbor and fxamacker/cbor packages.
//
// # Why we define this interface locally
//
//   - Provides abstraction: SDK consumers can implement custom unmarshaling without
//     directly importing fxamacker/cbor in simple cases
//   - Ensures compatibility: Types implementing this interface automatically work with
//     fxamacker/cbor since the signatures are identical
//   - Clarifies intent: Makes it explicit that custom CBOR unmarshaling is supported
//
// # Package independence limitations
//
// In practice, most implementations WILL need to import fxamacker/cbor because
// manually constructing/parsing CBOR bytes is complex and error-prone.
// You'll need fxamacker/cbor if your implementation needs to:
//   - Unmarshal the raw CBOR bytes into intermediate types (using cbor.Unmarshal)
//   - Handle complex CBOR structures (maps, arrays, tags)
//   - Use CBOR-specific decoding options or modes
//   - Handle variable-length integers, floats, or strings > 23 bytes
//
// See simple_example_test.go for a rare case where importing cbor is not needed.
//
// # When to implement this interface
//
//   - When you need custom CBOR decoding logic for your types
//   - When the default struct tag-based decoding is insufficient
//   - When you need to handle special CBOR constructs or perform validation during unmarshaling
//
// # Example usage
//
//	type Temperature struct {
//	    Value float64
//	    Unit  string // "C", "F", or "K"
//	}
//
//	func (t *Temperature) UnmarshalCBOR(data []byte) error {
//	    // Decode from a compact format: [value, unit_code]
//	    // where unit_code is: 0=Celsius, 1=Fahrenheit, 2=Kelvin
//	    var compact []interface{}
//	    // Note: This requires importing fxamacker/cbor for cbor.Unmarshal
//	    if err := cbor.Unmarshal(data, &compact); err != nil {
//	        return err
//	    }
//	    if len(compact) != 2 {
//	        return fmt.Errorf("invalid temperature format")
//	    }
//
//	    t.Value = compact[0].(float64)
//	    switch compact[1].(uint64) {
//	    case 0:
//	        t.Unit = "C"
//	    case 1:
//	        t.Unit = "F"
//	    case 2:
//	        t.Unit = "K"
//	    default:
//	        return fmt.Errorf("unknown unit code")
//	    }
//	    return nil
//	}
//
// Note: Types from the github.com/surrealdb/surrealdb.go/pkg/models package
// (RecordID, CustomDateTime, etc.) do NOT use this interface. They are handled
// via CBOR tags for proper SurrealDB protocol compatibility.
type Unmarshaler interface {
	UnmarshalCBOR(data []byte) error
}

// Marshaler is the interface implemented by types that can marshal themselves
// to a valid CBOR representation.
//
// This interface is intentionally identical to github.com/fxamacker/cbor/v2.Marshaler
// to ensure compatibility. Types implementing this interface will work with both
// surrealcbor and fxamacker/cbor packages.
//
// # Why we define this interface locally
//
//   - Provides abstraction: SDK consumers can implement custom marshaling without
//     directly importing fxamacker/cbor in simple cases
//   - Ensures compatibility: Types implementing this interface automatically work with
//     fxamacker/cbor since the signatures are identical
//   - Clarifies intent: Makes it explicit that custom CBOR marshaling is supported
//
// # Package independence limitations
//
// In practice, most implementations WILL need to import fxamacker/cbor because
// manually constructing CBOR bytes is complex and error-prone.
// You'll need fxamacker/cbor if your implementation needs to:
//   - Marshal intermediate values to CBOR (using cbor.Marshal)
//   - Create complex CBOR structures (maps, arrays, tags)
//   - Use CBOR-specific encoding options or modes
//   - Handle variable-length integers, floats, or strings > 23 bytes
//
// See simple_example_test.go for a rare case where importing cbor is not needed.
//
// # When to implement this interface
//
//   - When you need custom CBOR encoding logic for your types
//   - When the default struct tag-based encoding is insufficient
//   - When you need to produce specific CBOR constructs or optimize encoding
//
// # Example usage
//
//	type Temperature struct {
//	    Value float64
//	    Unit  string // "C", "F", or "K"
//	}
//
//	func (t Temperature) MarshalCBOR() ([]byte, error) {
//	    // Encode to a compact format: [value, unit_code]
//	    // where unit_code is: 0=Celsius, 1=Fahrenheit, 2=Kelvin
//	    var unitCode uint64
//	    switch t.Unit {
//	    case "C":
//	        unitCode = 0
//	    case "F":
//	        unitCode = 1
//	    case "K":
//	        unitCode = 2
//	    default:
//	        return nil, fmt.Errorf("unknown unit: %s", t.Unit)
//	    }
//
//	    compact := []interface{}{t.Value, unitCode}
//	    // Note: This requires importing fxamacker/cbor for cbor.Marshal
//	    return cbor.Marshal(compact)
//	}
//
// Note: Types from the github.com/surrealdb/surrealdb.go/pkg/models package
// (RecordID, CustomDateTime, etc.) do NOT use this interface. They are handled
// via CBOR tags for proper SurrealDB protocol compatibility.
type Marshaler interface {
	MarshalCBOR() ([]byte, error)
}


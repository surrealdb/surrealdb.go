package surrealcbor

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_invalidTarget tests unmarshaling to invalid targets
func TestDecode_invalidTarget(t *testing.T) {
	t.Run("unmarshal to non-pointer", func(t *testing.T) {
		data := []byte{0x01}
		var v int
		err := Unmarshal(data, v) // Not a pointer
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-nil pointer")
	})

	t.Run("unmarshal to nil pointer", func(t *testing.T) {
		data := []byte{0x01}
		err := Unmarshal(data, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})
}

// TestDecode_invalidSurrealCBOR tests various invalid SurrealCBOR decoding scenarios
func TestDecode_invalidSurrealCBOR(t *testing.T) {
	t.Run("decode with invalid major type", func(t *testing.T) {
		// Invalid CBOR data
		data := []byte{0xFF, 0xFF} // Break in unexpected position

		var v int
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})
}

// TestDecode_incompatibleType tests various incompatible type decoding scenarios
func TestDecode_incompatibleType(t *testing.T) {
	t.Run("decode float to non-float type", func(t *testing.T) {
		// Float32 data
		data := []byte{0xFA, 0x40, 0x48, 0xF5, 0xC3} // float32(3.14)

		var s string
		err := Unmarshal(data, &s)
		// Our decoder returns an error for type mismatch
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float32 into Go value of type string")
	})

	t.Run("decode array to non-array type", func(t *testing.T) {
		// Array data
		data := []byte{0x83, 0x01, 0x02, 0x03} // [1, 2, 3]

		var i int
		err := Unmarshal(data, &i)
		assert.Error(t, err)
	})

	t.Run("decode map to non-map type", func(t *testing.T) {
		// Map data
		data := []byte{0xA1, 0x61, 0x61, 0x01} // {"a": 1}

		var i int
		err := Unmarshal(data, &i)
		assert.Error(t, err)
	})

	t.Run("decode array to incompatible slice type", func(t *testing.T) {
		data := []byte{0x82, 0x61, 0x61, 0x61, 0x62} // ["a", "b"]
		var v []int                                  // Can't decode strings to ints
		err := Unmarshal(data, &v)
		// Type mismatch error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot decode string into int")
	})

	t.Run("decode uint to incompatible type", func(t *testing.T) {
		data := []byte{0x01} // uint(1)
		var ch chan int
		err := Unmarshal(data, &ch)
		assert.Error(t, err)
	})

	t.Run("decode negative int to incompatible type", func(t *testing.T) {
		data := []byte{0x20} // negative int(-1)
		var ch chan int
		err := Unmarshal(data, &ch)
		assert.Error(t, err)
	})

	t.Run("decode bytes to incompatible type", func(t *testing.T) {
		data := []byte{0x42, 0x01, 0x02} // bytes([1, 2])
		var ch chan int
		err := Unmarshal(data, &ch)
		assert.Error(t, err)
	})

	t.Run("decode string to incompatible type", func(t *testing.T) {
		data := []byte{0x61, 0x61} // string("a")
		var ch chan int
		err := Unmarshal(data, &ch)
		assert.Error(t, err)
	})
}

// TestDecode_invalidCBOR tests various invalid CBOR data scenarios
func TestDecode_invalidCBOR(t *testing.T) {
	t.Run("decode with EOF at start", func(t *testing.T) {
		data := []byte{}
		var v int
		err := Unmarshal(data, &v)
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("decode array with error in element", func(t *testing.T) {
		// Array with truncated element
		data := []byte{0x82, 0x01, 0x1A} // [1, <incomplete uint32>]
		var v []int
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode map with error in value", func(t *testing.T) {
		// Map with truncated value
		data := []byte{0xA1, 0x61, 0x61, 0x1A} // {"a": <incomplete uint32>}
		var v map[string]int
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode with insufficient bytes data", func(t *testing.T) {
		data := []byte{0x42, 0x01} // Byte string claiming 2 bytes but only has 1
		var v []byte
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode with insufficient string data", func(t *testing.T) {
		data := []byte{0x62, 0x61} // String claiming 2 bytes but only has 1
		var v string
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode simple value 24 with no data", func(t *testing.T) {
		data := []byte{0xF8} // Simple value 24 needs next byte
		var v any
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode float16 with insufficient data", func(t *testing.T) {
		data := []byte{0xF9, 0x00} // Float16 needs 2 bytes
		var v float32
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode float32 with insufficient data", func(t *testing.T) {
		data := []byte{0xFA, 0x00, 0x00} // Float32 needs 4 bytes
		var v float32
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode float64 with insufficient data", func(t *testing.T) {
		data := []byte{0xFB, 0x00, 0x00, 0x00} // Float64 needs 8 bytes
		var v float64
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})
}

package surrealcbor

import (
	"math"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_simple tests decoding of simple values
// like booleans, null, and undefined
func TestDecode_simple(t *testing.T) {
	t.Run("decode simple value false", func(t *testing.T) {
		data := []byte{0xF4} // false
		var v bool
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.False(t, v)
	})

	t.Run("decode simple value true", func(t *testing.T) {
		data := []byte{0xF5} // true
		var v bool
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.True(t, v)
	})

	t.Run("decode simple value null", func(t *testing.T) {
		data := []byte{0xF6} // null
		var v *int
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("decode simple value undefined", func(t *testing.T) {
		data := []byte{0xF7} // undefined
		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("decode simple value with byte", func(t *testing.T) {
		data := []byte{0xF8, 0x20} // Simple value 32
		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		// Should decode as simple value (unassigned)
		assert.NotNil(t, v)
	})

	t.Run("decode unassigned simple value", func(t *testing.T) {
		data := []byte{0xE0} // Unassigned simple value (0-19)
		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		// Should decode as unassigned simple value
		assert.NotNil(t, v)
	})
}

// TestDecode_float tests float16 decoding
func TestDecode_float(t *testing.T) {
	t.Run("decode float16 zero", func(t *testing.T) {
		data := []byte{0xF9, 0x00, 0x00} // float16(0.0)
		var v float32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, float32(0.0), v)
	})

	t.Run("decode float16 positive infinity", func(t *testing.T) {
		data := []byte{0xF9, 0x7C, 0x00} // float16(+Inf)
		var v float32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.True(t, math.IsInf(float64(v), 1))
	})

	t.Run("decode float16 negative infinity", func(t *testing.T) {
		data := []byte{0xF9, 0xFC, 0x00} // float16(-Inf)
		var v float32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.True(t, math.IsInf(float64(v), -1))
	})

	t.Run("decode float16 NaN", func(t *testing.T) {
		data := []byte{0xF9, 0x7E, 0x00} // float16(NaN)
		var v float32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.True(t, math.IsNaN(float64(v)))
	})

	t.Run("decode float16 normal number", func(t *testing.T) {
		data := []byte{0xF9, 0x3C, 0x00} // float16(1.0)
		var v float32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, float32(1.0), v)
	})

	t.Run("decode float16 subnormal number", func(t *testing.T) {
		data := []byte{0xF9, 0x00, 0x01} // Smallest positive subnormal
		var v float32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Greater(t, v, float32(0))
	})

	t.Run("decode float16 to float64", func(t *testing.T) {
		data := []byte{0xF9, 0x3C, 0x00} // float16(1.0)
		var v float64
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, 1.0, v)
	})

	t.Run("decode float16 to interface", func(t *testing.T) {
		data := []byte{0xF9, 0x3C, 0x00} // float16(1.0)
		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, float32(1.0), v)
	})
}

// TestDecode_floatToInterfaceWithExistingValue tests that decoding floats
// to interface{} correctly replaces the existing value with the float
func TestDecode_floatToInterfaceWithExistingValue(t *testing.T) {
	t.Run("decode float64 to interface containing int64", func(t *testing.T) {
		floatVal := 123.456
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		// Start with an interface containing an int64
		var v any = int64(42)
		err = Unmarshal(enc, &v)
		require.NoError(t, err)

		// Should be replaced with float64, not remain as int64
		f, ok := v.(float64)
		require.True(t, ok, "expected float64, got %T with value %v", v, v)
		assert.InDelta(t, floatVal, f, 0.000001)
	})

	t.Run("decode float32 to interface containing int64", func(t *testing.T) {
		floatVal := float32(45.67)
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		var v any = int64(42)
		err = Unmarshal(enc, &v)
		require.NoError(t, err)

		// Should be replaced with float32
		f, ok := v.(float32)
		require.True(t, ok, "expected float32, got %T with value %v", v, v)
		assert.InDelta(t, floatVal, f, 0.00001)
	})

	t.Run("decode float16 to interface containing string", func(t *testing.T) {
		data := []byte{0xF9, 0x3C, 0x00}

		var v any = "hello"
		err := Unmarshal(data, &v)
		require.NoError(t, err)

		// Should be replaced with float32 (float16 gets promoted)
		f, ok := v.(float32)
		require.True(t, ok, "expected float32, got %T with value %v", v, v)
		assert.Equal(t, float32(1.0), f)
	})

	t.Run("decode float64 to int64 directly should error", func(t *testing.T) {
		floatVal := 123.456
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		var i int64 = 42
		err = Unmarshal(enc, &i)
		// Should return an error for type mismatch
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float64 into Go value of type int64")
	})

	t.Run("decode float32 to string directly should error", func(t *testing.T) {
		floatVal := float32(45.67)
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		s := "hello"
		err = Unmarshal(enc, &s)
		// Should return an error for type mismatch
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float32 into Go value of type string")
	})

	t.Run("decode float16 to bool directly should error", func(t *testing.T) {
		data := []byte{0xF9, 0x3C, 0x00}

		var b bool
		err := Unmarshal(data, &b)
		// Should return an error for type mismatch
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float16 into Go value of type bool")
	})
}

// TestStdlibCBOR_floatDecoding tests how the standard cbor library handles float decoding
// to verify our implementation matches expected behavior
func TestStdlibCBOR_floatDecoding(t *testing.T) {
	t.Run("stdlib decode float to int64", func(t *testing.T) {
		floatVal := 3.14159
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		var i int64
		err = cbor.Unmarshal(enc, &i)
		// Standard library returns an error for type mismatch
		assert.Error(t, err)
	})

	t.Run("stdlib decode float to string", func(t *testing.T) {
		floatVal := 3.14159
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		var s string
		err = cbor.Unmarshal(enc, &s)
		// Standard library returns an error for type mismatch
		assert.Error(t, err)
	})

	t.Run("stdlib decode float to interface", func(t *testing.T) {
		floatVal := 3.14159
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		var v any = int64(42)
		err = cbor.Unmarshal(enc, &v)
		require.NoError(t, err)

		// Should be replaced with float64
		f, ok := v.(float64)
		require.True(t, ok, "expected float64, got %T", v)
		assert.InDelta(t, floatVal, f, 0.000001)
	})
}

// TestDecode_floatInterfacePointerIssue tests the specific case mentioned by the user
func TestDecode_floatInterfacePointerIssue(t *testing.T) {
	t.Run("interface pointing to int64 gets float64 value", func(t *testing.T) {
		floatVal := 789.012
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		// This simulates a case where we have interface{} that points to an int64
		var i int64 = 42
		var v any = &i

		err = Unmarshal(enc, &v)
		require.NoError(t, err)

		// After unmarshal, v should contain a float64, not a pointer to int64
		f, ok := v.(float64)
		require.True(t, ok, "expected float64, got %T", v)
		assert.InDelta(t, floatVal, f, 0.000001)

		// Original int64 should be unchanged
		assert.Equal(t, int64(42), i)
	})

	t.Run("pointer to interface containing int64", func(t *testing.T) {
		floatVal := 456.789
		enc, err := cbor.Marshal(floatVal)
		require.NoError(t, err)

		var v any = int64(42)
		pv := &v

		err = Unmarshal(enc, pv)
		require.NoError(t, err)

		// v should now contain float64
		f, ok := v.(float64)
		require.True(t, ok, "expected float64, got %T", v)
		assert.InDelta(t, floatVal, f, 0.000001)
	})
}

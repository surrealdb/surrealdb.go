package surrealcbor

import (
	"math"
	"testing"

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

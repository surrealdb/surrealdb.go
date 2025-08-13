package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_negativeInt tests negative integer decoding
func TestDecode_negativeInt(t *testing.T) {
	t.Run("decode negative int to int", func(t *testing.T) {
		data := []byte{0x20} // -1
		var v int
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, -1, v)
	})

	t.Run("decode negative int to int8", func(t *testing.T) {
		data := []byte{0x37} // -24
		var v int8
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, int8(-24), v)
	})

	t.Run("decode negative int to int16", func(t *testing.T) {
		data := []byte{0x38, 0xFF} // -256
		var v int16
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, int16(-256), v)
	})

	t.Run("decode negative int to int32", func(t *testing.T) {
		data := []byte{0x39, 0xFF, 0xFF} // -65536
		var v int32
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, int32(-65536), v)
	})

	t.Run("decode negative int to int64", func(t *testing.T) {
		data := []byte{0x3A, 0xFF, 0xFF, 0xFF, 0xFF} // -4294967296
		var v int64
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, int64(-4294967296), v)
	})

	t.Run("decode negative int to float32", func(t *testing.T) {
		data := []byte{0x29} // -10
		var v float32
		err := Unmarshal(data, &v)
		// Current implementation doesn't support negative int to float
		assert.Error(t, err)
	})

	t.Run("decode negative int to float64", func(t *testing.T) {
		data := []byte{0x29} // -10
		var v float64
		err := Unmarshal(data, &v)
		// Current implementation doesn't support negative int to float
		assert.Error(t, err)
	})

	t.Run("decode negative int to interface", func(t *testing.T) {
		data := []byte{0x29} // -10
		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, int64(-10), v)
	})

	t.Run("decode negative int overflow", func(t *testing.T) {
		// Try to decode a large negative number into int8
		data := []byte{0x38, 0xFF} // -256 (too large for int8)
		var v int8
		err := Unmarshal(data, &v)
		// Current implementation returns an overflow error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "overflow")
	})
}

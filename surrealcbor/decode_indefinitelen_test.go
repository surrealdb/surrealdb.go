package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_indefiniteLength tests indefinite length arrays and maps
func TestDecode_indefiniteLength(t *testing.T) {
	t.Run("decode indefinite length array", func(t *testing.T) {
		// CBOR indefinite length array: 0x9F (items) 0xFF
		data := []byte{0x9F, 0x01, 0x02, 0x03, 0xFF} // [1, 2, 3]

		var v []int
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, v)
	})

	t.Run("decode indefinite length map", func(t *testing.T) {
		// CBOR indefinite length map: 0xBF (key-value pairs) 0xFF
		data := []byte{0xBF, 0x61, 0x61, 0x01, 0x61, 0x62, 0x02, 0xFF} // {"a": 1, "b": 2}

		var v map[string]int
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, 1, v["a"])
		assert.Equal(t, 2, v["b"])
	})

	t.Run("decode indefinite length text string", func(t *testing.T) {
		// CBOR indefinite length text string: 0x7F (chunks) 0xFF
		// 0x7F = start indefinite string
		// 0x62 = 2-byte text string
		// "he" = first chunk
		// 0x63 = 3-byte text string
		// "llo" = second chunk
		// 0xFF = break
		data := []byte{0x7F, 0x62, 0x68, 0x65, 0x63, 0x6C, 0x6C, 0x6F, 0xFF}

		var s string
		err := Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Equal(t, "hello", s)
	})

	t.Run("decode indefinite length byte string", func(t *testing.T) {
		// CBOR indefinite length byte string: 0x5F (chunks) 0xFF
		data := []byte{0x5F, 0x42, 0x01, 0x02, 0x42, 0x03, 0x04, 0xFF} // byte chunks [1,2] + [3,4]

		var b []byte
		err := Unmarshal(data, &b)
		require.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3, 4}, b)
	})

	t.Run("decode indefinite length map to struct", func(t *testing.T) {
		type TestStruct struct {
			A int `json:"a"`
			B int `json:"b"`
		}

		// Indefinite length map
		data := []byte{0xBF, 0x61, 0x61, 0x01, 0x61, 0x62, 0x02, 0xFF}

		var s TestStruct
		err := Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Equal(t, 1, s.A)
		assert.Equal(t, 2, s.B)
	})

	t.Run("decode indefinite length map to interface", func(t *testing.T) {
		// Indefinite length map
		data := []byte{0xBF, 0x61, 0x61, 0x01, 0x61, 0x62, 0x02, 0xFF}

		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		m, ok := v.(map[string]any)
		assert.True(t, ok)
		// Values are decoded as uint64 for positive integers
		assert.Equal(t, uint64(1), m["a"])
		assert.Equal(t, uint64(2), m["b"])
	})

	t.Run("decode indefinite array with error", func(t *testing.T) {
		// Indefinite array with truncated element
		data := []byte{0x9F, 0x01, 0x1A} // Missing break and incomplete uint32

		var v []int
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode indefinite map with error", func(t *testing.T) {
		// Indefinite map with truncated value
		data := []byte{0xBF, 0x61, 0x61, 0x1A} // Missing break and incomplete uint32

		var v map[string]int
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode indefinite string with non-string chunk", func(t *testing.T) {
		// Indefinite string with integer chunk (invalid)
		data := []byte{0x7F, 0x01, 0xFF}

		var s string
		err := Unmarshal(data, &s)
		assert.Error(t, err)
	})

	t.Run("decode indefinite bytes with non-bytes chunk", func(t *testing.T) {
		// Indefinite bytes with string chunk (invalid)
		data := []byte{0x5F, 0x61, 0x61, 0xFF}

		var b []byte
		err := Unmarshal(data, &b)
		assert.Error(t, err)
	})
}

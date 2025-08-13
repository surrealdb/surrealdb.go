package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecode_map(t *testing.T) {
	t.Run("decode map to struct", func(t *testing.T) {
		type TestStruct struct {
			Field1 string `json:"field1"`
			Field2 int    `json:"field2"`
		}

		data, _ := cbor.Marshal(map[string]any{
			"field1": "value",
			"field2": 42,
		})

		var s TestStruct
		err := Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Equal(t, "value", s.Field1)
		assert.Equal(t, 42, s.Field2)
	})

	t.Run("decode map to interface", func(t *testing.T) {
		data, _ := cbor.Marshal(map[string]any{
			"key": "value",
		})

		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		m, ok := v.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "value", m["key"])
	})

	t.Run("decode map with non-string key", func(t *testing.T) {
		// CBOR map with integer key
		data := []byte{0xA1, 0x01, 0x02} // {1: 2}

		var v map[int]int
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		assert.Equal(t, 2, v[1])
	})

	t.Run("decode map with key decode error", func(t *testing.T) {
		// Map with truncated key
		data := []byte{0xA1, 0x62, 0x61} // {"a...: incomplete key
		var v map[string]int
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("None in map values", func(t *testing.T) {
		m := map[string]any{
			"key1": "value1",
			"key2": models.None,
			"key3": nil,
		}

		data, err := Marshal(m)
		require.NoError(t, err, "Marshal map failed")

		var decodedMap map[string]any
		err = Unmarshal(data, &decodedMap)
		require.NoError(t, err, "Unmarshal map failed")

		assert.Equal(t, "value1", decodedMap["key1"], "Map key1 mismatch")
		assert.Nil(t, decodedMap["key2"], "Map key2 should be nil (from None)")
		assert.Nil(t, decodedMap["key3"], "Map key3 should be nil")
	})
}

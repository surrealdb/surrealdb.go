package surrealcbor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalWithDifferentMapTypes(t *testing.T) {
	t.Run("default uses map[string]any", func(t *testing.T) {
		data, err := Marshal(map[string]any{
			"key1": "value1",
			"key2": 42,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)

		m, ok := result.(map[string]any)
		assert.True(t, ok, "Expected map[string]any")
		assert.Equal(t, "value1", m["key1"])
		assert.Equal(t, uint64(42), m["key2"])
	})

	t.Run("custom map[any]any", func(t *testing.T) {
		data, err := Marshal(map[string]any{
			"key1": "value1",
			"key2": 42,
		})
		require.NoError(t, err)

		var result any
		opts := UnmarshalOptions{
			DefaultMapType: map[any]any{},
		}
		err = UnmarshalWithOptions(data, &result, opts)
		require.NoError(t, err)

		m, ok := result.(map[any]any)
		assert.True(t, ok, "Expected map[any]any")
		assert.Equal(t, "value1", m["key1"])
		assert.Equal(t, uint64(42), m["key2"])
	})

	t.Run("custom map[string]string", func(t *testing.T) {
		data, err := Marshal(map[string]any{
			"key1": "value1",
			"key2": "value2",
		})
		require.NoError(t, err)

		var result any
		opts := UnmarshalOptions{
			DefaultMapType: map[string]string{},
		}
		err = UnmarshalWithOptions(data, &result, opts)
		require.NoError(t, err)

		m, ok := result.(map[string]string)
		assert.True(t, ok, "Expected map[string]string")
		assert.Equal(t, "value1", m["key1"])
		assert.Equal(t, "value2", m["key2"])
	})

	t.Run("nested maps use same type", func(t *testing.T) {
		data, err := Marshal(map[string]any{
			"outer": map[string]any{
				"inner": "value",
			},
		})
		require.NoError(t, err)

		var result any
		opts := UnmarshalOptions{
			DefaultMapType: map[any]any{},
		}
		err = UnmarshalWithOptions(data, &result, opts)
		require.NoError(t, err)

		outer, ok := result.(map[any]any)
		assert.True(t, ok, "Expected outer map to be map[any]any")

		inner, ok := outer["outer"].(map[any]any)
		assert.True(t, ok, "Expected inner map to be map[any]any")
		assert.Equal(t, "value", inner["inner"])
	})

	t.Run("error on non-map type", func(t *testing.T) {
		data, err := Marshal(map[string]any{"key": "value"})
		require.NoError(t, err)

		var result any
		opts := UnmarshalOptions{
			DefaultMapType: "not a map",
		}
		err = UnmarshalWithOptions(data, &result, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a map type")
	})

	t.Run("nil DefaultMapType uses default", func(t *testing.T) {
		data, err := Marshal(map[string]any{
			"key": "value",
		})
		require.NoError(t, err)

		var result any
		opts := UnmarshalOptions{
			DefaultMapType: nil,
		}
		err = UnmarshalWithOptions(data, &result, opts)
		require.NoError(t, err)

		m, ok := result.(map[string]any)
		assert.True(t, ok, "Expected map[string]any when DefaultMapType is nil")
		assert.Equal(t, "value", m["key"])
	})
}

func TestDecoderSetDefaultMapType(t *testing.T) {
	t.Run("set custom map type on decoder", func(t *testing.T) {
		// First encode the data
		data, err := Marshal(map[string]any{
			"key": "value",
		})
		require.NoError(t, err)

		// Create decoder with the encoded data
		dec := NewDecoder(bytes.NewReader(data))
		err = dec.SetDefaultMapType(map[any]any{})
		require.NoError(t, err)

		var result any
		err = dec.Decode(&result)
		require.NoError(t, err)

		m, ok := result.(map[any]any)
		assert.True(t, ok, "Expected map[any]any after SetDefaultMapType")
		assert.Equal(t, "value", m["key"])
	})

	t.Run("error on non-map type", func(t *testing.T) {
		dec := NewDecoder(bytes.NewReader([]byte{}))
		err := dec.SetDefaultMapType("not a map")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires a map type")
	})

	t.Run("nil resets to default", func(t *testing.T) {
		dec := NewDecoder(bytes.NewReader([]byte{}))
		err := dec.SetDefaultMapType(nil)
		assert.NoError(t, err)
		// Should use default map[string]any
	})
}

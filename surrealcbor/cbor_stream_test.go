package surrealcbor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestStreamingEncoderDecoder tests the streaming encoder/decoder
func TestStreamingEncoderDecoder(t *testing.T) {
	buf := &bytes.Buffer{}

	// Create encoder
	enc := NewEncoder(buf)

	// Encode multiple values
	values := []any{
		"hello",
		42,
		map[string]any{"key": "value"},
		models.None,
	}

	for _, v := range values {
		err := enc.Encode(v)
		require.NoErrorf(t, err, "Encode failed for %v", v)
	}

	// Create decoder
	dec := NewDecoder(buf)

	// Decode and verify each value
	t.Run("decode string", func(t *testing.T) {
		var decoded any
		err := dec.Decode(&decoded)
		require.NoError(t, err, "Decode failed for string")
		assert.Equal(t, "hello", decoded, "String value mismatch")
	})

	t.Run("decode number", func(t *testing.T) {
		var decoded any
		err := dec.Decode(&decoded)
		require.NoError(t, err, "Decode failed for number")

		// Handle both uint64 and int64
		switch v := decoded.(type) {
		case uint64:
			assert.Equal(t, uint64(42), v, "Number value mismatch")
		case int64:
			assert.Equal(t, int64(42), v, "Number value mismatch")
		default:
			t.Errorf("Number type mismatch: got %T, want numeric", decoded)
		}
	})

	t.Run("decode map", func(t *testing.T) {
		var decoded any
		err := dec.Decode(&decoded)
		require.NoError(t, err, "Decode failed for map")

		m, ok := decoded.(map[string]any)
		require.True(t, ok, "Expected map[string]any")
		assert.Equal(t, "value", m["key"], "Map value mismatch")
	})

	t.Run("decode None as nil", func(t *testing.T) {
		var decoded any
		err := dec.Decode(&decoded)
		require.NoError(t, err, "Decode failed for None")
		assert.Nil(t, decoded, "None should decode to nil")
	})
}

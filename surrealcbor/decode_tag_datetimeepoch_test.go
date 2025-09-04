package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_customDateTimeEpoch tests decoding of CustomDateTime (tag 12)
func TestDecode_customDateTimeEpoch(t *testing.T) {
	t.Run("decode tag 12 (CustomDateTime) with array format", func(t *testing.T) {
		// Tag 12 with [seconds, nanoseconds] array
		// 0xCC = tag 12, 0x82 = array of 2, 0x3A = negative int follows, then -2, then 0
		data := []byte{0xCC, 0x82, 0x3A, 0x00, 0x00, 0x00, 0x01, 0x00} // [-2 seconds, 0 nanoseconds]

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		require.NoError(t, err)
		assert.NotZero(t, dt.Time)
		// Should be 2 seconds before epoch
		assert.Equal(t, int64(-2), dt.Unix())
	})

	t.Run("decode tag 12 to interface", func(t *testing.T) {
		// Tag 12 with [seconds, nanoseconds] array
		data := []byte{0xCC, 0x82, 0x1A, 0x5F, 0x5E, 0x0F, 0xF0, 0x00} // [timestamp, 0]

		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		// Tag 12 is decoded as CustomDateTime when to interface
		_, ok := v.(models.CustomDateTime)
		assert.True(t, ok, "expected CustomDateTime, got %T", v)
	})

	t.Run("decode tag 12 with invalid content type", func(t *testing.T) {
		// Tag 12 with string (invalid for CustomDateTime which expects array)
		data := []byte{0xCC, 0x67} // Tag 12 with 7-byte string
		data = append(data, []byte("invalid")...)

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		// Should return error for invalid content
		require.Error(t, err)
	})
}

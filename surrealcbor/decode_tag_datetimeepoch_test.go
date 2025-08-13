package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_customDateTimeEpoch tests decoding of Tag 1 date time
func TestDecode_customDateTimeEpoch(t *testing.T) {
	t.Run("decode datetime tag with negative number", func(t *testing.T) {
		// Tag 1 with negative number (before epoch)
		data := []byte{0xC1, 0x3A, 0x00, 0x00, 0x00, 0x01} // -2 seconds

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		require.NoError(t, err)
	})

	t.Run("decode datetime tag to interface", func(t *testing.T) {
		// Tag 1 with Unix timestamp
		data := []byte{0xC1, 0x1A, 0x5F, 0x5E, 0x0F, 0xF0}

		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		// DateTime tag is decoded as CustomDateTime, not time.Time when to interface
		assert.NotNil(t, v)
	})

	t.Run("decode datetime tag with invalid content type", func(t *testing.T) {
		// Tag 1 with array (invalid for datetime)
		data := []byte{0xC1, 0x82, 0x01, 0x02} // Tag 1 with [1, 2]

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		// Should handle gracefully
		require.NoError(t, err)
	})
}

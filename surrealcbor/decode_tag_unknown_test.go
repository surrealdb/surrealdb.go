package surrealcbor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecode_unknownTag(t *testing.T) {
	t.Run("decode unknown tag", func(t *testing.T) {
		// Unknown tag number
		data := []byte{0xD8, 0x64, 0x61, 0x61} // Tag 100 with "a"

		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		// Should decode the content, ignoring unknown tag
		assert.Equal(t, "a", v)
	})

	t.Run("decode datetime tag to time.Time", func(t *testing.T) {
		// Tag 1 (datetime epoch)
		data := []byte{0xC1, 0x1A, 0x5F, 0x5E, 0x0F, 0xF0} // Tag 1 with Unix timestamp

		var tm time.Time
		err := Unmarshal(data, &tm)
		require.NoError(t, err)
	})

	t.Run("decode datetime tag with float", func(t *testing.T) {
		// Tag 1 with float64
		data := []byte{0xC1, 0xFB, 0x41, 0xD7, 0x97, 0x8B, 0xFC, 0x00, 0x00, 0x00}

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		require.NoError(t, err)
	})
}

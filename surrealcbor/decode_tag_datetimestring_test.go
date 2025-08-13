package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecode_datetimestring(t *testing.T) {
	t.Run("decode datetime string tag", func(t *testing.T) {
		// Tag 0 (datetime string)
		data := []byte{0xC0, 0x78, 0x19} // Tag 0 with 25-byte string
		data = append(data, []byte("2024-01-01T00:00:00+00:00")...)

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		require.NoError(t, err)
	})

	t.Run("decode tag with no data", func(t *testing.T) {
		data := []byte{0xC0} // Tag 0 with no following data
		var v any
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode tag with error in content", func(t *testing.T) {
		// Tag with truncated content
		data := []byte{0xC0, 0x1A} // Tag 0 with incomplete uint32

		var v any
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})

	t.Run("decode datetime string tag with invalid format", func(t *testing.T) {
		// Tag 0 with invalid datetime string
		data := []byte{0xC0, 0x67, 0x69, 0x6E, 0x76, 0x61, 0x6C, 0x69, 0x64} // "invalid"

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		// Invalid format returns an error
		assert.Error(t, err)
	})
}

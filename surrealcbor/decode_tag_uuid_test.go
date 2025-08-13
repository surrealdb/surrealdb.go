package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_uuid tests decoding of UUID
func TestDecode_uuid(t *testing.T) {
	t.Run("decode uuid tag", func(t *testing.T) {
		// Tag 37 (UUID)
		uuidBytes := make([]byte, 16)
		for i := range uuidBytes {
			uuidBytes[i] = byte(i)
		}
		data := []byte{0xD8, 0x25, 0x50} // Tag 37 with 16 bytes
		data = append(data, uuidBytes...)

		var uuid models.UUID
		err := Unmarshal(data, &uuid)
		require.NoError(t, err)
	})

	t.Run("decode uuid tag with wrong byte length", func(t *testing.T) {
		// Tag 37 with wrong number of bytes
		data := []byte{0xD8, 0x25, 0x44, 0x01, 0x02, 0x03, 0x04} // Only 4 bytes

		var uuid models.UUID
		err := Unmarshal(data, &uuid)
		// Wrong byte length
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "16 bytes")
	})

	t.Run("decode uuid tag with non-bytes", func(t *testing.T) {
		// Tag 37 with string instead of bytes
		data := []byte{0xD8, 0x25, 0x61, 0x61} // "a"

		var uuid models.UUID
		err := Unmarshal(data, &uuid)
		// Expected byte string
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "byte string")
	})
}

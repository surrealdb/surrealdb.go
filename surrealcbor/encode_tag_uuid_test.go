package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestEncode_uuid tests encoding of Tag 37 (UUID)
func TestEncode_uuid(t *testing.T) {
	t.Run("encode models.UUID", func(t *testing.T) {
		// Create UUID from bytes
		uuidBytes := []byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		}
		u, err := uuid.FromBytes(uuidBytes)
		require.NoError(t, err)
		modelUUID := models.UUID{UUID: u}

		enc, err := Marshal(modelUUID)
		require.NoError(t, err)

		// Decode the raw CBOR to check the tag
		var tag cbor.Tag
		err = cbor.Unmarshal(enc, &tag)
		require.NoError(t, err)
		assert.Equal(t, uint64(37), tag.Number)

		// Verify the content is byte string
		bytes, ok := tag.Content.([]byte)
		require.True(t, ok, "expected byte string, got %T", tag.Content)
		assert.Len(t, bytes, 16)

		// Decode back to verify round-trip
		var decoded models.UUID
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, modelUUID, decoded)
	})

	t.Run("round-trip zero UUID", func(t *testing.T) {
		modelUUID := models.UUID{UUID: uuid.Nil} // All zeros

		enc, err := Marshal(modelUUID)
		require.NoError(t, err)

		// Verify tag structure
		var tag cbor.Tag
		err = cbor.Unmarshal(enc, &tag)
		require.NoError(t, err)
		assert.Equal(t, uint64(37), tag.Number)

		bytes, ok := tag.Content.([]byte)
		require.True(t, ok, "expected byte string, got %T", tag.Content)
		assert.Len(t, bytes, 16)
		for i, b := range bytes {
			assert.Equal(t, byte(0), b, "byte at index %d should be 0", i)
		}

		// Decode back
		var decoded models.UUID
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, modelUUID, decoded)
	})

	t.Run("encode UUID with all bytes set", func(t *testing.T) {
		// Create UUID with all bytes set to 0xff
		uuidBytes := []byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		}
		u, err := uuid.FromBytes(uuidBytes)
		require.NoError(t, err)
		modelUUID := models.UUID{UUID: u}

		enc, err := Marshal(modelUUID)
		require.NoError(t, err)

		// Verify tag structure
		var tag cbor.Tag
		err = cbor.Unmarshal(enc, &tag)
		require.NoError(t, err)
		assert.Equal(t, uint64(37), tag.Number)

		bytes, ok := tag.Content.([]byte)
		require.True(t, ok, "expected byte string, got %T", tag.Content)
		assert.Len(t, bytes, 16)
		for i, b := range bytes {
			assert.Equal(t, byte(0xff), b, "byte at index %d should be 0xff", i)
		}

		// Decode back
		var decoded models.UUID
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, modelUUID, decoded)
	})

	t.Run("encode various UUIDs", func(t *testing.T) {
		testCases := []struct {
			name string
			uuid models.UUID
		}{
			{
				name: "sequential bytes",
				uuid: func() models.UUID {
					b := []byte{
						0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
						0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
					}
					u, _ := uuid.FromBytes(b)
					return models.UUID{UUID: u}
				}(),
			},
			{
				name: "version 4 UUID pattern",
				uuid: func() models.UUID {
					b := []byte{
						0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0x4d, 0xef,
						0x81, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
					}
					u, _ := uuid.FromBytes(b)
					return models.UUID{UUID: u}
				}(),
			},
			{
				name: "alternating pattern",
				uuid: func() models.UUID {
					b := []byte{
						0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55,
						0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55,
					}
					u, _ := uuid.FromBytes(b)
					return models.UUID{UUID: u}
				}(),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				enc, err := Marshal(tc.uuid)
				require.NoError(t, err)

				// Verify tag structure
				var tag cbor.Tag
				err = cbor.Unmarshal(enc, &tag)
				require.NoError(t, err)
				assert.Equal(t, uint64(37), tag.Number)

				bytes, ok := tag.Content.([]byte)
				require.True(t, ok, "expected byte string, got %T", tag.Content)
				assert.Len(t, bytes, 16)

				// Verify round-trip
				var decoded models.UUID
				err = Unmarshal(enc, &decoded)
				require.NoError(t, err)
				assert.Equal(t, tc.uuid, decoded)
			})
		}
	})

	t.Run("encode UUID matches expected CBOR structure", func(t *testing.T) {
		// Create UUID from specific bytes
		uuidBytes := []byte{
			0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
			0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
		}
		u, err := uuid.FromBytes(uuidBytes)
		require.NoError(t, err)
		modelUUID := models.UUID{UUID: u}

		enc, err := Marshal(modelUUID)
		require.NoError(t, err)

		// The CBOR encoding should be:
		// 0xD8, 0x25 (Tag 37)
		// 0x50 (16-byte byte string)
		// followed by the 16 UUID bytes
		assert.Greater(t, len(enc), 18) // At least tag + length + 16 bytes
		assert.Equal(t, byte(0xD8), enc[0])
		assert.Equal(t, byte(0x25), enc[1])
		assert.Equal(t, byte(0x50), enc[2])

		// Verify the UUID bytes match
		uBytes := modelUUID.Bytes()
		for i := 0; i < 16; i++ {
			assert.Equal(t, uBytes[i], enc[3+i], "UUID byte at index %d mismatch", i)
		}
	})
}

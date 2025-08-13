package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecoder_readuint tests various uint reading scenarios
func TestDecoder_readuint(t *testing.T) {
	t.Run("readUint with 1-byte extension (24)", func(t *testing.T) {
		// CBOR uint with additional info = 24 (1 byte follows)
		data := []byte{0x18, 0xFF} // uint8 255
		d := &decoder{data: data, pos: 0}
		val, err := d.readUint()
		require.NoError(t, err)
		assert.Equal(t, uint64(255), val)
	})

	t.Run("readUint with 2-byte extension (25)", func(t *testing.T) {
		// CBOR uint with additional info = 25 (2 bytes follow)
		data := []byte{0x19, 0xFF, 0xFF} // uint16 65535
		d := &decoder{data: data, pos: 0}
		val, err := d.readUint()
		require.NoError(t, err)
		assert.Equal(t, uint64(65535), val)
	})

	t.Run("readUint with 4-byte extension (26)", func(t *testing.T) {
		// CBOR uint with additional info = 26 (4 bytes follow)
		data := []byte{0x1A, 0xFF, 0xFF, 0xFF, 0xFF} // uint32 max
		d := &decoder{data: data, pos: 0}
		val, err := d.readUint()
		require.NoError(t, err)
		assert.Equal(t, uint64(4294967295), val)
	})

	t.Run("readUint with 8-byte extension (27)", func(t *testing.T) {
		// CBOR uint with additional info = 27 (8 bytes follow)
		data := []byte{0x1A, 0xFF, 0xFF, 0xFF, 0xFF} // uint32 max
		d := &decoder{data: data, pos: 0}
		val, err := d.readUint()
		require.NoError(t, err)
		assert.Equal(t, uint64(4294967295), val)
	})

	t.Run("readUint with insufficient data for 1-byte", func(t *testing.T) {
		data := []byte{0x18} // Missing the actual byte
		d := &decoder{data: data, pos: 0}
		_, err := d.readUint()
		assert.Error(t, err)
	})

	t.Run("readUint with insufficient data for 2-byte", func(t *testing.T) {
		data := []byte{0x19, 0xFF} // Missing second byte
		d := &decoder{data: data, pos: 0}
		_, err := d.readUint()
		assert.Error(t, err)
	})

	t.Run("readUint with insufficient data for 4-byte", func(t *testing.T) {
		data := []byte{0x1A, 0xFF, 0xFF} // Missing bytes
		d := &decoder{data: data, pos: 0}
		_, err := d.readUint()
		assert.Error(t, err)
	})

	t.Run("readUint with insufficient data for 8-byte", func(t *testing.T) {
		data := []byte{0x1B, 0xFF, 0xFF, 0xFF} // Missing bytes
		d := &decoder{data: data, pos: 0}
		_, err := d.readUint()
		assert.Error(t, err)
	})

	t.Run("readUint with invalid additional info 28", func(t *testing.T) {
		data := []byte{0x1C} // Additional info 28 (reserved)
		d := &decoder{data: data, pos: 0}
		_, err := d.readUint()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid additional info")
	})

	t.Run("readUint with invalid additional info 31", func(t *testing.T) {
		data := []byte{0x1F} // Additional info 31 (indefinite length)
		d := &decoder{data: data, pos: 0}
		_, err := d.readUint()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid additional info")
	})
}

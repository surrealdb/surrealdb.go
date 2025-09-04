package surrealcbor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const testISO8601String = "2024-01-01T12:00:00Z"

// TestDecode_datetimestring tests tag 0 (ISO 8601 datetime string) decoding
// Tag 0 is handled by our surrealcbor decoder's decodeDateTimeStringTag function
// which can unmarshal to time.Time, but NOT to CustomDateTime (which only handles tag 12)
func TestDecode_datetimestring(t *testing.T) {
	t.Run("decode tag 0 to time.Time", func(t *testing.T) {
		// Tag 0 with ISO 8601 string
		isoString := testISO8601String
		// 0xC0 = tag 0, 0x74 = text string of 20 bytes
		data := []byte{0xC0, 0x74}
		data = append(data, []byte(isoString)...)

		var tm time.Time
		err := Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, 2024, tm.Year())
		assert.Equal(t, time.January, tm.Month())
		assert.Equal(t, 1, tm.Day())
		assert.Equal(t, 12, tm.Hour())
	})

	t.Run("decode tag 0 to interface", func(t *testing.T) {
		// Tag 0 with ISO 8601 string
		isoString := testISO8601String
		data := []byte{0xC0, 0x74}
		data = append(data, []byte(isoString)...)

		var v any
		err := Unmarshal(data, &v)
		require.NoError(t, err)
		// Should be decoded as time.Time
		tm, ok := v.(time.Time)
		assert.True(t, ok, "expected time.Time, got %T", v)
		assert.Equal(t, 2024, tm.Year())
	})

	t.Run("decode tag 0 to CustomDateTime fails", func(t *testing.T) {
		// Tag 0 with ISO 8601 string - CustomDateTime only handles tag 12
		isoString := testISO8601String
		data := []byte{0xC0, 0x74}
		data = append(data, []byte(isoString)...)

		var dt models.CustomDateTime
		err := Unmarshal(data, &dt)
		// Should fail because CustomDateTime.UnmarshalCBOR only handles tag 12
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected tag number: got 0, want 12")
	})

	t.Run("decode tag 0 with invalid ISO string", func(t *testing.T) {
		// Tag 0 with invalid ISO 8601 string
		data := []byte{0xC0, 0x67} // Tag 0 with 7-byte string
		data = append(data, []byte("invalid")...)

		var tm time.Time
		err := Unmarshal(data, &tm)
		// Should return error for invalid ISO 8601 format
		require.Error(t, err)
	})

	t.Run("decode tag 0 with truncated data", func(t *testing.T) {
		// Tag 0 with incomplete string length
		data := []byte{0xC0, 0x74} // Tag 0 with 20-byte string but no content

		var v any
		err := Unmarshal(data, &v)
		assert.Error(t, err)
	})
}

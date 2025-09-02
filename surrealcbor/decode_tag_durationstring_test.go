package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_durationString tests decoding of Tag 13 (CustomDurationString)
func TestDecode_durationString(t *testing.T) {
	t.Run("decode CustomDurationString tag", func(t *testing.T) {
		// Tag 13 (CustomDurationString - string format)
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: "1h30m",
		})

		var durStr models.CustomDurationString
		err := Unmarshal(enc, &durStr)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDurationString("1h30m"), durStr)
	})

	t.Run("decode CustomDurationString to interface", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: "2h45m30s",
		})

		var result any
		err := Unmarshal(enc, &result)
		require.NoError(t, err)
		durStr, ok := result.(models.CustomDurationString)
		require.True(t, ok, "expected models.CustomDurationString, got %T", result)
		assert.Equal(t, models.CustomDurationString("2h45m30s"), durStr)
	})

	t.Run("decode CustomDurationString with complex format", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: "1y2w3d4h5m6s7ms8us9ns",
		})

		var durStr models.CustomDurationString
		err := Unmarshal(enc, &durStr)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDurationString("1y2w3d4h5m6s7ms8us9ns"), durStr)
	})

	t.Run("decode CustomDurationString with zero duration", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: "0s",
		})

		var durStr models.CustomDurationString
		err := Unmarshal(enc, &durStr)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDurationString("0s"), durStr)
	})

	t.Run("decode CustomDurationString with negative duration", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: "-5m30s",
		})

		var durStr models.CustomDurationString
		err := Unmarshal(enc, &durStr)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDurationString("-5m30s"), durStr)
	})

	t.Run("decode CustomDurationString with non-string content", func(t *testing.T) {
		// Tag 13 with number (invalid)
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: 123,
		})

		var durStr models.CustomDurationString
		err := Unmarshal(enc, &durStr)
		// Should error as tag 13 expects string content
		assert.Error(t, err)
	})

	t.Run("decode CustomDurationString with empty string", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  13,
			Content: "",
		})

		var durStr models.CustomDurationString
		err := Unmarshal(enc, &durStr)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDurationString(""), durStr)
	})
}

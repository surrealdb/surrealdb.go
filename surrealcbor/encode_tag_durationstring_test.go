package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestEncode_durationString tests encoding of Tag 13 (CustomDurationString)
func TestEncode_durationString(t *testing.T) {
	t.Run("encode models.CustomDurationString", func(t *testing.T) {
		dur := models.CustomDurationString("1h30m")

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back to verify round-trip
		var decoded models.CustomDurationString
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur, decoded)
	})

	t.Run("encode CustomDurationString with complex format", func(t *testing.T) {
		dur := models.CustomDurationString("1y2w3d4h5m6s7ms8us9ns")

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back to verify
		var decoded models.CustomDurationString
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur, decoded)
	})

	t.Run("encode empty CustomDurationString", func(t *testing.T) {
		dur := models.CustomDurationString("")

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back to verify
		var decoded models.CustomDurationString
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur, decoded)
	})

	t.Run("encode negative CustomDurationString", func(t *testing.T) {
		dur := models.CustomDurationString("-5m30s")

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back to verify
		var decoded models.CustomDurationString
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur, decoded)
	})

	t.Run("encode zero CustomDurationString", func(t *testing.T) {
		dur := models.CustomDurationString("0s")

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back to verify
		var decoded models.CustomDurationString
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur, decoded)
	})
}

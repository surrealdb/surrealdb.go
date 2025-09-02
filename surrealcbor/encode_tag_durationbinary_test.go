package surrealcbor

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestEncode_durationBinary tests encoding of Tag 14 (CustomDuration - binary format)
func TestEncode_durationBinary(t *testing.T) {
	t.Run("encode models.CustomDuration", func(t *testing.T) {
		dur := models.CustomDuration{Duration: 90 * time.Minute}

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode the raw CBOR to check the tag
		var raw cbor.RawMessage
		err = cbor.Unmarshal(enc, &raw)
		require.NoError(t, err)

		// Decode back to verify round-trip
		var decoded models.CustomDuration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decoded.Duration)
	})

	t.Run("round-trip time.Duration through models.CustomDuration", func(t *testing.T) {
		originalDuration := 1*time.Hour + 30*time.Minute + 45*time.Second + 123*time.Nanosecond

		// Encode as CustomDuration
		dur := models.CustomDuration{Duration: originalDuration}
		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode as time.Duration
		var decoded time.Duration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, originalDuration, decoded)

		// Also decode as models.CustomDuration
		var decodedCustom models.CustomDuration
		err = Unmarshal(enc, &decodedCustom)
		require.NoError(t, err)
		assert.Equal(t, originalDuration, decodedCustom.Duration)
	})

	t.Run("encode negative duration", func(t *testing.T) {
		dur := models.CustomDuration{Duration: -5 * time.Second}

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back
		var decoded models.CustomDuration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decoded.Duration)

		// Also decode as time.Duration
		var decodedDuration time.Duration
		err = Unmarshal(enc, &decodedDuration)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decodedDuration)
	})

	t.Run("encode zero duration", func(t *testing.T) {
		dur := models.CustomDuration{Duration: 0}

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Current implementation always uses [2]int64{s, ns} format
		var tag cbor.Tag
		err = cbor.Unmarshal(enc, &tag)
		require.NoError(t, err)
		assert.Equal(t, uint64(14), tag.Number)

		arr, ok := tag.Content.([]interface{})
		require.True(t, ok, "expected array, got %T", tag.Content)
		// Current implementation: always 2 elements
		assert.Len(t, arr, 2, "expected two element array")

		// Decode back
		var decoded models.CustomDuration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decoded.Duration)
	})

	t.Run("encode whole seconds", func(t *testing.T) {
		dur := models.CustomDuration{Duration: 30 * time.Second}

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Current implementation always uses [2]int64{s, ns} format
		var tag cbor.Tag
		err = cbor.Unmarshal(enc, &tag)
		require.NoError(t, err)
		assert.Equal(t, uint64(14), tag.Number)

		arr, ok := tag.Content.([]interface{})
		require.True(t, ok, "expected array, got %T", tag.Content)
		// Current implementation: always 2 elements
		assert.Len(t, arr, 2, "expected two element array")

		// Decode back
		var decoded models.CustomDuration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decoded.Duration)
	})

	t.Run("encode duration with nanoseconds", func(t *testing.T) {
		dur := models.CustomDuration{Duration: 30*time.Second + 500*time.Millisecond}

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Current implementation always uses [2]int64{s, ns} format
		var tag cbor.Tag
		err = cbor.Unmarshal(enc, &tag)
		require.NoError(t, err)
		assert.Equal(t, uint64(14), tag.Number)

		arr, ok := tag.Content.([]interface{})
		require.True(t, ok, "expected array, got %T", tag.Content)
		// Current implementation: always 2 elements
		assert.Len(t, arr, 2, "expected two element array")

		// Decode back
		var decoded models.CustomDuration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decoded.Duration)
	})

	t.Run("encode maximum duration", func(t *testing.T) {
		// Test with maximum duration value
		maxDuration := time.Duration(1<<63 - 1) // max int64 nanoseconds
		dur := models.CustomDuration{Duration: maxDuration}

		enc, err := Marshal(dur)
		require.NoError(t, err)

		// Decode back
		var decoded models.CustomDuration
		err = Unmarshal(enc, &decoded)
		require.NoError(t, err)
		assert.Equal(t, dur.Duration, decoded.Duration)
	})

	t.Run("encode various durations", func(t *testing.T) {
		testCases := []struct {
			name     string
			duration time.Duration
		}{
			{"zero", 0},
			{"1 second", time.Second},
			{"1 minute", time.Minute},
			{"1 hour", time.Hour},
			{"1.5 seconds", time.Second + 500*time.Millisecond},
			{"1 nanosecond", time.Nanosecond},
			{"negative whole seconds", -10 * time.Second},
			{"negative with nanos", -time.Second - 500*time.Millisecond},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				dur := models.CustomDuration{Duration: tc.duration}
				enc, err := Marshal(dur)
				require.NoError(t, err)

				// Verify encoding uses [2]int64 format
				var tag cbor.Tag
				err = cbor.Unmarshal(enc, &tag)
				require.NoError(t, err)
				assert.Equal(t, uint64(14), tag.Number)

				arr, ok := tag.Content.([]interface{})
				require.True(t, ok, "expected array, got %T", tag.Content)
				// Current implementation: always 2 elements
				assert.Len(t, arr, 2, "expected two element array for %v", tc.duration)

				// Verify round-trip
				var decoded models.CustomDuration
				err = Unmarshal(enc, &decoded)
				require.NoError(t, err)
				assert.Equal(t, tc.duration, decoded.Duration)
			})
		}
	})
}

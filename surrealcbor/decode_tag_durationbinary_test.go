package surrealcbor

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_durationBinary tests decoding of Tag 14 (CustomDuration - binary format)
func TestDecode_durationBinary(t *testing.T) {
	t.Run("decode CustomDuration tag to models.CustomDuration", func(t *testing.T) {
		// Tag 14 (CustomDuration - binary format as [seconds, nanoseconds])
		expectedDuration := 90 * time.Minute // 1h30m
		seconds := int64(expectedDuration / time.Second)
		nanoseconds := int64(expectedDuration % time.Second)

		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{seconds, nanoseconds},
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, expectedDuration, dur.Duration)
	})

	t.Run("decode CustomDuration tag to time.Duration", func(t *testing.T) {
		// Tag 14 (CustomDuration - binary format as [seconds, nanoseconds])
		expectedDuration := 90 * time.Minute // 1h30m
		seconds := int64(expectedDuration / time.Second)
		nanoseconds := int64(expectedDuration % time.Second)

		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{seconds, nanoseconds},
		})

		var dur time.Duration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, expectedDuration, dur)
	})

	t.Run("decode CustomDuration tag to any", func(t *testing.T) {
		// Tag 14 (CustomDuration - binary format as [seconds, nanoseconds])
		expectedDuration := 90 * time.Minute // 1h30m
		seconds := int64(expectedDuration / time.Second)
		nanoseconds := int64(expectedDuration % time.Second)

		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{seconds, nanoseconds},
		})

		var result any
		err := Unmarshal(enc, &result)
		require.NoError(t, err)
		dur, ok := result.(models.CustomDuration)
		require.True(t, ok, "expected models.CustomDuration, got %T", result)
		assert.Equal(t, expectedDuration, dur.Duration)
	})

	t.Run("decode CustomDuration with empty array (zero duration)", func(t *testing.T) {
		// Per SurrealDB spec: empty array represents duration of 0
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{}, // Empty array = 0 duration
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), dur.Duration)

		// Also test with time.Duration
		var timeDur time.Duration
		err = Unmarshal(enc, &timeDur)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), timeDur)
	})

	t.Run("decode CustomDuration with single element (seconds only)", func(t *testing.T) {
		// Per SurrealDB spec: single element represents seconds only
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{30}, // 30 seconds
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, dur.Duration)

		// Also test with time.Duration
		var timeDur time.Duration
		err = Unmarshal(enc, &timeDur)
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, timeDur)
	})

	t.Run("decode CustomDuration with nanoseconds", func(t *testing.T) {
		// Test with a duration that has nanoseconds
		expectedDuration := 1*time.Second + 500*time.Millisecond + 123*time.Nanosecond
		seconds := int64(expectedDuration / time.Second)
		nanoseconds := int64(expectedDuration % time.Second)

		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{seconds, nanoseconds},
		})

		var dur time.Duration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, expectedDuration, dur)
	})

	t.Run("decode negative CustomDuration", func(t *testing.T) {
		// Test with a negative duration
		expectedDuration := -5 * time.Second
		seconds := int64(expectedDuration / time.Second)
		nanoseconds := int64(expectedDuration % time.Second)

		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{seconds, nanoseconds},
		})

		var dur time.Duration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, expectedDuration, dur)
	})

	t.Run("decode CustomDuration with non-array", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: 123,
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		assert.Error(t, err)
	})

	t.Run("decode CustomDuration with non-integer elements", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []string{"not", "numbers"},
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		assert.Error(t, err)
	})

	t.Run("decode CustomDuration with float elements", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []float64{1.5, 2.5},
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		// Should handle floats by converting to int64
		assert.Error(t, err)
	})

	t.Run("decode CustomDuration with large values", func(t *testing.T) {
		// Test with maximum duration values
		expectedDuration := time.Duration(1<<63 - 1) // max int64 nanoseconds
		seconds := int64(expectedDuration / time.Second)
		nanoseconds := int64(expectedDuration % time.Second)

		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  14,
			Content: []int64{seconds, nanoseconds},
		})

		var dur time.Duration
		err := Unmarshal(enc, &dur)
		require.NoError(t, err)
		assert.Equal(t, expectedDuration, dur)
	})
}

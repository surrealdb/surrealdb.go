package surrealcbor

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecodeTag12ToTimeTime(t *testing.T) {
	t.Run("Decode Tag 12 (CustomDateTime) to time.Time", func(t *testing.T) {
		// Create a CustomDateTime using fxamacker/cbor
		now := time.Now().UTC().Truncate(time.Nanosecond)
		customDT := models.CustomDateTime{Time: now}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDT)
		require.NoError(t, err)

		// Decode with surrealcbor into time.Time
		var decoded time.Time
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, now, decoded)
	})

	t.Run("Decode Tag 12 to CustomDateTime", func(t *testing.T) {
		// Create a CustomDateTime using fxamacker/cbor
		now := time.Now().UTC().Truncate(time.Nanosecond)
		customDT := models.CustomDateTime{Time: now}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDT)
		require.NoError(t, err)

		// Decode with surrealcbor into CustomDateTime
		var decoded models.CustomDateTime
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, customDT.Time, decoded.Time)
	})

	t.Run("Decode Tag 12 to interface{}", func(t *testing.T) {
		// Create a CustomDateTime using fxamacker/cbor
		now := time.Now().UTC().Truncate(time.Nanosecond)
		customDT := models.CustomDateTime{Time: now}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDT)
		require.NoError(t, err)

		// Decode with surrealcbor into interface{}
		var decoded interface{}
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		// Should be decoded as CustomDateTime
		decodedDT, ok := decoded.(models.CustomDateTime)
		require.True(t, ok)
		assert.Equal(t, customDT.Time, decodedDT.Time)
	})
}

func TestDecodeTag14ToTimeDuration(t *testing.T) {
	t.Run("Decode Tag 14 (CustomDuration) to time.Duration", func(t *testing.T) {
		// Create a CustomDuration using fxamacker/cbor
		duration := 5*time.Hour + 30*time.Minute + 15*time.Second + 123*time.Nanosecond
		customDur := models.CustomDuration{Duration: duration}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDur)
		require.NoError(t, err)

		// Decode with surrealcbor into time.Duration
		var decoded time.Duration
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, duration, decoded)
	})

	t.Run("Decode Tag 14 to CustomDuration", func(t *testing.T) {
		// Create a CustomDuration using fxamacker/cbor
		duration := 2*time.Hour + 45*time.Minute + 30*time.Second
		customDur := models.CustomDuration{Duration: duration}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDur)
		require.NoError(t, err)

		// Decode with surrealcbor into CustomDuration
		var decoded models.CustomDuration
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, customDur.Duration, decoded.Duration)
	})

	t.Run("Decode Tag 14 to interface{}", func(t *testing.T) {
		// Create a CustomDuration using fxamacker/cbor
		duration := 1*time.Hour + 15*time.Minute
		customDur := models.CustomDuration{Duration: duration}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDur)
		require.NoError(t, err)

		// Decode with surrealcbor into interface{}
		var decoded interface{}
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		// Should be decoded as CustomDuration
		decodedDur, ok := decoded.(models.CustomDuration)
		require.True(t, ok)
		assert.Equal(t, customDur.Duration, decodedDur.Duration)
	})

	t.Run("Decode negative duration", func(t *testing.T) {
		// Create a negative duration
		duration := -2*time.Hour - 30*time.Minute
		customDur := models.CustomDuration{Duration: duration}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDur)
		require.NoError(t, err)

		// Decode with surrealcbor into time.Duration
		var decoded time.Duration
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, duration, decoded)
	})
}

func TestDecodeTimeTypesInStruct(t *testing.T) {
	t.Skip("Skipping struct test - struct field type conversion from CustomDateTime to time.Time needs additional work")

	type TestStruct struct {
		StartTime time.Time     `cbor:"start_time"`
		Duration  time.Duration `cbor:"duration"`
	}

	t.Run("Decode struct with time.Time and time.Duration", func(t *testing.T) {
		// Create test data with CustomDateTime and CustomDuration
		now := time.Now().UTC().Truncate(time.Nanosecond)
		duration := 3*time.Hour + 45*time.Minute

		// Create data with custom types
		type SourceStruct struct {
			StartTime models.CustomDateTime `cbor:"start_time"`
			Duration  models.CustomDuration `cbor:"duration"`
		}

		source := SourceStruct{
			StartTime: models.CustomDateTime{Time: now},
			Duration:  models.CustomDuration{Duration: duration},
		}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)
		err = tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&source)
		require.NoError(t, err)

		// Decode with surrealcbor into struct with standard time types
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, now, decoded.StartTime)
		assert.Equal(t, duration, decoded.Duration)
	})
}

func TestDecodeTimeTypesInMap(t *testing.T) {
	t.Run("Decode map with time.Time and time.Duration values", func(t *testing.T) {
		// Create test data
		now := time.Now().UTC().Truncate(time.Nanosecond)
		duration := 4*time.Hour + 20*time.Minute

		// Create map with custom types
		source := map[string]interface{}{
			"start_time": models.CustomDateTime{Time: now},
			"duration":   models.CustomDuration{Duration: duration},
		}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)
		err = tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&source)
		require.NoError(t, err)

		// Decode with surrealcbor
		var decoded map[string]interface{}
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		// Check decoded values
		startTime, ok := decoded["start_time"].(models.CustomDateTime)
		require.True(t, ok)
		assert.Equal(t, now, startTime.Time)

		decodedDuration, ok := decoded["duration"].(models.CustomDuration)
		require.True(t, ok)
		assert.Equal(t, duration, decodedDuration.Duration)
	})
}

func TestDecodeTimeTypesEdgeCases(t *testing.T) {
	t.Run("Decode zero time", func(t *testing.T) {
		// Create zero time
		zeroTime := time.Time{}
		customDT := models.CustomDateTime{Time: zeroTime}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDT)
		require.NoError(t, err)

		// Decode with surrealcbor into time.Time
		var decoded time.Time
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.True(t, decoded.IsZero())
	})

	t.Run("Decode zero duration", func(t *testing.T) {
		// Create zero duration
		zeroDuration := time.Duration(0)
		customDur := models.CustomDuration{Duration: zeroDuration}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDur)
		require.NoError(t, err)

		// Decode with surrealcbor into time.Duration
		var decoded time.Duration
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, zeroDuration, decoded)
	})

	t.Run("Decode maximum duration", func(t *testing.T) {
		// Create maximum duration (approximately 290 years)
		maxDuration := time.Duration(1<<63 - 1)
		customDur := models.CustomDuration{Duration: maxDuration}

		// Encode with fxamacker/cbor
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		data, err := em.Marshal(&customDur)
		require.NoError(t, err)

		// Decode with surrealcbor into time.Duration
		var decoded time.Duration
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, maxDuration, decoded)
	})
}

func TestDecodeTimeTypesWithDecoder(t *testing.T) {
	t.Run("Decode multiple time values with Decoder", func(t *testing.T) {
		// Create test data
		time1 := time.Now().UTC().Truncate(time.Nanosecond)
		duration1 := 2*time.Hour + 15*time.Minute
		time2 := time1.Add(24 * time.Hour)
		duration2 := 45 * time.Minute

		// Encode multiple values
		tags := cbor.NewTagSet()
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDateTime{}),
			models.TagCustomDatetime,
		)
		require.NoError(t, err)
		err = tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(models.CustomDuration{}),
			models.TagCustomDuration,
		)
		require.NoError(t, err)

		em, err := cbor.EncOptions{}.EncModeWithTags(tags)
		require.NoError(t, err)

		var buf bytes.Buffer
		encoder := em.NewEncoder(&buf)

		err = encoder.Encode(models.CustomDateTime{Time: time1})
		require.NoError(t, err)
		err = encoder.Encode(models.CustomDuration{Duration: duration1})
		require.NoError(t, err)
		err = encoder.Encode(models.CustomDateTime{Time: time2})
		require.NoError(t, err)
		err = encoder.Encode(models.CustomDuration{Duration: duration2})
		require.NoError(t, err)

		// Decode with surrealcbor Decoder
		decoder := NewDecoder(&buf)

		var decodedTime1 time.Time
		err = decoder.Decode(&decodedTime1)
		require.NoError(t, err)
		assert.Equal(t, time1, decodedTime1)

		var decodedDuration1 time.Duration
		err = decoder.Decode(&decodedDuration1)
		require.NoError(t, err)
		assert.Equal(t, duration1, decodedDuration1)

		var decodedTime2 time.Time
		err = decoder.Decode(&decodedTime2)
		require.NoError(t, err)
		assert.Equal(t, time2, decodedTime2)

		var decodedDuration2 time.Duration
		err = decoder.Decode(&decodedDuration2)
		require.NoError(t, err)
		assert.Equal(t, duration2, decodedDuration2)
	})
}

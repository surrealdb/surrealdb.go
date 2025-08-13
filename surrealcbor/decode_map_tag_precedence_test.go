package surrealcbor

import (
	"reflect"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecode_mapStructFieldTagPrecedence(t *testing.T) {
	// Test that cbor tags take precedence over json tags

	t.Run("cbor tag takes precedence over json tag", func(t *testing.T) {
		type TestStruct struct {
			// Field with both cbor and json tags with different names
			Field1 string `cbor:"cbor_name" json:"json_name"`
			// Field with only json tag
			Field2 string `json:"field2"`
			// Field with only cbor tag
			Field3 string `cbor:"field3"`
			// Field with no tags (uses field name)
			Field4 string
		}

		// Create CBOR data using cbor field names
		data := map[string]string{
			"cbor_name": "value1", // Should match Field1 via cbor tag
			"field2":    "value2", // Should match Field2 via json tag
			"field3":    "value3", // Should match Field3 via cbor tag
			"Field4":    "value4", // Should match Field4 via field name
		}

		encoded, err := cbor.Marshal(data)
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "value1", decoded.Field1)
		assert.Equal(t, "value2", decoded.Field2)
		assert.Equal(t, "value3", decoded.Field3)
		assert.Equal(t, "value4", decoded.Field4)
	})

	t.Run("json tag is ignored when cbor tag exists", func(t *testing.T) {
		type TestStruct struct {
			Field1 string `cbor:"cbor_name" json:"json_name"`
		}

		// Try to use json tag name - should NOT match
		data := map[string]string{
			"json_name": "value_via_json",
		}

		encoded, err := cbor.Marshal(data)
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		// Field1 should be empty because json_name doesn't match cbor_name
		assert.Equal(t, "", decoded.Field1)
	})

	t.Run("tag options are handled correctly", func(t *testing.T) {
		type TestStruct struct {
			Field1 string `cbor:"field1,omitempty" json:"ignored"`
			Field2 string `json:"field2,omitempty"`
		}

		data := map[string]string{
			"field1": "value1",
			"field2": "value2",
		}

		encoded, err := cbor.Marshal(data)
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "value1", decoded.Field1)
		assert.Equal(t, "value2", decoded.Field2)
	})

	t.Run("time types work with json tags when no cbor tag", func(t *testing.T) {
		type TestStruct struct {
			// Only json tag, no cbor tag
			StartTime time.Time     `json:"start_time"`
			Duration  time.Duration `json:"duration"`
		}

		now := time.Now().UTC().Truncate(time.Nanosecond)
		duration := 2*time.Hour + 30*time.Minute

		// Create data with custom types
		type SourceStruct struct {
			StartTime models.CustomDateTime `json:"start_time"`
			Duration  models.CustomDuration `json:"duration"`
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

		// Decode with surrealcbor
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, now, decoded.StartTime)
		assert.Equal(t, duration, decoded.Duration)
	})

	t.Run("field name fallback works when no tags", func(t *testing.T) {
		type TestStruct struct {
			StartTime time.Time
			Duration  time.Duration
		}

		now := time.Now().UTC().Truncate(time.Nanosecond)
		duration := 1*time.Hour + 15*time.Minute

		// Create data using field names
		type SourceStruct struct {
			StartTime models.CustomDateTime
			Duration  models.CustomDuration
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

		// Decode with surrealcbor
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, now, decoded.StartTime)
		assert.Equal(t, duration, decoded.Duration)
	})
}

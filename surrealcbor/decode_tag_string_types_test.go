package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecodeStringUUIDTag(t *testing.T) {
	t.Run("decode UUIDString", func(t *testing.T) {
		// Create test data with Tag 9
		uuidStr := models.UUIDString("550e8400-e29b-41d4-a716-446655440000")
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagStringUUID,
			Content: string(uuidStr),
		})
		require.NoError(t, err)

		var result models.UUIDString
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, uuidStr, result)
	})

	t.Run("decode into interface", func(t *testing.T) {
		uuidStr := "550e8400-e29b-41d4-a716-446655440000"
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagStringUUID,
			Content: uuidStr,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, models.UUIDString(uuidStr), result)
	})
}

func TestDecodeStringDecimalTag(t *testing.T) {
	t.Run("decode DecimalString", func(t *testing.T) {
		decimalStr := models.DecimalString("123.456")
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagStringDecimal,
			Content: string(decimalStr),
		})
		require.NoError(t, err)

		var result models.DecimalString
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, decimalStr, result)
	})

	t.Run("decode into interface", func(t *testing.T) {
		decimalStr := "999.99"
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagStringDecimal,
			Content: decimalStr,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, models.DecimalString(decimalStr), result)
	})
}

func TestDecodeStringDurationTag(t *testing.T) {
	t.Run("decode CustomDurationString", func(t *testing.T) {
		durationStr := models.CustomDurationString("1h30m")
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagStringDuration,
			Content: string(durationStr),
		})
		require.NoError(t, err)

		var result models.CustomDurationString
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, durationStr, result)
	})

	t.Run("decode into interface", func(t *testing.T) {
		durationStr := "2d3h4m5s"
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagStringDuration,
			Content: durationStr,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDurationString(durationStr), result)
	})
}

func TestDecodeStringTypesInStruct(t *testing.T) {
	type TestStruct struct {
		UUID     models.UUIDString           `cbor:"uuid"`
		Decimal  models.DecimalString        `cbor:"decimal"`
		Duration models.CustomDurationString `cbor:"duration"`
	}

	t.Run("decode struct with string types", func(t *testing.T) {
		// Create test data
		original := map[string]any{
			"uuid": cbor.Tag{
				Number:  models.TagStringUUID,
				Content: "550e8400-e29b-41d4-a716-446655440000",
			},
			"decimal": cbor.Tag{
				Number:  models.TagStringDecimal,
				Content: "123.456",
			},
			"duration": cbor.Tag{
				Number:  models.TagStringDuration,
				Content: "1h30m",
			},
		}

		data, err := cbor.Marshal(original)
		require.NoError(t, err)

		var result TestStruct
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, models.UUIDString("550e8400-e29b-41d4-a716-446655440000"), result.UUID)
		assert.Equal(t, models.DecimalString("123.456"), result.Decimal)
		assert.Equal(t, models.CustomDurationString("1h30m"), result.Duration)
	})
}

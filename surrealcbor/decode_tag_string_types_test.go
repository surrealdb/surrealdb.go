package surrealcbor

import (
	"math"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
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

// TestDecode_compactFormsIntoTargets covers what each compact form decodes
// into. Since the SDK sends a large uint64 as tag 10, a UUIDString as tag 37
// and a CustomDurationString as tag 14, each must decode back into the Go type
// it came from, and a target that can't hold the value must give an error.
func TestDecode_compactFormsIntoTargets(t *testing.T) {
	type myString string
	const uuidText = "0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b"
	uuidBytes := [16]byte{0x01, 0x90, 0xb1, 0xa2, 0x7c, 0x3e, 0x7d, 0x1f, 0x8a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b}
	decimal := func(s string) cbor.Tag { return cbor.Tag{Number: models.TagStringDecimal, Content: s} }
	binaryUUID := cbor.Tag{Number: models.TagSpecBinaryUUID, Content: uuidBytes[:]}
	duration := func(parts ...uint64) cbor.Tag { return cbor.Tag{Number: models.TagCustomDuration, Content: parts} }

	cases := []struct {
		name    string
		in      any
		target  func() any // returns a pointer to decode into
		want    any        // what the pointer points to afterwards
		wantErr string
	}{
		{"tag 10 into uint64", decimal("18446744073709551615"), func() any { return new(uint64) }, uint64(math.MaxUint64), ""},
		{"tag 10 into uint8", decimal("255"), func() any { return new(uint8) }, uint8(255), ""},
		{"tag 10 into int64", decimal("-9223372036854775808"), func() any { return new(int64) }, int64(math.MinInt64), ""},
		{"tag 10 into int8", decimal("-42"), func() any { return new(int8) }, int8(-42), ""},
		{"tag 10 into float64", decimal("1.5"), func() any { return new(float64) }, 1.5, ""},
		{"tag 10 into float32", decimal("-0.25"), func() any { return new(float32) }, float32(-0.25), ""},
		{"tag 10 into DecimalString", decimal("1.5"), func() any { return new(models.DecimalString) }, models.DecimalString("1.5"), ""},
		{"tag 10 into a named string", decimal("1.5"), func() any { return new(myString) }, myString("1.5"), ""},
		{"tag 10 above int64", decimal("18446744073709551615"), func() any { return new(int64) }, nil, "cannot decode decimal"},
		{"tag 10 above int8", decimal("128"), func() any { return new(int8) }, nil, "cannot decode decimal"},
		{"tag 10 negative into uint64", decimal("-1"), func() any { return new(uint64) }, nil, "cannot decode decimal"},
		{"tag 10 fraction into int", decimal("1.5"), func() any { return new(int) }, nil, "cannot decode decimal"},
		{"tag 10 into bool", decimal("1"), func() any { return new(bool) }, nil, "cannot decode models.DecimalString into bool"},

		{"tag 37 into UUIDString", binaryUUID, func() any { return new(models.UUIDString) }, models.UUIDString(uuidText), ""},
		{"tag 37 into a named string", binaryUUID, func() any { return new(myString) }, myString(uuidText), ""},
		{"tag 37 into [16]byte", binaryUUID, func() any { return new([16]byte) }, uuidBytes, ""},
		{"tag 37 into gofrs UUID", binaryUUID, func() any { return new(uuid.UUID) }, uuid.UUID(uuidBytes), ""},
		{"tag 37 into models.UUID", binaryUUID, func() any { return new(models.UUID) }, models.UUID{UUID: uuidBytes}, ""},
		{"tag 37 into [8]byte", binaryUUID, func() any { return new([8]byte) }, nil, "cannot decode a UUID into [8]uint8"},
		{"tag 37 into int", binaryUUID, func() any { return new(int) }, nil, "cannot decode a UUID into int"},

		{"tag 14 into int64", duration(5400, 7), func() any { return new(int64) }, int64(5400_000_000_007), ""},
		{"tag 14 into CustomDurationString", duration(5400, 7), func() any { return new(models.CustomDurationString) }, models.CustomDurationString("1h30m7ns"), ""},
		{"tag 14 of 600y into CustomDurationString", duration(18921600000), func() any { return new(models.CustomDurationString) }, models.CustomDurationString("600y"), ""},
		{"tag 14 of u64 seconds into CustomDurationString", duration(math.MaxUint64, 999999999), func() any { return new(models.CustomDurationString) }, models.CustomDurationString("584942417355y3w5d7h15s999ms999µs999ns"), ""},
		{"empty tag 14 into CustomDurationString", duration(), func() any { return new(models.CustomDurationString) }, models.CustomDurationString(""), ""},
		{"tag 14 with a second of nanos", duration(1, 1e9), func() any { return new(models.CustomDurationString) }, nil, "not below a second"},
		{"tag 14 into bool", duration(1), func() any { return new(bool) }, nil, "cannot decode a duration into bool"},

		{"tag 9 into int", cbor.Tag{Number: models.TagStringUUID, Content: uuidText}, func() any { return new(int) }, nil, "cannot decode models.UUIDString into int"},
		{"tag 13 into int", cbor.Tag{Number: models.TagStringDuration, Content: "1h"}, func() any { return new(int) }, nil, "cannot decode models.CustomDurationString into int"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := cbor.Marshal(tc.in)
			require.NoError(t, err)
			target := tc.target()
			err = Unmarshal(data, target)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, reflect.ValueOf(target).Elem().Interface())
		})
	}
}

// TestDecode_uuidIntoArrayFieldKeepsTheRest checks that a UUID decoded into a
// 16-byte array field, such as a gofrs UUID, fills it, and that the rest of
// the record still decodes.
func TestDecode_uuidIntoArrayFieldKeepsTheRest(t *testing.T) {
	id := uuid.Must(uuid.FromString("0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b"))
	data, err := cbor.Marshal(map[string]any{
		"id":   cbor.Tag{Number: models.TagSpecBinaryUUID, Content: id.Bytes()},
		"name": "x",
	})
	require.NoError(t, err)
	var got struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	require.NoError(t, Unmarshal(data, &got))
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "x", got.Name)
}

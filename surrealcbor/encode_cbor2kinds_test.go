package surrealcbor

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

type dateRange = models.Range[models.PlainDate, models.BoundIncluded[models.PlainDate], models.BoundExcluded[models.PlainDate]]

// Each kind is sent as its cbor.2 tag over cbor.2, and widened as the server
// widens it over plain cbor, wherever it is in the value.
func TestMarshal_cbor2Kinds(t *testing.T) {
	date := models.MustParsePlainDate("2024-02-29")
	cases := []struct {
		name   string
		v      any
		v1, v2 string
	}{
		{"date", date, "cc821a65dfc90000", "d864194d46"},
		{"date pointer", &date, "cc821a65dfc90000", "d864194d46"},
		{"nil date pointer", (*models.PlainDate)(nil), "f6", "f6"},
		{"date infinity", models.PlainDateInfinity, "cc821b000007779a0a6b7f1a3b9ac9ff", "d8641a05a9fc3c"},
		{"time", models.MustParsePlainTime("12:34:56.789"), "ce8219b0f01a2f072f40", "d8bd1b000029327b048f40"},
		{"midnight", models.PlainTime{}, "ce80", "d8bd00"},
		{"timestamp", models.MustParseTimestamp("2024-01-01T12:00:00.000000001"), "cc821a6592a94001", "d8be821a6592a94001"},
		{"interval", models.Interval{Months: 14, Days: 3, Microseconds: 14706500000}, "70503159324d3344543448354d362e3553", "d8bf830e031b000000036c9361a0"},
		{"bigdecimal that fits a decimal", models.MustParseBigDecimal("1.50"), "ca63312e35", "d8c064312e3530"},
		{
			"bigdecimal past a decimal", models.MustParseBigDecimal("79228162514264337593543950336"),
			"781d3739323238313632353134323634333337353933353433393530333336",
			"d8c0781d3739323238313632353134323634333337353933353433393530333336",
		},
		{"in a map", map[string]any{"d": models.PlainDate{}}, "a16164cc820000", "a16164d86400"},
		{"beside a sized number", []any{int8(1), models.PlainDate{}}, "8201cc820000", "82d8b401d86400"},
		{
			"in a record id key", models.NewRecordID("t", []any{models.MustParsePlainDate("1970-01-02")}),
			"c882617481cc821a0001518000", "c882617481d86401",
		},
		{
			"in a range", dateRange{
				Begin: &models.BoundIncluded[models.PlainDate]{Value: models.MustParsePlainDate("1970-01-02")},
				End:   &models.BoundExcluded[models.PlainDate]{Value: models.MustParsePlainDate("1970-01-06")},
			},
			"d83182d832cc821a0001518000d833cc821a0006978000", "d83182d832d86401d833d86405",
		},
		{"regex", models.Regex{Pattern: "a.*"}, "", "d8c163612e2a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v2, err := NewV2().Marshal(tc.v)
			require.NoError(t, err)
			assert.Equal(t, tc.v2, hex.EncodeToString(v2), "cbor.2")

			v1, err := New().Marshal(tc.v)
			if tc.v1 == "" {
				require.ErrorIs(t, err, constants.ErrNeedsCBOR2, "plain cbor has no form for it")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.v1, hex.EncodeToString(v1), "plain cbor")
		})
	}
}

func TestMarshal_cbor2KindsRoundTrip(t *testing.T) {
	type row struct {
		Date     models.PlainDate   `cbor:"date"`
		Time     models.PlainTime   `cbor:"time"`
		At       models.Timestamp   `cbor:"at"`
		Every    models.Interval    `cbor:"every"`
		Price    models.BigDecimal  `cbor:"price"`
		Pattern  models.Regex       `cbor:"pattern"`
		Optional *models.PlainDate  `cbor:"optional,omitempty"`
		Many     []models.Timestamp `cbor:"many"`
	}
	in := row{
		Date:    models.PlainDateNegInfinity,
		Time:    models.MustParsePlainTime("24:00:00"),
		At:      models.MustParseTimestamp("1969-12-31T23:59:59.5"),
		Every:   models.Interval{Months: -1, Microseconds: -500000},
		Price:   models.MustParseBigDecimal("0.000"),
		Pattern: models.Regex{Pattern: `\d+/x`},
		Many:    []models.Timestamp{models.TimestampInfinity, {}},
	}
	data, err := NewV2().Marshal(in)
	require.NoError(t, err)
	var out row
	require.NoError(t, NewV2().Unmarshal(data, &out))
	assert.Equal(t, in, out)

	// Over plain cbor, all but the regex make the trip, each read back from
	// its widened form.
	in.Pattern = models.Regex{}
	_, err = New().Marshal(in)
	require.ErrorIs(t, err, constants.ErrNeedsCBOR2, "even the empty pattern is a regex")
}

// The deprecated fxamacker-based codec widens the kinds too.
func TestMarshal_cbor2KindsDeprecatedCodec(t *testing.T) {
	marshaler := &models.CborMarshaler{} //nolint:staticcheck // the deprecated encoder must widen the kinds
	data, err := marshaler.Marshal(map[string]any{"d": models.PlainDate{}})
	require.NoError(t, err)
	assert.Equal(t, "a16164cc820000", hex.EncodeToString(data))
	_, err = marshaler.Marshal(map[string]any{"r": models.Regex{Pattern: "a"}})
	require.ErrorIs(t, err, constants.ErrNeedsCBOR2)

	// And the package-level encoder, which writes plain cbor.
	data, err = Marshal(models.Interval{Days: 1})
	require.NoError(t, err)
	assert.Equal(t, "63503144", hex.EncodeToString(data), `"P1D"`)
}

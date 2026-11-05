package models

import (
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// cborUnmarshaler is the pointer side of a kind.
type cborUnmarshaler interface {
	UnmarshalCBOR([]byte) error
}

// The canonical cbor.2 vectors of spec/vectors/cbor2.yaml in surrealdb-protocol
// v0.15.0: each value writes these bytes and reads back from them.
func TestKinds_cbor2Vectors(t *testing.T) {
	for _, tc := range []struct {
		hex   string
		value cbor.Marshaler
		into  func() cborUnmarshaler
	}{
		{"d864194d46", MustParsePlainDate("2024-02-29"), func() cborUnmarshaler { return new(PlainDate) }},
		{"d86400", MustParsePlainDate("1970-01-01"), func() cborUnmarshaler { return new(PlainDate) }},
		{"d86420", MustParsePlainDate("1969-12-31"), func() cborUnmarshaler { return new(PlainDate) }},
		{"d8641a002cc0a1", MustParsePlainDate("+10000-01-01"), func() cborUnmarshaler { return new(PlainDate) }},
		{"d8641a05a9fc3c", PlainDateInfinity, func() cborUnmarshaler { return new(PlainDate) }},
		{"d8643a05bff18b", PlainDateNegInfinity, func() cborUnmarshaler { return new(PlainDate) }},
		{"d8bd00", MustParsePlainTime("00:00:00"), func() cborUnmarshaler { return new(PlainTime) }},
		{"d8bd1b000029327b048f40", MustParsePlainTime("12:34:56.789"), func() cborUnmarshaler { return new(PlainTime) }},
		{"d8bd1b00004e94914effff", MustParsePlainTime("23:59:59.999999999"), func() cborUnmarshaler { return new(PlainTime) }},
		{"d8bd1b00004e94914f0000", MustParsePlainTime("24:00:00"), func() cborUnmarshaler { return new(PlainTime) }},
		{"d8be820000", MustParseTimestamp("1970-01-01T00:00:00"), func() cborUnmarshaler { return new(Timestamp) }},
		{"d8be821a6592a94001", MustParseTimestamp("2024-01-01T12:00:00.000000001"), func() cborUnmarshaler { return new(Timestamp) }},
		{"d8be82201a1dcd6500", MustParseTimestamp("1969-12-31T23:59:59.5"), func() cborUnmarshaler { return new(Timestamp) }},
		{"d8be821b000007779a0a6b7f1a3b9ac9ff", TimestampInfinity, func() cborUnmarshaler { return new(Timestamp) }},
		{"d8be823b000007948cf211ff00", TimestampNegInfinity, func() cborUnmarshaler { return new(Timestamp) }},
		{"d8bf830e031b000000036c9361a0", Interval{14, 3, 14706500000}, func() cborUnmarshaler { return new(Interval) }},
		{"d8bf83000000", Interval{}, func() cborUnmarshaler { return new(Interval) }},
		{"d8bf83200000", Interval{Months: -1}, func() cborUnmarshaler { return new(Interval) }},
		{"d8bf8300003a0016e35f", Interval{Microseconds: -1500000}, func() cborUnmarshaler { return new(Interval) }},
		{"d8bf83000700", Interval{Days: 7}, func() cborUnmarshaler { return new(Interval) }},
		{
			"d8c0781d3739323238313632353134323634333337353933353433393530333336",
			MustParseBigDecimal("79228162514264337593543950336"), func() cborUnmarshaler { return new(BigDecimal) },
		},
		{
			"d8c0782037393232383136323531343236343333373539333534333935303333362e3530",
			MustParseBigDecimal("79228162514264337593543950336.50"), func() cborUnmarshaler { return new(BigDecimal) },
		},
		{"d8c06130", BigDecimal{}, func() cborUnmarshaler { return new(BigDecimal) }},
		{"d8c163612e2a", Regex{Pattern: "a.*"}, func() cborUnmarshaler { return new(Regex) }},
		{"d8c1655e5c642b24", Regex{Pattern: `^\d+$`}, func() cborUnmarshaler { return new(Regex) }},
		{"d8c160", Regex{}, func() cborUnmarshaler { return new(Regex) }},
	} {
		t.Run(tc.hex, func(t *testing.T) {
			b, err := tc.value.MarshalCBOR()
			require.NoError(t, err)
			assert.Equal(t, tc.hex, hex.EncodeToString(b))

			got := tc.into()
			require.NoError(t, got.UnmarshalCBOR(mustDecodeHex(t, tc.hex)))
			assert.Equal(t, tc.value, deref(got))
		})
	}
}

func deref(v cborUnmarshaler) any {
	switch p := v.(type) {
	case *PlainDate:
		return *p
	case *PlainTime:
		return *p
	case *Timestamp:
		return *p
	case *Interval:
		return *p
	case *BigDecimal:
		return *p
	case *Regex:
		return *p
	}
	return nil
}

// Each kind also reads the form plain cbor widens it to, from
// spec/vectors/cbor.yaml, so a field decodes the same in either dialect.
func TestKinds_decodeWidenedForms(t *testing.T) {
	var date PlainDate
	require.NoError(t, date.UnmarshalCBOR(mustDecodeHex(t, "cc821a65dfc90000")))
	assert.Equal(t, MustParsePlainDate("2024-02-29"), date)
	require.NoError(t, date.UnmarshalCBOR(mustDecodeHex(t, "cc823a0001517f00")))
	assert.Equal(t, MustParsePlainDate("1969-12-31"), date)
	// SurrealDB 4.0.0 widens infinity to its latest datetime.
	require.NoError(t, date.UnmarshalCBOR(mustDecodeHex(t, "cc821b000007779a0a6b7f1a3b9ac9ff")))
	assert.Equal(t, PlainDateInfinity, date)
	assert.ErrorContains(t, date.UnmarshalCBOR(mustDecodeHex(t, "cc821a65dfc90101")), "not at midnight UTC")

	var pt PlainTime
	for in, want := range map[string]string{
		"ce80":                 "00:00:00",
		"ce8119a8c0":           "12:00:00",
		"ce811a00015180":       "24:00:00",
		"ce8219b0f01a2f072f40": "12:34:56.789",
	} {
		require.NoError(t, pt.UnmarshalCBOR(mustDecodeHex(t, in)), in)
		assert.Equal(t, want, pt.String(), in)
	}
	assert.ErrorContains(t, pt.UnmarshalCBOR(mustDecodeHex(t, "ce811a00015181")), "longer than a day")

	var ts Timestamp
	require.NoError(t, ts.UnmarshalCBOR(mustDecodeHex(t, "cc821a6592a94001")))
	assert.Equal(t, MustParseTimestamp("2024-01-01T12:00:00.000000001"), ts)

	var interval Interval
	require.NoError(t, interval.UnmarshalCBOR(mustDecodeHex(t, "70503159324d3344543448354d362e3553")))
	assert.Equal(t, Interval{14, 3, 14706500000}, interval)
	require.NoError(t, interval.UnmarshalCBOR(mustDecodeHex(t, "6450543053")))
	assert.Equal(t, Interval{}, interval)

	var decimal BigDecimal
	require.NoError(t, decimal.UnmarshalCBOR(mustDecodeHex(t, "ca63312e35")))
	assert.Equal(t, MustParseBigDecimal("1.5"), decimal)
	require.NoError(t, decimal.UnmarshalCBOR(mustDecodeHex(t, "781d3739323238313632353134323634333337353933353433393530333336")))
	assert.Equal(t, MustParseBigDecimal("79228162514264337593543950336"), decimal)
	// An exponent is folded in, and the scale kept.
	require.NoError(t, decimal.UnmarshalCBOR(mustDecodeHex(t, "d8c067312e35452d3330")))
	assert.Equal(t, "0.0000000000000000000000000000015", decimal.String())
}

// Another cbor.2 kind decodes as plain cbor would carry it: a date into a
// Timestamp is its midnight, and a timestamp at midnight into a PlainDate is
// its date.
func TestKinds_decodeOtherKinds(t *testing.T) {
	var ts Timestamp
	require.NoError(t, ts.UnmarshalCBOR(mustDecodeHex(t, "d864194d46")))
	assert.Equal(t, MustParseTimestamp("2024-02-29T00:00:00"), ts)
	require.NoError(t, ts.UnmarshalCBOR(mustDecodeHex(t, "d8641a05a9fc3c")))
	assert.Equal(t, TimestampInfinity, ts, "the date infinity widens to the last instant")

	var date PlainDate
	midnight, err := MustParseTimestamp("2024-02-29T00:00:00").MarshalCBOR()
	require.NoError(t, err)
	require.NoError(t, date.UnmarshalCBOR(midnight))
	assert.Equal(t, MustParsePlainDate("2024-02-29"), date)
	noon, err := MustParseTimestamp("2024-02-29T12:00:00").MarshalCBOR()
	require.NoError(t, err)
	assert.Error(t, date.UnmarshalCBOR(noon))

	// A kind with no widened form the type reads is refused, never dropped.
	assert.ErrorContains(t, date.UnmarshalCBOR(mustDecodeHex(t, "d8c163612e2a")), "regex")
	assert.Error(t, date.UnmarshalCBOR(mustDecodeHex(t, "d8bd00")))
	var pt PlainTime
	assert.Error(t, pt.UnmarshalCBOR(mustDecodeHex(t, "d864194d46")))
	var interval Interval
	assert.Error(t, interval.UnmarshalCBOR(mustDecodeHex(t, "d8bd00")))
	var regex Regex
	assert.Error(t, regex.UnmarshalCBOR(mustDecodeHex(t, "63612e2a")), "plain cbor has no regex: text is not one")
}

// A datetime holds a leap second as chrono does, in the last second of a
// minute, and the server refuses one anywhere else. A timestamp read from
// one carries it into the next second.
func TestKinds_decodeLeapSecondDatetimes(t *testing.T) {
	var ts Timestamp
	require.NoError(t, ts.UnmarshalCBOR(mustDecodeHex(t, "cc82183b1a59682f00"))) // 12([59, 1500000000])
	assert.Equal(t, MustParseTimestamp("1970-01-01T00:01:00.5"), ts)
	assert.Error(t, ts.UnmarshalCBOR(mustDecodeHex(t, "cc82001a3b9aca00")), "12([0, 1000000000]): not the end of a minute")
	assert.Error(t, ts.UnmarshalCBOR(mustDecodeHex(t, "cc82183b1a77359400")), "12([59, 2000000000])")

	var date PlainDate
	assert.Error(t, date.UnmarshalCBOR(mustDecodeHex(t, "cc821a0001517f1a3b9aca00")), "12([86399, 1000000000]) is 23:59:60, not midnight")
}

func TestKinds_decodeLeapSeconds(t *testing.T) {
	// As on SurrealDB 4.0.0 (commit 4cd688c), nanoseconds up to the largest
	// uint32 carry into the seconds.
	var ts, want Timestamp
	require.NoError(t, ts.UnmarshalCBOR(mustDecodeHex(t, "d8be82183b1a59682f00")))   // 190([59, 1500000000])
	require.NoError(t, want.UnmarshalCBOR(mustDecodeHex(t, "d8be82183c1a1dcd6500"))) // 190([60, 500000000])
	assert.Equal(t, want, ts)
	require.NoError(t, ts.UnmarshalCBOR(mustDecodeHex(t, "d8be82001affffffff")))
	assert.Equal(t, MustParseTimestamp("1970-01-01T00:00:04.294967295"), ts)
	assert.Error(t, ts.UnmarshalCBOR(mustDecodeHex(t, "d8be82001b0000000100000000")))
	assert.Error(t, ts.UnmarshalCBOR(mustDecodeHex(t, "d8be821b000007779a0a6b7f1a3b9aca00")))
}

// The reject vectors of spec/vectors/cbor2.yaml in surrealdb-protocol v0.15.0
// that each kind's own decoder sees.
func TestKinds_rejectVectors(t *testing.T) {
	for name, tc := range map[string]struct {
		hex  string
		into cborUnmarshaler
	}{
		"reject-date-past-infinity":             {"d8641a05a9fc3d", new(PlainDate)},
		"reject-date-before-negative-infinity":  {"d8643a05bff18c", new(PlainDate)},
		"reject-date-past-i32":                  {"d8641a80000000", new(PlainDate)},
		"reject-date-float":                     {"d864f93c00", new(PlainDate)},
		"reject-time-past-end-of-day":           {"d8bd1b00004e94914f0001", new(PlainTime)},
		"reject-time-negative":                  {"d8bd20", new(PlainTime)},
		"reject-timestamp-past-infinity":        {"d8be821b000007779a0a6b8000", new(Timestamp)},
		"reject-timestamp-one-element":          {"d8be8100", new(Timestamp)},
		"reject-timestamp-negative-nanoseconds": {"d8be822020", new(Timestamp)},
		"reject-interval-months-past-i32":       {"d8bf831a800000000000", new(Interval)},
		"reject-interval-two-elements":          {"d8bf820000", new(Interval)},
		"reject-interval-float":                 {"d8bf830000f93c00", new(Interval)},
		"reject-bigdecimal-nan":                 {"d8c0634e614e", new(BigDecimal)},
		"reject-bigdecimal-comma":               {"d8c063312c35", new(BigDecimal)},
		"reject-bigdecimal-whitespace":          {"d8c06420312e35", new(BigDecimal)},
		"reject-bigdecimal-integer":             {"d8c001", new(BigDecimal)},
		"reject-regex-integer":                  {"d8c101", new(Regex)},
	} {
		assert.Error(t, tc.into.UnmarshalCBOR(mustDecodeHex(t, tc.hex)), name)
	}
}

// NULL and NONE leave a kind as it is, as fxamacker/cbor hands them to the
// Unmarshaler of a value field.
func TestKinds_absentValues(t *testing.T) {
	for _, in := range []string{"f6", "f7", "c6f6", "c6f7"} {
		date := MustParsePlainDate("2024-02-29")
		require.NoError(t, date.UnmarshalCBOR(mustDecodeHex(t, in)), in)
		assert.Equal(t, MustParsePlainDate("2024-02-29"), date, in)
	}

	var row struct {
		Date PlainDate  `cbor:"date"`
		Re   Regex      `cbor:"re"`
		Ptr  *Timestamp `cbor:"ptr"`
	}
	// {"date": null, "re": NONE, "ptr": null}
	require.NoError(t, cbor.Unmarshal(mustDecodeHex(t, "a36464617465f6627265c6f663707472f6"), &row))
	assert.Equal(t, PlainDate{}, row.Date)
	assert.Nil(t, row.Ptr)
}

// fxamacker/cbor, which the deprecated codec and the models types use, calls
// the kinds' methods wherever they are.
func TestKinds_fxamacker(t *testing.T) {
	type row struct {
		Date     PlainDate    `cbor:"date"`
		Time     *PlainTime   `cbor:"time,omitempty"`
		Decimals []BigDecimal `cbor:"decimals"`
		Any      any          `cbor:"any"`
	}
	in := row{
		Date:     MustParsePlainDate("2024-02-29"),
		Decimals: []BigDecimal{MustParseBigDecimal("1.50")},
		Any:      Interval{Days: 1},
	}
	data, err := cbor.Marshal(in)
	require.NoError(t, err)
	assert.Contains(t, hex.EncodeToString(data), "d8bf83000100", "a kind inside an any encodes itself")
	var out row
	require.NoError(t, cbor.Unmarshal(data, &out))
	assert.Equal(t, in.Date, out.Date)
	assert.Nil(t, out.Time, "a nil pointer is omitted")
	assert.Equal(t, in.Decimals, out.Decimals)

	// A range of dates writes the protocol's date-range vector.
	r := Range[PlainDate, BoundIncluded[PlainDate], BoundExcluded[PlainDate]]{
		Begin: &BoundIncluded[PlainDate]{Value: MustParsePlainDate("1970-01-02")},
		End:   &BoundExcluded[PlainDate]{Value: MustParsePlainDate("1970-01-06")},
	}
	data, err = r.MarshalCBOR()
	require.NoError(t, err)
	assert.Equal(t, "d83182d832d86401d833d86405", hex.EncodeToString(data))
	var back Range[PlainDate, BoundIncluded[PlainDate], BoundExcluded[PlainDate]]
	require.NoError(t, back.UnmarshalCBOR(data))
	assert.Equal(t, r, back)

	// A bound held as any gets the models type too, as a record id key does.
	var loose Range[any, BoundIncluded[any], BoundExcluded[any]]
	require.NoError(t, loose.UnmarshalCBOR(data))
	assert.Equal(t, MustParsePlainDate("1970-01-02"), loose.Begin.Value)
	assert.Equal(t, MustParsePlainDate("1970-01-06"), loose.End.Value)
}

// The zero values are real values: omitempty keeps them, and omitzero drops
// them, so an optional field is a pointer.
func TestKinds_zeroValues(t *testing.T) {
	type tagged struct {
		Empty PlainDate `cbor:"empty,omitempty"`
		Zero  PlainDate `cbor:"zero,omitzero"`
	}
	data, err := cbor.Marshal(tagged{})
	require.NoError(t, err)
	assert.Equal(t, "a165656d707479d86400", hex.EncodeToString(data))
}

func TestCustomDateTime_decodesDatesAndTimestamps(t *testing.T) {
	var dt CustomDateTime
	require.NoError(t, dt.UnmarshalCBOR(mustDecodeHex(t, "d864194d46")))
	assert.Equal(t, time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), dt.Time)
	require.NoError(t, dt.UnmarshalCBOR(mustDecodeHex(t, "d8be821a6592a94001")))
	assert.Equal(t, time.Date(2024, 1, 1, 12, 0, 0, 1, time.UTC), dt.Time)
	require.NoError(t, dt.UnmarshalCBOR(mustDecodeHex(t, "d8641a05a9fc3c")))
	assert.Equal(t, time.Unix(8210266876799, 999999999).UTC(), dt.Time, "infinity is the server's latest datetime")
	assert.ErrorContains(t, dt.UnmarshalCBOR(mustDecodeHex(t, "d8bd00")), "unexpected tag number: got 189, want 12")
}

func TestCustomDuration_decodesTimes(t *testing.T) {
	var d CustomDuration
	require.NoError(t, d.UnmarshalCBOR(mustDecodeHex(t, "d8bd1b000029327b048f40")))
	assert.Equal(t, 12*time.Hour+34*time.Minute+56*time.Second+789*time.Millisecond, d.Duration)
	assert.ErrorContains(t, d.UnmarshalCBOR(mustDecodeHex(t, "d864194d46")), "unexpected tag number: got 100")
}

func TestRecordID_kindKeys(t *testing.T) {
	// reject-date-record-id-key: 8(["t", 100(1)])
	var r RecordID
	assert.ErrorContains(t, r.UnmarshalCBOR(mustDecodeHex(t, "c8826174d86401")), "not a valid record id key")

	// Inside an array key, a date is fine: 8(["t", [100(1), 191([0, 1, 0])]])
	require.NoError(t, r.UnmarshalCBOR(mustDecodeHex(t, "c882617482d86401d8bf83000100")))
	assert.Equal(t, []any{MustParsePlainDate("1970-01-02"), Interval{Days: 1}}, r.ID)

	// And inside an object key: 8(["t", {"at": 190([0, 0])}])
	require.NoError(t, r.UnmarshalCBOR(mustDecodeHex(t, "c8826174a1626174d8be820000")))
	assert.Equal(t, map[any]any{"at": Timestamp{}}, r.ID)
}

func TestKinds_marshalExtremes(t *testing.T) {
	b, err := Interval{Months: math.MinInt32, Days: math.MaxInt32, Microseconds: math.MinInt64}.MarshalCBOR()
	require.NoError(t, err)
	var back Interval
	require.NoError(t, back.UnmarshalCBOR(b))
	assert.Equal(t, Interval{Months: math.MinInt32, Days: math.MaxInt32, Microseconds: math.MinInt64}, back)
}

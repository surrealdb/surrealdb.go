package cborwire

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
)

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := cbor.Marshal(v)
	require.NoError(t, err)
	return b
}

// The cbor.2 forms and the plain forms the server widens them to, from
// spec/vectors/cbor2.yaml and spec/vectors/cbor.yaml of surrealdb-protocol
// v0.15.0, where those vectors have both.
var widenedKinds = []struct {
	name, cbor2, plain string
}{
	{"date", "d864194d46", "cc821a65dfc90000"},
	{"date before 1970", "d86420", "cc823a0001517f00"},
	{"time", "d8bd1b000029327b048f40", "ce8219b0f01a2f072f40"},
	{"time at the end of the day", "d8bd1b00004e94914f0000", "ce811a00015180"},
	{"time at midnight", "d8bd00", "ce80"},
	{"timestamp", "d8be821a6592a94001", "cc821a6592a94001"},
	{"interval", "d8bf830e031b000000036c9361a0", "70503159324d3344543448354d362e3553"},
	{"zero interval", "d8bf83000000", "6450543053"},
	{"negative interval", "d8bf8300003a0016e35f", "6750542d312e3553"},
	{
		"bigdecimal past a decimal",
		"d8c0781d3739323238313632353134323634333337353933353433393530333336",
		"781d3739323238313632353134323634333337353933353433393530333336",
	},
	{"bigdecimal that fits a decimal", "d8c063312e35", "ca63312e35"},
	{"bigdecimal zero", "d8c06130", "ca6130"},
}

func TestNormalize_widensKindsForPlainCBOR(t *testing.T) {
	for _, tc := range widenedKinds {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Normalize(mustHex(t, tc.cbor2), Plain)
			require.NoError(t, err)
			assert.Equal(t, tc.plain, hex.EncodeToString(out))
		})
	}
}

func TestNormalize_widensAsServerV4(t *testing.T) {
	// SurrealDB 4.0.0 widens the date infinity to its latest datetime (commit
	// 4cd688c), where protocol v0.15.0's vector date-infinity-widens-to-datetime
	// still has midnight of that day (cc821b000007779a091a0000).
	out, err := Normalize(mustHex(t, "d8641a05a9fc3c"), Plain)
	require.NoError(t, err)
	assert.Equal(t, "cc821b000007779a0a6b7f1a3b9ac9ff", hex.EncodeToString(out))

	// -infinity is midnight of the first day either way.
	out, err = Normalize(mustHex(t, "d8643a05bff18b"), Plain)
	require.NoError(t, err)
	want := mustMarshal(t, cbor.Tag{Number: 12, Content: []any{MinTimestampSeconds, uint64(0)}})
	assert.Equal(t, want, out)

	// A bigdecimal with trailing zeros that fits a decimal loses them, as a
	// decimal does on the server; one that does not fit keeps its scale.
	out, err = Normalize(mustHex(t, "d8c064312e3530"), Plain)
	require.NoError(t, err)
	assert.Equal(t, "ca63312e35", hex.EncodeToString(out))
	scaled := "79228162514264337593543950336.50"
	out, err = Normalize(mustMarshal(t, cbor.Tag{Number: 192, Content: scaled}), Plain)
	require.NoError(t, err)
	assert.Equal(t, mustMarshal(t, scaled), out)

	// An exponent is folded in.
	out, err = Normalize(mustHex(t, "d8c06431653430"), Plain)
	require.NoError(t, err)
	assert.Equal(t, mustMarshal(t, "1"+strings.Repeat("0", 40)), out)
}

func TestNormalize_timestampCarriesLeapSeconds(t *testing.T) {
	for _, tc := range []struct {
		in, want [2]int64
	}{
		{[2]int64{59, 1500000000}, [2]int64{60, 500000000}},
		{[2]int64{0, math.MaxUint32}, [2]int64{4, 294967295}},
		{[2]int64{-1, 1000000000}, [2]int64{0, 0}},
	} {
		out, err := Normalize(mustMarshal(t, cbor.Tag{Number: 190, Content: tc.in}), Plain)
		require.NoError(t, err)
		assert.Equal(t, mustMarshal(t, cbor.Tag{Number: 12, Content: tc.want}), out)
	}
}

func TestNormalize_widensNestedKinds(t *testing.T) {
	// 49([50(100(1)), 51(100(5))]), the range 1970-01-02..1970-01-06.
	out, err := Normalize(mustHex(t, "d83182d832d86401d833d86405"), Plain)
	require.NoError(t, err)
	assert.Equal(t, "d83182d832cc821a0001518000d833cc821a0006978000", hex.EncodeToString(out))

	in := mustMarshal(t, map[string]any{
		"at":   cbor.Tag{Number: 100, Content: 1},
		"list": []any{"x", cbor.Tag{Number: 189, Content: 1_000_000_000}, cbor.Tag{Number: 0, Content: "1970-01-01T00:00:01Z"}},
	})
	out, err = Normalize(in, Plain)
	require.NoError(t, err)
	var got any
	require.NoError(t, cbor.Unmarshal(out, &got))
	assert.Equal(t, map[any]any{
		"at":   cbor.Tag{Number: 12, Content: []any{uint64(86400), uint64(0)}},
		"list": []any{"x", cbor.Tag{Number: 14, Content: []any{uint64(1)}}, cbor.Tag{Number: 12, Content: []any{uint64(1), uint64(0)}}},
	}, got)
}

func TestNormalize_cbor2LeavesKinds(t *testing.T) {
	for _, tc := range widenedKinds {
		in := mustHex(t, tc.cbor2)
		out, err := Normalize(in, CBOR2)
		require.NoError(t, err, tc.name)
		assert.Equal(t, in, out, tc.name)
	}
	// A regex, too.
	in := mustHex(t, "d8c163612e2a")
	out, err := Normalize(in, CBOR2)
	require.NoError(t, err)
	assert.Equal(t, in, out)

	// The other rewrites still apply around them, but not inside them.
	in = mustMarshal(t, []any{
		cbor.Tag{Number: 100, Content: 1},
		cbor.Tag{Number: 0, Content: "1970-01-01T00:00:01Z"},
		cbor.Tag{Number: 192, Content: "18446744073709551615"},
	})
	out, err = Normalize(in, CBOR2)
	require.NoError(t, err)
	var got []any
	require.NoError(t, cbor.Unmarshal(out, &got))
	assert.Equal(t, []any{
		cbor.Tag{Number: 100, Content: uint64(1)},
		cbor.Tag{Number: 12, Content: []any{uint64(1), uint64(0)}},
		cbor.Tag{Number: 192, Content: "18446744073709551615"},
	}, got)
}

func TestNormalize_regexNeedsCBOR2(t *testing.T) {
	_, err := Normalize(mustHex(t, "d8c163612e2a"), Plain)
	require.ErrorIs(t, err, constants.ErrNeedsCBOR2)
	assert.ErrorContains(t, err, "regex")

	// Inside a value too.
	_, err = Normalize(mustMarshal(t, map[string]any{"re": cbor.Tag{Number: 193, Content: "a"}}), Plain)
	require.ErrorIs(t, err, constants.ErrNeedsCBOR2)
}

func TestNormalize_refusesMalformedKinds(t *testing.T) {
	for name, in := range map[string]string{
		"date as a float":           "d864f93c00",
		"date past infinity":        "d8641a05a9fc3d",
		"date before -infinity":     "d8643a05bff18c",
		"date as text":              "d8646161",
		"negative time":             "d8bd20",
		"time past the end of day":  "d8bd1b00004e94914f0001",
		"timestamp of one element":  "d8be8100",
		"timestamp negative nanos":  "d8be822020",
		"timestamp nanos past u32":  "d8be82001b0000000100000000",
		"timestamp past infinity":   "d8be821b000007779a0a6b8000",
		"interval of two elements":  "d8bf820000",
		"interval with a float":     "d8bf830000f93c00",
		"interval months past i32":  "d8bf831a800000000000",
		"interval with a sized int": "d8bf83d8b80e0000",
		"bigdecimal as an integer":  "d8c001",
		"bigdecimal NaN":            "d8c0634e614e",
		"bigdecimal with a comma":   "d8c063312c35",
		"bigdecimal with a space":   "d8c06420312e35",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Normalize(mustHex(t, in), Plain)
			assert.Error(t, err)
		})
	}
}

func TestReaders(t *testing.T) {
	t.Run("IsAbsent", func(t *testing.T) {
		for _, in := range []string{"f6", "f7", "c6f6", "c6f7"} {
			assert.True(t, IsAbsent(mustHex(t, in)), in)
		}
		for _, in := range []string{"", "00", "c600", "c6", "f6f6", "d86400"} {
			assert.False(t, IsAbsent(mustHex(t, in)), in)
		}
	})

	t.Run("SplitTag", func(t *testing.T) {
		tag, content, tagged, err := SplitTag(mustHex(t, "d8bf83010203"))
		require.NoError(t, err)
		assert.True(t, tagged)
		assert.Equal(t, uint64(191), tag)
		assert.Equal(t, "83010203", hex.EncodeToString(content))

		_, _, tagged, err = SplitTag(mustHex(t, "6161"))
		require.NoError(t, err)
		assert.False(t, tagged)

		_, _, _, err = SplitTag(mustHex(t, "df"))
		assert.Error(t, err, "a tag has no indefinite length")
	})

	t.Run("ReadInts", func(t *testing.T) {
		ints, n, err := ReadInts(mustHex(t, "8301203b7fffffffffffffff"))
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, [3]int64{1, -1, math.MinInt64}, ints)

		for name, in := range map[string]string{
			"four elements":     "8401020304",
			"a float":           "81f93c00",
			"a tag":             "81c101",
			"past int64":        "811b8000000000000000",
			"indefinite":        "9f01ff",
			"not an array":      "01",
			"trailing data":     "810100",
			"truncated element": "82011a",
		} {
			_, _, err := ReadInts(mustHex(t, in))
			assert.Error(t, err, name)
		}
	})

	t.Run("ReadText", func(t *testing.T) {
		text, err := ReadText(mustHex(t, "63612e2a"))
		require.NoError(t, err)
		assert.Equal(t, "a.*", text)
		for name, in := range map[string]string{
			"indefinite":   "7f6161ff",
			"bytes":        "4161",
			"invalid utf8": "61ff",
			"trailing":     "616100",
		} {
			_, err := ReadText(mustHex(t, in))
			assert.Error(t, err, name)
		}
	})

	t.Run("ReadDuration", func(t *testing.T) {
		for in, want := range map[string]int64{
			"80":                 0,
			"8119a8c0":           43200 * 1e9,
			"82001a3b9aca00":     1e9, // nanoseconds carry into the seconds
			"8219b0f01a2f072f40": 45296789000000,
		} {
			got, err := ReadDuration(mustHex(t, in))
			require.NoError(t, err, in)
			assert.Equal(t, want, got, in)
		}
		for _, in := range []string{"8301020a", "8120", "8201c11a3b9aca00", "811b7fffffffffffffff"} {
			_, err := ReadDuration(mustHex(t, in))
			assert.Error(t, err, in)
		}
	})
}

func TestFormatAndParseInterval(t *testing.T) {
	for _, tc := range []struct {
		months, days int32
		micros       int64
		text         string
	}{
		{14, 3, 14706500000, "P1Y2M3DT4H5M6.5S"},
		{0, 0, 0, "PT0S"},
		{-1, 0, 0, "P-1M"},
		{0, 7, 0, "P7D"},
		{12, 0, 0, "P1Y"},
		{0, 0, -1500000, "PT-1.5S"},
		{0, 0, -500000, "PT-0.5S"},
		{0, 0, -90_000_000, "PT-1M-30S"},
		{-13, 0, -1, "P-1Y-1MT-0.000001S"},
		{0, 0, 1, "PT0.000001S"},
		{0, 0, 3_600_000_000, "PT1H"},
		{0, 0, math.MinInt64, "PT-2562047788H-54.775808S"},
		{0, 0, math.MaxInt64, "PT2562047788H54.775807S"},
		{math.MinInt32, math.MinInt32, 0, "P-178956970Y-8M-2147483648D"},
		{math.MaxInt32, math.MaxInt32, 0, "P178956970Y7M2147483647D"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			assert.Equal(t, tc.text, FormatInterval(tc.months, tc.days, tc.micros))
			months, days, micros, err := ParseInterval(tc.text)
			require.NoError(t, err)
			assert.Equal(t, []int64{int64(tc.months), int64(tc.days), tc.micros}, []int64{int64(months), int64(days), micros})
		})
	}

	// Each number carries its own sign, and the seconds may have fewer digits.
	months, days, micros, err := ParseInterval("P1Y-1MT-1H30M0.5S")
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 0, -3_600_000_000 + 1_800_000_000 + 500_000}, []int64{int64(months), int64(days), micros})

	for _, bad := range []string{
		"", "P", "PT", "T1H", "1Y", "P1", "P1Y2Y", "P1D1Y", "P1.5Y", "PT1.1234567S", "PT1.S", "PT.5S",
		"P2147483648M", "P-178956971Y", "PT9223372036854775808S", "PT2562047789H", "P1YT", "p1y", "P1Y ",
	} {
		_, _, _, err := ParseInterval(bad)
		assert.Error(t, err, bad)
	}
}

func TestCanonicalBigDecimal(t *testing.T) {
	for in, want := range map[string]string{
		"0":                                      "0",
		"000":                                    "0",
		"-0":                                     "0",
		"0e5":                                    "0",
		"-0.00":                                  "0.00",
		"1.5":                                    "1.5",
		"007.50":                                 "7.50",
		"+1":                                     "1",
		"-5":                                     "-5",
		".5":                                     "0.5",
		"5.":                                     "5",
		"1.50e1":                                 "15.0",
		"1e-3":                                   "0.001",
		"1.5e3":                                  "1500",
		"1.5E+3":                                 "1500",
		"1e000005":                               "100000",
		"1e40":                                   "1" + strings.Repeat("0", 40),
		"1.5E-30":                                "0." + strings.Repeat("0", 29) + "15",
		"79228162514264337593543950336.50":       "79228162514264337593543950336.50",
		"-12345678901234567890123456789012345.5": "-12345678901234567890123456789012345.5",
	} {
		got, err := CanonicalBigDecimal(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, bad := range []string{
		"", " 1.5", "1.5 ", "1,5", "1_000", "NaN", "nan", "inf", "Infinity", "-infinity",
		".", "e5", "1e", "1e+", "1.2.3", "++1", "0x10", "1e147456", "1e" + strings.Repeat("1", 19),
	} {
		_, err := CanonicalBigDecimal(bad)
		assert.Error(t, err, bad)
	}
}

func TestCanonicalBigDecimal_limits(t *testing.T) {
	// The server counts digits without the leading zeros, but with the trailing ones.
	_, err := CanonicalBigDecimal(strings.Repeat("9", 147455))
	require.NoError(t, err)
	_, err = CanonicalBigDecimal("0000" + strings.Repeat("9", 147455))
	require.NoError(t, err, "leading zeros do not count")
	_, err = CanonicalBigDecimal(strings.Repeat("9", 147456))
	require.Error(t, err)

	// A folded exponent counts as digits too.
	_, err = CanonicalBigDecimal("1e147454")
	require.NoError(t, err)
	_, err = CanonicalBigDecimal("1e147455")
	require.Error(t, err)
	_, err = CanonicalBigDecimal("10e147454")
	require.Error(t, err)

	// The scale may reach 294910, through the digits after the point and a
	// negative exponent together.
	got, err := CanonicalBigDecimal("0." + strings.Repeat("0", 147454) + "1")
	require.NoError(t, err)
	assert.Len(t, got, 2+147455)
	_, err = CanonicalBigDecimal("0." + strings.Repeat("0", 147454) + "1e-147455")
	require.NoError(t, err)
	_, err = CanonicalBigDecimal("0." + strings.Repeat("0", 147455) + "1e-147455")
	require.Error(t, err)
}

func TestBigDecimalAsDecimal(t *testing.T) {
	for in, want := range map[string]string{
		"0":                               "0",
		"0.00":                            "0",
		"1.5":                             "1.5",
		"1.50":                            "1.5",
		"-1.50":                           "-1.5",
		"1500":                            "1500",
		"10000000000000000000000000000":   "10000000000000000000000000000", // 1e28
		"79228162514264337593543950335":   "79228162514264337593543950335", // 2^96 - 1
		"-79228162514264337593543950335":  "-79228162514264337593543950335",
		"7.9228162514264337593543950335":  "7.9228162514264337593543950335",
		"0.1111111111111111111111111111":  "0.1111111111111111111111111111", // 28 digits after the point
		"0.10000000000000000000000000000": "0.1",
	} {
		got, ok := BigDecimalAsDecimal(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{
		"90000000000000000000000000000",  // 9e28
		"100000000000000000000000000000", // 1e29
		"79228162514264337593543950336",  // 2^96
		"-79228162514264337593543950336",
		"0.11111111111111111111111111111", // 29 digits after the point
		"79228162514264337593543950336.50",
	} {
		_, ok := BigDecimalAsDecimal(in)
		assert.False(t, ok, in)
	}
}

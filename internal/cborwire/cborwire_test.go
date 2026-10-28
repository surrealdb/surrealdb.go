package cborwire

import (
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want any
	}{
		{
			"a datetime as text becomes tag 12",
			cbor.Tag{Number: 0, Content: "2026-01-02T03:04:05.000000006Z"},
			cbor.Tag{Number: 12, Content: []any{uint64(1767323045), uint64(6)}},
		},
		{
			"a datetime before 1970 keeps its sign",
			cbor.Tag{Number: 0, Content: "1969-12-31T23:59:59.5Z"},
			cbor.Tag{Number: 12, Content: []any{int64(-1), uint64(500000000)}},
		},
		{
			"an offset is applied",
			cbor.Tag{Number: 0, Content: "2026-01-02T04:04:05+01:00"},
			cbor.Tag{Number: 12, Content: []any{uint64(1767323045), uint64(0)}},
		},
		{
			"a UUID as text becomes tag 37",
			cbor.Tag{Number: 9, Content: "0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b"},
			cbor.Tag{Number: 37, Content: mustHex(t, "0190b1a27c3e7d1f8a2b3c4d5e6f7a8b")},
		},
		{
			"a duration as text becomes tag 14",
			cbor.Tag{Number: 13, Content: "1h30m5s7ns"},
			cbor.Tag{Number: 14, Content: []any{uint64(5405), uint64(7)}},
		},
		{
			"an integer above i64 becomes a decimal",
			uint64(math.MaxUint64),
			cbor.Tag{Number: 10, Content: "18446744073709551615"},
		},
		{
			"the largest i64 stays an integer",
			uint64(math.MaxInt64),
			uint64(math.MaxInt64),
		},
		{
			"nested values are rewritten, others kept",
			map[string]any{
				"at":   cbor.Tag{Number: 0, Content: "2026-01-02T03:04:05Z"},
				"list": []any{"x", cbor.Tag{Number: 13, Content: "2d"}, cbor.Tag{Number: 6, Content: nil}},
				"n":    int64(-5),
			},
			map[any]any{
				"at":   cbor.Tag{Number: 12, Content: []any{uint64(1767323045), uint64(0)}},
				"list": []any{"x", cbor.Tag{Number: 14, Content: []any{uint64(172800), uint64(0)}}, cbor.Tag{Number: 6, Content: nil}},
				"n":    int64(-5),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := cbor.Marshal(tc.in)
			require.NoError(t, err)
			out, err := Normalize(in)
			require.NoError(t, err)

			var got any
			require.NoError(t, cbor.Unmarshal(out, &got))
			want, err := cbor.Marshal(tc.want)
			require.NoError(t, err)
			var wantDecoded any
			require.NoError(t, cbor.Unmarshal(want, &wantDecoded))
			assert.Equal(t, wantDecoded, got)
		})
	}
}

func TestNormalize_negativeBelowI64(t *testing.T) {
	// -2^64, the most negative integer CBOR can hold.
	out, err := Normalize(mustHex(t, "3bffffffffffffffff"))
	require.NoError(t, err)
	var got cbor.Tag
	require.NoError(t, cbor.Unmarshal(out, &got))
	assert.Equal(t, cbor.Tag{Number: 10, Content: "-18446744073709551616"}, got)

	// math.MinInt64 is still an integer.
	in := mustHex(t, "3b7fffffffffffffff")
	out, err = Normalize(in)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestNormalize_leavesOtherBytesAlone(t *testing.T) {
	in, err := cbor.Marshal(map[string]any{
		"s": "text", "b": []byte{1, 2}, "f": 1.5, "t": cbor.Tag{Number: 12, Content: []int64{1, 2}},
	})
	require.NoError(t, err)
	out, err := Normalize(in)
	require.NoError(t, err)
	assert.Same(t, &in[0], &out[0], "nothing to rewrite returns the input")
}

func TestNormalize_indefiniteLength(t *testing.T) {
	// [_ "a" "b" tag0("1970-01-01T00:00:01Z") ] with an indefinite text "a".
	in := mustHex(t, "9f7f6161ff6162c074313937302d30312d30315430303a30303a30315aff")
	out, err := Normalize(in)
	require.NoError(t, err)
	var got []any
	require.NoError(t, cbor.Unmarshal(out, &got))
	require.Len(t, got, 3)
	assert.Equal(t, "a", got[0])
	assert.Equal(t, cbor.Tag{Number: 12, Content: []any{uint64(1), uint64(0)}}, got[2])
}

func TestNormalize_stream(t *testing.T) {
	in := append(mustHex(t, "01"), mustHex(t, "c974")...)
	in = append(in, []byte("not-a-uuid-at-all-xx")...)
	_, err := Normalize(in)
	assert.ErrorContains(t, err, "invalid UUID")
}

func TestNormalize_invalid(t *testing.T) {
	for name, in := range map[string]string{
		"truncated":       "a1",
		"bad datetime":    "c0636e6f77",
		"text tag on int": "c001",
		"stray break":     "ff",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Normalize(mustHex(t, in))
			assert.Error(t, err)
		})
	}
}

func TestNormalize_sizedNumbersAreLeft(t *testing.T) {
	// 186(18446744073709551615), a u64 in cbor.2, must not become 186(10(...)).
	in := mustHex(t, "d8ba1bffffffffffffffff")
	out, err := Normalize(in)
	require.NoError(t, err)
	assert.Equal(t, in, out)

	// Nor is a text tag inside one rewritten, but one after it is.
	in = mustHex(t, "82d8bcc0636e6f77c96130")
	_, err = Normalize(in)
	require.ErrorContains(t, err, "invalid UUID", "the walk goes on after the sized number")
	in = mustHex(t, "81d8bcc0636e6f77")
	out, err = Normalize(in)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestNormalize_datetimeOutsideFourDigitYears(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 12, 31, 23, 59, 59, 999999999, time.UTC),
		time.Date(12000, 2, 29, 12, 0, 0, 5, time.FixedZone("x", -3600)),
		time.Date(-4, 2, 29, 0, 0, 0, 0, time.UTC),
		time.Date(10001, 3, 1, 0, 0, 0, 0, time.UTC),
	} {
		t.Run(at.Format(time.RFC3339Nano), func(t *testing.T) {
			text := at.Format(time.RFC3339Nano)
			in, err := cbor.Marshal(cbor.Tag{Number: 0, Content: text})
			require.NoError(t, err)
			out, err := Normalize(in)
			require.NoError(t, err)
			var got cbor.Tag
			require.NoError(t, cbor.Unmarshal(out, &got))
			nanos := uint64(at.Nanosecond()) //nolint:gosec // 0 to 999999999
			assert.Equal(t, cbor.Tag{Number: 12, Content: []any{toCBORInt(at.Unix()), nanos}}, got)
		})
	}

	_, err := Normalize(mustHex(t, "c0"+hex.EncodeToString(append([]byte{0x74}, "10000-13-01T00:00:00Z"[:20]...))))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "tag 0", "a caller who passed a time.Time never saw tag 0")
}

func toCBORInt(v int64) any {
	if v < 0 {
		return v
	}
	return uint64(v)
}

func TestNormalize_longDurations(t *testing.T) {
	const year = 365 * 24 * 3600
	for text, want := range map[string][]any{
		"300y":                             {uint64(300 * year), uint64(0)},
		"600y":                             {uint64(600 * year), uint64(0)},
		"1000y":                            {uint64(1000 * year), uint64(0)},
		"584942417355y":                    {uint64(584942417355 * year), uint64(0)},
		"18446744073709551615s":            {uint64(math.MaxUint64), uint64(0)},
		"18446744073709551614s999999999ns": {uint64(math.MaxUint64 - 1), uint64(999999999)},
		"18446744073709551614s1000ms":      {uint64(math.MaxUint64), uint64(0)},
	} {
		t.Run(text, func(t *testing.T) {
			in, err := cbor.Marshal(cbor.Tag{Number: 13, Content: text})
			require.NoError(t, err)
			out, err := Normalize(in)
			require.NoError(t, err)
			var got cbor.Tag
			require.NoError(t, cbor.Unmarshal(out, &got))
			assert.Equal(t, cbor.Tag{Number: 14, Content: want}, got)
		})
	}

	for _, text := range []string{"584942417356y", "18446744073709551615s1000ms", "18446744073709551616ns"} {
		t.Run(text, func(t *testing.T) {
			in, err := cbor.Marshal(cbor.Tag{Number: 13, Content: text})
			require.NoError(t, err)
			_, err = Normalize(in)
			assert.ErrorContains(t, err, "too long")
		})
	}
}

func TestParseDuration(t *testing.T) {
	seconds, nanos, err := ParseDuration("1y2w3d4h5m6s7ms8µs9ns")
	require.NoError(t, err)
	want := 365*24*time.Hour + 2*7*24*time.Hour + 3*24*time.Hour + 4*time.Hour + 5*time.Minute +
		6*time.Second + 7*time.Millisecond + 8*time.Microsecond + 9
	assert.Equal(t, uint64(want/time.Second), seconds)
	assert.Equal(t, uint32(want%time.Second), nanos)

	// Sub-second units carry into the seconds.
	seconds, nanos, err = ParseDuration([]byte("1500ms600ms2500000000ns3us"))
	require.NoError(t, err)
	assert.Equal(t, uint64(4), seconds)
	assert.Equal(t, uint32(600003000), nanos)

	// Past 292 years, which an int64 of nanoseconds can't hold.
	seconds, nanos, err = ParseDuration("600y")
	require.NoError(t, err)
	assert.Equal(t, uint64(600*365*24*3600), seconds)
	assert.Zero(t, nanos)

	for _, bad := range []string{"3fortnights", "-5m", "5m 3s", "h", "5", "1.5s"} {
		_, _, err = ParseDuration(bad)
		assert.Error(t, err, bad)
	}
	_, _, err = ParseDuration("584942417356y")
	assert.ErrorContains(t, err, "too long")
	_, _, err = ParseDuration("99999999999999999999ns")
	assert.ErrorContains(t, err, "too long", "a value past a uint64")

	seconds, nanos, err = ParseDuration("")
	require.NoError(t, err)
	assert.Zero(t, seconds)
	assert.Zero(t, nanos)
}

func TestFormatDuration(t *testing.T) {
	type parts struct {
		seconds uint64
		nanos   uint32
	}
	for want, in := range map[string]parts{
		"":                                      {0, 0},
		"1h30m":                                 {5400, 0},
		"8µs":                                   {0, 8000},
		"600y":                                  {600 * 365 * 24 * 3600, 0},
		"1y2w3d4h5m6s7ms8µs9ns":                 {365*24*3600 + 2*7*24*3600 + 3*24*3600 + 4*3600 + 5*60 + 6, 7008009},
		"584942417355y3w5d7h15s999ms999µs999ns": {math.MaxUint64, 999999999},
	} {
		got := FormatDuration(in.seconds, in.nanos)
		assert.Equal(t, want, got)
		seconds, nanos, err := ParseDuration(got)
		require.NoError(t, err)
		assert.Equal(t, in, parts{seconds, nanos}, "%q parses back", got)
	}
}

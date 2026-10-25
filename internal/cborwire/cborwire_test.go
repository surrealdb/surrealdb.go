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
	assert.ErrorContains(t, err, "tag 9")
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

func TestParseDuration(t *testing.T) {
	ns, err := ParseDuration("1y2w3d4h5m6s7ms8µs9ns")
	require.NoError(t, err)
	want := 365*24*time.Hour + 2*7*24*time.Hour + 3*24*time.Hour + 4*time.Hour + 5*time.Minute +
		6*time.Second + 7*time.Millisecond + 8*time.Microsecond + 9
	assert.Equal(t, int64(want), ns)

	for _, bad := range []string{"3fortnights", "-5m", "5m 3s", "h"} {
		_, err = ParseDuration(bad)
		assert.Error(t, err, bad)
	}
	ns, err = ParseDuration("")
	require.NoError(t, err)
	assert.Zero(t, ns)
}

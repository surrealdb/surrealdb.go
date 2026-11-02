package surrealcbor

import (
	"math"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestMarshal_sendsOnlyFormsSurrealDBAccepts checks that both encoders send a
// time.Time, a UUIDString, a CustomDurationString and a uint64 above
// math.MaxInt64 in forms every SurrealDB release accepts. They used to send
// tags 0, 9 and 13 and an integer outside the range of an i64, which
// SurrealDB 4.0 refuses.
func TestMarshal_sendsOnlyFormsSurrealDBAccepts(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 6, time.FixedZone("x", 3600))
	cases := []struct {
		name string
		in   any
		want cbor.Tag
	}{
		{"time.Time", at, cbor.Tag{Number: 12, Content: []any{uint64(1767319445), uint64(6)}}},
		{"UUIDString", models.UUIDString("0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b"), cbor.Tag{
			Number:  37,
			Content: []byte{0x01, 0x90, 0xb1, 0xa2, 0x7c, 0x3e, 0x7d, 0x1f, 0x8a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b},
		}},
		{"CustomDurationString", models.CustomDurationString("1m2s3ns"), cbor.Tag{Number: 14, Content: []any{uint64(62), uint64(3)}}},
		{"uint64", uint64(math.MaxUint64), cbor.Tag{Number: 10, Content: "18446744073709551615"}},
	}
	encoders := map[string]func(any) ([]byte, error){
		"surrealcbor": New().Marshal,
		"fxamacker":   (&models.CborMarshaler{}).Marshal, //nolint:staticcheck // the deprecated encoder must send the same forms
	}
	for encName, marshal := range encoders {
		for _, tc := range cases {
			t.Run(encName+"/"+tc.name, func(t *testing.T) {
				b, err := marshal(map[string]any{"v": tc.in})
				require.NoError(t, err)
				var got map[string]cbor.Tag
				require.NoError(t, cbor.Unmarshal(b, &got))
				assert.Equal(t, tc.want, got["v"])
			})
		}
	}
}

func TestMarshal_timeTimeRoundTripsToTheNanosecond(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	b, err := Marshal(struct {
		At time.Time `json:"at"`
	}{at})
	require.NoError(t, err)

	var got struct {
		At time.Time `json:"at"`
	}
	require.NoError(t, Unmarshal(b, &got))
	assert.True(t, at.Equal(got.At), "%v != %v", at, got.At)
}

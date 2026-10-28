package surrealcbor

import (
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestCodec_versions(t *testing.T) {
	assert.Equal(t, 1, New().CBORVersion())
	assert.Equal(t, 2, NewV2().CBORVersion())
	assert.Equal(t, 1, (&Codec{}).CBORVersion())

	v1 := New()
	v2 := v1.ForCBORVersion(2).(*Codec)
	assert.Equal(t, 2, v2.CBORVersion())
	assert.Same(t, v1, v1.ForCBORVersion(1))
	assert.Equal(t, 1, v2.ForCBORVersion(1).(*Codec).CBORVersion())
	assert.Nil(t, v1.ForCBORVersion(3))
}

func TestMarshalV2_sizedKinds(t *testing.T) {
	f16 := models.MustFloat16(1.5)
	cases := []struct {
		name string
		v    any
		v1   string
		v2   string
	}{
		{"i8 min", int8(-128), "387f", "d8b4387f"},
		{"i8 max", int8(127), "187f", "d8b4187f"},
		{"u8", uint8(255), "18ff", "d8b518ff"},
		{"i16", int16(-32768), "397fff", "d8b6397fff"},
		{"u16", uint16(65535), "19ffff", "d8b719ffff"},
		{"i32", int32(-2147483648), "3a7fffffff", "d8b83a7fffffff"},
		{"u32", uint32(4294967295), "1affffffff", "d8b91affffffff"},
		{"u64 past int64", uint64(math.MaxUint64), "1bffffffffffffffff", "d8ba1bffffffffffffffff"},
		{"uint past int64", uint(math.MaxUint64), "1bffffffffffffffff", "d8ba1bffffffffffffffff"},
		{"u64 within int64 stays an int", uint64(5), "05", "05"},
		{"int stays an int", 5, "05", "05"},
		{"int64 stays an int", int64(-1), "20", "20"},
		{"f32", float32(1.5), "fa3fc00000", "d8bcfa3fc00000"},
		{"f32 NaN", float32(math.NaN()), "f97e00", "d8bcf97e00"},
		{"f16", f16, "f93e00", "d8bbf93e00"},
		{"float64 stays a float", 1.5, "fb3ff8000000000000", "fb3ff8000000000000"},
		{"pointer", &f16, "f93e00", "d8bbf93e00"},
		{"named kind", namedU8(7), "07", "d8b507"},
		{"bytes stay bytes", []byte{1, 2}, "420102", "420102"},
		{"array", []any{int8(1), "a", uint16(2)}, "83016161" + "02", "83d8b4016161d8b702"},
		{"set", models.Set{int8(1)}, "d83881" + "01", "d83881d8b401"},
		{"map", map[string]any{"a": int8(1)}, "a1616101", "a16161d8b401"},
		{"record id key", models.RecordID{Table: "t", ID: []any{uint8(5), float32(1.5)}}, "c882617482" + "05fa3fc00000", "c882617482d8b505d8bcfa3fc00000"},
		{"record id sized key", models.RecordID{Table: "t", ID: uint8(5)}, "c882617405", "c8826174d8b505"},
		{"datetime is untouched", models.CustomDateTime{Time: time.Unix(1, 0).UTC()}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b1, err := New().Marshal(tc.v)
			require.NoError(t, err)
			b2, err := NewV2().Marshal(tc.v)
			require.NoError(t, err)
			if tc.v1 == "" {
				assert.Equal(t, hex.EncodeToString(b1), hex.EncodeToString(b2))
				return
			}
			assert.Equal(t, tc.v1, hex.EncodeToString(b1), "plain cbor")
			assert.Equal(t, tc.v2, hex.EncodeToString(b2), "cbor.2")
		})
	}
}

type namedU8 uint8

type selfEncoding uint8

func (s selfEncoding) MarshalCBOR() ([]byte, error) { return []byte{byte(s)}, nil }

type sizedEmbedded struct {
	E int16 `json:"e"`
}

type sizedStruct struct {
	sizedEmbedded
	A    int8           `cbor:"a"`
	B    uint32         `json:"b,omitempty"`
	C    float32        `json:"-"`
	D    *int32         `json:"d"`
	S    selfEncoding   `json:"s"`
	Nest []sizedInner   `json:"nest"`
	Any  any            `json:"any"`
	Map  map[string]any `json:"map,omitempty"`
}

type sizedInner struct {
	X uint16
}

func TestMarshalV2_struct(t *testing.T) {
	d := int32(-2)
	v := sizedStruct{
		sizedEmbedded: sizedEmbedded{E: 3},
		A:             1,
		C:             9,
		D:             &d,
		S:             5,
		Nest:          []sizedInner{{X: 4}},
		Any:           uint8(6),
	}
	b, err := NewV2().Marshal(v)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, NewV2().Unmarshal(b, &got))
	assert.Equal(t, map[string]any{
		"e":    int16(3),
		"a":    int8(1),
		"d":    int32(-2),
		"s":    uint64(5), // encodes itself, so it is left untagged
		"nest": []any{map[string]any{"X": uint16(4)}},
		"any":  uint8(6),
	}, got)

	var back sizedStruct
	require.NoError(t, NewV2().Unmarshal(b, &back))
	assert.Equal(t, int8(1), back.A)
	assert.Equal(t, int32(-2), *back.D)
	assert.Equal(t, uint16(4), back.Nest[0].X)
	assert.Equal(t, uint8(6), back.Any)
}

func TestMarshalV2_nothingToTagReturnsSameBytes(t *testing.T) {
	// No maps here: fxamacker/cbor writes map entries in random order.
	v := []any{1, "x", []any{1.5, true, map[string]any{"a": int64(-1)}}}
	b1, err := New().Marshal(v)
	require.NoError(t, err)
	b2, err := NewV2().Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, b1, b2)
}

func TestUnmarshalV2_intoInterface(t *testing.T) {
	// [180(-1), 181(2), 182(3), 183(4), 184(5), 185(6), 186(7), 187(1.5), 188(2.5), 8, 1.5]
	data := mustDecodeHex(t, "8b"+"d8b420"+"d8b502"+"d8b603"+"d8b704"+"d8b805"+"d8b906"+"d8ba07"+"d8bbf93e00"+"d8bcf94100"+"08"+"f93e00")
	var got []any
	require.NoError(t, NewV2().Unmarshal(data, &got))
	assert.Equal(t, []any{
		int8(-1), uint8(2), int16(3), uint16(4), int32(5), uint32(6), uint64(7),
		models.MustFloat16(1.5), float32(2.5),
		uint64(8), // a plain integer decodes as it always has
		1.5,       // a plain float is float64 under cbor.2, whatever its width
	}, got)

	// Plain cbor keeps float32 for narrow floats, and refuses the sized tags.
	var plain any
	require.NoError(t, New().Unmarshal(mustDecodeHex(t, "f93e00"), &plain))
	assert.Equal(t, float32(1.5), plain)
	for _, h := range []string{"d8b401", "d8ba05", "d8bbf93e00", "d8bcf94100"} {
		assert.Error(t, New().Unmarshal(mustDecodeHex(t, h), &plain), h)
	}
}

func TestUnmarshalV2_typedDestinations(t *testing.T) {
	c := NewV2()

	var i8 int8
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "d8b4387f"), &i8))
	assert.Equal(t, int8(-128), i8)

	var wide int64
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "d8b518ff"), &wide))
	assert.Equal(t, int64(255), wide)

	var f float64
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "d8bbf93e00"), &f))
	assert.Equal(t, 1.5, f)

	var h models.Float16
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "d8bbf93e00"), &h))
	assert.Equal(t, models.MustFloat16(1.5), h)

	var f32 float32
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "d8bcfa3dcccccd"), &f32))
	assert.Equal(t, float32(0.1), f32)

	var u64 uint64
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "d8ba1bffffffffffffffff"), &u64))
	assert.Equal(t, uint64(math.MaxUint64), u64)

	// The value must fit the destination...
	var small int8
	assert.ErrorContains(t, c.Unmarshal(mustDecodeHex(t, "d8b518ff"), &small), "overflows")
	var signed int64
	assert.ErrorContains(t, c.Unmarshal(mustDecodeHex(t, "d8ba1bffffffffffffffff"), &signed), "overflows")
	var unsigned uint32
	assert.ErrorContains(t, c.Unmarshal(mustDecodeHex(t, "d8b420"), &unsigned), "overflows")
	var s string
	assert.Error(t, c.Unmarshal(mustDecodeHex(t, "d8b401"), &s))

	// ...and the tag, whatever the destination.
	var big int64
	assert.ErrorContains(t, c.Unmarshal(mustDecodeHex(t, "d8b41880"), &big), "outside the i8 range")
	assert.ErrorContains(t, c.Unmarshal(mustDecodeHex(t, "d8b520"), &big), "outside the u8 range")
	var bigf float64
	assert.ErrorContains(t, c.Unmarshal(mustDecodeHex(t, "d8bbfb3fb999999999999a"), &bigf), "f16")
	assert.Error(t, c.Unmarshal(mustDecodeHex(t, "d8bb01"), &bigf))
}

func TestUnmarshalV2_recordIDKeys(t *testing.T) {
	c := NewV2()
	var got any
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "c8826174d8b505"), &got))
	assert.Equal(t, models.RecordID{Table: "t", ID: uint64(5)}, got)

	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "c8826174d8b420"), &got))
	assert.Equal(t, models.RecordID{Table: "t", ID: int64(-1)}, got)

	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "c882617482d8b505d8bcf93e00"), &got))
	assert.Equal(t, models.RecordID{Table: "t", ID: []any{uint8(5), float32(1.5)}}, got)

	assert.Error(t, c.Unmarshal(mustDecodeHex(t, "c8826174d8ba1bffffffffffffffff"), &got))
	assert.Error(t, c.Unmarshal(mustDecodeHex(t, "c8826174d8bbf93e00"), &got))

	var typed models.RecordID
	require.NoError(t, c.Unmarshal(mustDecodeHex(t, "c882617482d8b505d8bcf93e00"), &typed))
	assert.Equal(t, models.RecordID{Table: "t", ID: []any{uint8(5), float32(1.5)}}, typed)
}

func TestRoundTripV2_keepsKinds(t *testing.T) {
	in := []any{int8(-1), uint8(1), int16(-2), uint16(2), int32(-3), uint32(3), uint64(math.MaxUint64),
		models.MustFloat16(0.5), float32(0.25), uint64(4), int64(-4), 0.1}
	b, err := NewV2().Marshal(in)
	require.NoError(t, err)
	var out []any
	require.NoError(t, NewV2().Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// A u64 up to math.MaxInt64 decoded into an interface goes back as an int.
// See the Codec documentation.
func TestRoundTripV2_u64BecomesInt(t *testing.T) {
	var got any
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "d8ba05"), &got))
	assert.Equal(t, uint64(5), got)
	b, err := NewV2().Marshal(got)
	require.NoError(t, err)
	assert.Equal(t, "05", hex.EncodeToString(b))
}

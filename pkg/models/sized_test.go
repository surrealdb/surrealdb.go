package models

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFloat16_exactness(t *testing.T) {
	for _, f := range []float64{0, 1, 1.5, -2, 65504, 5.960464477539063e-8, math.Inf(1), math.Inf(-1)} {
		h, err := NewFloat16(f)
		require.NoError(t, err, f)
		assert.Equal(t, f, h.Float64())
		assert.Equal(t, float32(f), h.Float32())
	}
	for _, f := range []float64{0.1, 65505, 65536, 1e-9, 1.0000001} {
		_, err := NewFloat16(f)
		assert.Error(t, err, f)
	}

	nan, err := NewFloat16(math.NaN())
	require.NoError(t, err)
	assert.True(t, math.IsNaN(nan.Float64()))

	negZero := MustFloat16(math.Copysign(0, -1))
	assert.Equal(t, uint16(0x8000), negZero.Bits())
	assert.Equal(t, uint16(0x3e00), MustFloat16(1.5).Bits())
	assert.Equal(t, MustFloat16(1.5), Float16FromBits(0x3e00))
	assert.Equal(t, "1.5", MustFloat16(1.5).String())
	assert.Panics(t, func() { MustFloat16(0.1) })
	assert.Equal(t, Float16{}, MustFloat16(0))
}

func TestFloat16_CBOR(t *testing.T) {
	b, err := MustFloat16(1.5).MarshalCBOR()
	require.NoError(t, err)
	assert.Equal(t, "f93e00", hex.EncodeToString(b))

	// fxamacker/cbor uses the methods too.
	b, err = cbor.Marshal(map[string]any{"h": MustFloat16(-2)})
	require.NoError(t, err)
	assert.Equal(t, "a16168f9c000", hex.EncodeToString(b))

	for in, want := range map[string]float64{
		"f93e00":                 1.5,
		"fa3fc00000":             1.5,
		"fb3ff8000000000000":     1.5,
		"d8bbf93e00":             1.5,
		"d8bbfb3ff8000000000000": 1.5,
	} {
		data, _ := hex.DecodeString(in)
		var h Float16
		require.NoError(t, h.UnmarshalCBOR(data), in)
		assert.Equal(t, want, h.Float64(), in)
	}
	for _, in := range []string{"fb3fb999999999999a", "01", "6131", "d8bb01"} {
		data, _ := hex.DecodeString(in)
		var h Float16
		assert.Error(t, h.UnmarshalCBOR(data), in)
	}
}

func TestIsSizedNumberTag(t *testing.T) {
	assert.False(t, IsSizedNumberTag(179))
	for tag := TagI8; tag <= TagF32; tag++ {
		assert.True(t, IsSizedNumberTag(tag))
	}
	assert.False(t, IsSizedNumberTag(189))
}

func TestRecordID_UnmarshalCBOR_sizedKeys(t *testing.T) {
	for in, want := range map[string]any{
		"c8826174d8b505":             uint64(5),
		"c8826174d8b420":             int64(-1),
		"c882617482d8b505d8bcf93e00": []any{uint8(5), float32(1.5)},
	} {
		data, _ := hex.DecodeString(in)
		var r RecordID
		require.NoError(t, r.UnmarshalCBOR(data), in)
		assert.Equal(t, RecordID{Table: "t", ID: want}, r, in)
	}
	for _, in := range []string{"c8826174d8ba1bffffffffffffffff", "c8826174d8bbf93e00", "c8826174d8b41880"} {
		data, _ := hex.DecodeString(in)
		var r RecordID
		assert.Error(t, r.UnmarshalCBOR(data), in)
	}
}

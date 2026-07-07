package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type config391 struct {
	MaxSteps int `json:"max_steps"`
	Priority int `json:"priority"`
}

// TestCBORFloat16ToInt reproduces issue #391: SurrealDB v3.0.4 encodes small
// integers as CBOR float16 in RPC responses.
func TestCBORFloat16ToInt(t *testing.T) {
	// map{"max_steps": float16(0.0)}
	data := []byte{0xA1, 0x69, 0x6D, 0x61, 0x78, 0x5F, 0x73, 0x74, 0x65, 0x70, 0x73, 0xF9, 0x00, 0x00}
	var cfg config391
	err := Unmarshal(data, &cfg)
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.MaxSteps)
}

func TestCBORFloat32ToInt(t *testing.T) {
	// map{"max_steps": float32(42.0)}
	data := []byte{0xA1, 0x69, 0x6D, 0x61, 0x78, 0x5F, 0x73, 0x74, 0x65, 0x70, 0x73, 0xFA, 0x42, 0x28, 0x00, 0x00}
	var cfg config391
	err := Unmarshal(data, &cfg)
	require.NoError(t, err)
	assert.Equal(t, 42, cfg.MaxSteps)
}

func TestCBORFloatExactIntegers(t *testing.T) {
	t.Run("float16 one", func(t *testing.T) {
		data := []byte{0xF9, 0x3C, 0x00}
		var i int
		err := Unmarshal(data, &i)
		require.NoError(t, err)
		assert.Equal(t, 1, i)
	})

	t.Run("float16 two", func(t *testing.T) {
		data := []byte{0xF9, 0x40, 0x00}
		var i int
		err := Unmarshal(data, &i)
		require.NoError(t, err)
		assert.Equal(t, 2, i)
	})

	t.Run("float64 exact zero", func(t *testing.T) {
		enc, err := cbor.Marshal(float64(0))
		require.NoError(t, err)
		var i int
		err = Unmarshal(enc, &i)
		require.NoError(t, err)
		assert.Equal(t, 0, i)
	})

	t.Run("float64 non-integer errors", func(t *testing.T) {
		enc, err := cbor.Marshal(123.456)
		require.NoError(t, err)
		var i int64
		err = Unmarshal(enc, &i)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float64 into Go value of type int64")
	})

	t.Run("float16 NaN to int errors", func(t *testing.T) {
		data := []byte{0xF9, 0x7E, 0x00}
		var i int
		err := Unmarshal(data, &i)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float16 into Go value of type int")
	})

	t.Run("negative exact float to uint errors", func(t *testing.T) {
		data := []byte{0xF9, 0xBC, 0x00} // float16(-1.0)
		var u uint
		err := Unmarshal(data, &u)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal CBOR float16 into Go value of type uint")
	})
}

package models_test

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

type withNone struct {
	Field models.CustomNil `cbor:"field"`
}

func TestCustomNil_unmarshal(t *testing.T) {
	codecs := map[string]codec.Unmarshaler{
		"fxamacker":      &models.CborUnmarshaler{},
		"surrealcbor":    surrealcbor.New(),
		"plain-fxamaker": plainUnmarshaler{},
	}

	accepted := map[string][]byte{
		"NONE":      {0xc6, 0xf6},
		"null":      {0xf6},
		"undefined": {0xf7},
	}
	rejected := map[string][]byte{
		"empty map": {0xa0},
		"map":       {0xa1, 0x61, 0x61, 0x01},
		"string":    {0x61, 0x61},
		"other tag": {0xc8, 0x61, 0x61},
	}

	for name, dec := range codecs {
		t.Run(name, func(t *testing.T) {
			for what, data := range accepted {
				var n models.CustomNil
				assert.NoError(t, dec.Unmarshal(data, &n), what)

				var s withNone
				enc := append([]byte{0xa1, 0x65, 'f', 'i', 'e', 'l', 'd'}, data...)
				assert.NoError(t, dec.Unmarshal(enc, &s), what)
			}
			for what, data := range rejected {
				var n models.CustomNil
				assert.Error(t, dec.Unmarshal(data, &n), what)
			}
		})
	}
}

func TestCustomNil_roundTrip(t *testing.T) {
	for name, c := range map[string]interface {
		codec.Marshaler
		codec.Unmarshaler
	}{
		"fxamacker":   &fxamackerCodec{},
		"surrealcbor": surrealcbor.New(),
	} {
		t.Run(name, func(t *testing.T) {
			data, err := c.Marshal(withNone{Field: models.None})
			require.NoError(t, err)
			assert.Equal(t, []byte{0xa1, 0x65, 'f', 'i', 'e', 'l', 'd', 0xc6, 0xf6}, data)

			var back withNone
			require.NoError(t, c.Unmarshal(data, &back))
		})
	}
}

type plainUnmarshaler struct{}

func (plainUnmarshaler) Unmarshal(data []byte, v any) error { return cbor.Unmarshal(data, v) }

type fxamackerCodec struct {
	models.CborMarshaler
	models.CborUnmarshaler
}

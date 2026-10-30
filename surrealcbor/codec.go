package surrealcbor

import (
	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/internal/cborwire"
)

type Codec struct {
	encMode cbor.EncMode
}

func (c *Codec) Unmarshal(data []byte, v any) error {
	d := &decoder{
		data: data,
		pos:  0,
	}
	return d.decode(v)
}

func (c *Codec) Marshal(v any) ([]byte, error) {
	em := getEncMode()
	data, err := em.Marshal(v)
	if err != nil {
		return nil, err
	}
	return cborwire.Normalize(data)
}

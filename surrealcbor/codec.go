package surrealcbor

import (
	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/internal/cborwire"
	"github.com/surrealdb/surrealdb.go/internal/codec"
)

const (
	cborV1 = codec.CBORv1
	cborV2 = codec.CBORv2
)

// Codec marshals and unmarshals SurrealDB CBOR.
//
// A Codec speaks one CBOR protocol version. [New] returns a plain cbor (v1)
// codec, and [NewV2] a cbor.2 codec, SurrealDB 4.0's CBOR with sized numbers.
// Connections negotiate the version with the server and switch to the cbor.2
// codec on their own (see [Codec.ForCBORVersion]), so configure a connection
// with [New] whichever server it talks to.
//
// Over cbor.2:
//
//   - Go int8, uint8, int16, uint16, int32, uint32 and float32 values are sent
//     as SurrealDB's i8, u8, i16, u16, i32, u32 and f32 (CBOR tags 180 to 185
//     and 188), and [models.Float16] as f16 (tag 187).
//   - Go uint64 and uint values are sent as u64 (tag 186) only when they exceed
//     the int64 range; otherwise they are sent as plain integers, as before.
//     A non-negative integer decoded into an interface is a uint64, so this
//     keeps a SurrealDB int an int when it makes a round trip through Go.
//   - The other way round, a SurrealDB u64 (tag 186) also decodes into an
//     interface as a uint64. Sent back, one up to math.MaxInt64 goes untagged,
//     so the server reads it as an int. u64 is the only sized kind that does
//     not keep its kind when a value read into an interface is written back.
//   - int, int64 and float64 values are plain integers and floats, as before.
//   - Decoding into an interface yields the exact kind for each tag: int8,
//     uint8, int16, uint16, int32, uint32, uint64, [models.Float16] and
//     float32. A plain CBOR float decodes as float64 whatever its width (plain
//     cbor gives float32 for half- and single-width floats), so float32 always
//     means SurrealDB's f32.
//   - Decoding into a typed destination checks that the value fits both the
//     tag's kind and the destination.
//   - Sized numbers inside a value that encodes itself (a type implementing
//     [Marshaler], time.Time, and the like) are not tagged.
//
// Plain cbor refuses tags 180 to 188, as the server does.
type Codec struct {
	encMode     cbor.EncMode
	cborVersion int
}

var _ codec.CBORVersioned = (*Codec)(nil)

// NewV2 returns a codec for cbor.2, SurrealDB 4.0's CBOR with sized numbers.
// Most code needs [New] instead: connections switch to cbor.2 themselves when
// the server offers it.
func NewV2() *Codec {
	return &Codec{
		encMode:     getEncMode(),
		cborVersion: cborV2,
	}
}

// CBORVersion returns the CBOR protocol version the codec speaks: 1 or 2.
func (c *Codec) CBORVersion() int {
	if c.cborVersion >= cborV2 {
		return cborV2
	}
	return cborV1
}

// ForCBORVersion returns a codec like c that speaks CBOR protocol version v
// (1 or 2), or nil for any other version.
func (c *Codec) ForCBORVersion(v int) codec.Codec {
	switch v {
	case cborV1, cborV2:
		if c.CBORVersion() == v {
			return c
		}
		return &Codec{encMode: c.encMode, cborVersion: v}
	default:
		return nil
	}
}

func (c *Codec) Unmarshal(data []byte, v any) error {
	d := &decoder{
		data:        data,
		pos:         0,
		cborVersion: c.cborVersion,
	}
	return d.decode(v)
}

func (c *Codec) Marshal(v any) ([]byte, error) {
	em := c.encMode
	if em == nil {
		em = getEncMode()
	}
	data, err := em.Marshal(v)
	if err != nil {
		return nil, err
	}
	if c.CBORVersion() >= cborV2 {
		// Tag the sized numbers first: tagSizedNumbers walks data alongside v,
		// so it needs the encoder's output as it was written. Normalize then
		// leaves the content of tags 180 to 188 alone, so the two compose.
		if data, err = tagSizedNumbers(data, v); err != nil {
			return nil, err
		}
	}
	return cborwire.Normalize(data)
}

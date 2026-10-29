package models

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

type DecimalString string

type CustomNil struct{}

func (c *CustomNil) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagNone,
		Content: nil,
	})
}

// UnmarshalCBOR decodes NONE (tag 6), or a CBOR null or undefined, into c.
// Any other value is an error, rather than being silently dropped.
func (c *CustomNil) UnmarshalCBOR(data []byte) error {
	if len(data) == 1 && (data[0] == cborNull || data[0] == cborUndefined) {
		*c = CustomNil{}
		return nil
	}

	var tag cbor.RawTag
	if err := cbor.Unmarshal(data, &tag); err != nil || data[0]>>5 != cborMajorTag {
		return fmt.Errorf("cannot unmarshal CBOR value 0x%x into models.CustomNil", data)
	}
	if tag.Number != TagNone {
		return fmt.Errorf("cannot unmarshal CBOR tag %d into models.CustomNil", tag.Number)
	}

	*c = CustomNil{}
	return nil
}

// UnMarshalCBOR decodes NONE into c.
//
// Deprecated: misspelled, so the CBOR decoders never called it. Use [CustomNil.UnmarshalCBOR].
func (c *CustomNil) UnMarshalCBOR(data []byte) error {
	return c.UnmarshalCBOR(data)
}

const (
	cborNull      = 0xf6
	cborUndefined = 0xf7
	cborMajorTag  = 6
)

// None is SurrealQL's NONE: something that does not exist, such as a
// missing field. It is not NULL, which means a field exists but has no value.
//
// In array-style record ids, None is also the lowest part, as in ['London', NONE].
// Use it inside []any{...}, not as a scalar begin/end of a [Range] — SurrealDB
// rejects NONE as a scalar record id.
var None = CustomNil{}

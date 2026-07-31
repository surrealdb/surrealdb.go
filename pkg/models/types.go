package models

import (
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

func (c *CustomNil) UnMarshalCBOR(data []byte) error {
	*c = CustomNil{}
	return nil
}

// None is SurrealQL's NONE: something that does not exist, such as a
// missing field. It is not NULL, which means a field exists but has no value.
//
// In array-style record ids, None is also the lowest part, as in ['London', NONE].
// Use it inside []any{...}, not as a scalar begin/end of an [OpenRange] — SurrealDB
// rejects NONE as a scalar record id.
var None = CustomNil{}

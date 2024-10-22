package models

import (
	"github.com/fxamacker/cbor/v2"
)

type DecimalString string

type CustomNil struct {
}

func (c *CustomNil) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  TagNone,
		Content: nil,
	})
}

func (c *CustomNil) UnMarshalCBOR(data []byte) error {
	*c = CustomNil{}
	return nil
}

var None = CustomNil{}

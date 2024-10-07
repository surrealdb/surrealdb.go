package models

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"time"
)

type TableOrRecord interface {
	string | Table | RecordID | []Table | []RecordID
}

type Table string

// type UUID string

// type UUIDBin []byte
type UUID struct {
	uuid.UUID
}

type Decimal string

type CustomDateTime time.Time

func (d *CustomDateTime) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	totalNS := time.Time(*d).Nanosecond()

	s := totalNS / 1_000_000_000
	ns := totalNS % 1_000_000_000

	return enc.Marshal(cbor.Tag{
		Number:  uint64(DateTimeCompactString),
		Content: [2]int64{int64(s), int64(ns)},
	})
}

func (d *CustomDateTime) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	s := temp[0].(int64)
	ns := temp[1].(int64)

	*d = CustomDateTime(time.Unix(s, ns))

	return nil
}

type CustomNil struct {
}

func (c *CustomNil) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(NoneTag),
		Content: nil,
	})
}

func (c *CustomNil) UnMarshalCBOR(data []byte) error {
	*c = CustomNil{}
	return nil
}

var None = CustomNil{}

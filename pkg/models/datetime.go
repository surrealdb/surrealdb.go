package models

import (
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

// CustomDateTime embeds time.Time
type CustomDateTime struct {
	time.Time
}

func (d *CustomDateTime) MarshalCBOR() ([]byte, error) {
	if d.IsZero() {
		return cbor.Marshal(cbor.Tag{Number: TagNone})
	}

	totalNS := d.UnixNano()

	s := totalNS / constants.OneSecondToNanoSecond
	ns := totalNS % constants.OneSecondToNanoSecond

	return cbor.Marshal(cbor.Tag{
		Number:  TagCustomDatetime,
		Content: [2]int64{s, ns},
	})
}

func (d *CustomDateTime) UnmarshalCBOR(data []byte) error {
	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return err
	}

	if tag.Number == TagNone {
		return nil
	}

	if tag.Number != TagCustomDatetime {
		return fmt.Errorf("unexpected tag number: got %d, want %d", tag.Number, TagCustomDatetime)
	}

	var temp [2]int64
	err := cbor.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	s := temp[0]
	ns := temp[1]

	*d = CustomDateTime{time.Unix(s, ns)}

	return nil
}

func (d *CustomDateTime) IsZero() bool {
	return d == nil || d.Time.IsZero()
}

func (d *CustomDateTime) String() string {
	layout := "2006-01-02T15:04:05Z"
	return d.Format(layout)
}

func (d *CustomDateTime) SurrealString() string {
	return fmt.Sprintf("<datetime> '%s'", d.String())
}

package models

import (
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// CustomDateTime embeds time.Time
type CustomDateTime struct {
	time.Time
}

func (d *CustomDateTime) MarshalCBOR() ([]byte, error) {
	if d.IsZero() {
		return cbor.Marshal(cbor.Tag{Number: TagNone})
	}

	s := d.Unix()
	ns := int64(d.Nanosecond())

	return cbor.Marshal(cbor.Tag{
		Number:  TagCustomDatetime,
		Content: [2]int64{s, ns},
	})
}

// UnmarshalCBOR decodes a datetime (tag 12) into d. A date (tag 100) or a
// timestamp (tag 190) decodes as the datetime plain cbor carries it as: a
// date's midnight UTC, or a timestamp's wall clock read as UTC. NONE leaves d
// as it is.
func (d *CustomDateTime) UnmarshalCBOR(data []byte) error {
	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return err
	}

	if tag.Number == TagNone {
		return nil
	}

	if tag.Number == TagPlainDate || tag.Number == TagTimestamp {
		widened, err := cborwire.Normalize(data, cborwire.Plain)
		if err != nil {
			return err
		}
		return d.UnmarshalCBOR(widened)
	}

	if tag.Number != TagCustomDatetime {
		return fmt.Errorf("unexpected tag number: got %d, want %d, %d or %d",
			tag.Number, TagCustomDatetime, TagPlainDate, TagTimestamp)
	}

	var temp [2]int64
	err := cbor.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	s := temp[0]
	ns := temp[1]

	*d = CustomDateTime{time.Unix(s, ns).UTC()}

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

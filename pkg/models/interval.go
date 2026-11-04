package models

import (
	"cmp"
	"fmt"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// Interval is a span of months, days and microseconds, such as
// P1Y2M3DT4H5M6.5S: SurrealDB 4.0's interval kind, as in PostgreSQL.
//
// The three parts are kept apart, because a month and a day have no fixed
// length: a month after January 31 is not 30 days after it. Each part may be
// negative on its own, and every combination is valid.
//
// Like the server, [Interval.Equal] and [Interval.Compare] use the total of
// the parts, in which a month is 30 days and a day is 24 hours, so P1M equals
// P30D. == compares the parts, as with time.Time: a Go map keeps P1M and P30D
// apart, where a SurrealDB set holds only one of them.
//
// Over cbor.2 an Interval is sent as tag 191 and decoded from it. Over plain
// cbor it is sent as its ISO 8601 text, the way the server widens it, and
// decoded from that text.
//
// The zero value is PT0S, a real interval rather than NONE. For a field that
// may be left out, use an *Interval with omitempty: omitempty never drops an
// Interval, and omitzero always drops PT0S.
type Interval struct {
	Months       int32
	Days         int32
	Microseconds int64
}

// Compare returns -1 if the total of i is below that of o, +1 if it is above,
// and 0 if they are equal, where a month is 30 days and a day is 24 hours.
func (i Interval) Compare(o Interval) int {
	iDays, iRest := i.total()
	oDays, oRest := o.total()
	if c := cmp.Compare(iDays, oDays); c != 0 {
		return c
	}
	return cmp.Compare(iRest, oRest)
}

// Equal reports whether i and o have the same total, as the server compares
// intervals: P1M equals P30D, and PT24H equals P1D.
func (i Interval) Equal(o Interval) bool {
	return i.Compare(o) == 0
}

// total returns the total of i as whole days and the microseconds left over,
// from zero up to a day. Neither can overflow.
func (i Interval) total() (days, micros int64) {
	days = int64(i.Months)*30 + int64(i.Days) + i.Microseconds/microsPerDay
	micros = i.Microseconds % microsPerDay
	if micros < 0 {
		days, micros = days-1, micros+microsPerDay
	}
	return days, micros
}

// String returns i as the server writes it: ISO 8601 with the years folded
// out of the months, such as P1Y2M3DT4H5M6.5S, and PT0S for zero. Each time
// part carries the sign of the microseconds, as in PT-1M-30S.
func (i Interval) String() string {
	return cborwire.FormatInterval(i.Months, i.Days, i.Microseconds)
}

// SurrealString returns i as a SurrealQL interval literal, such as ivl'P1D'.
func (i Interval) SurrealString() string {
	return "ivl'" + i.String() + "'"
}

// MarshalText returns i as String writes it.
func (i Interval) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText parses the form String writes. Each number may carry its own
// sign, and the seconds may have up to six digits after the point. The server
// reads more forms, such as PostgreSQL's 1 mon 2 days, which UnmarshalText
// does not.
func (i *Interval) UnmarshalText(text []byte) error {
	months, days, micros, err := cborwire.ParseInterval(string(text))
	if err != nil {
		return err
	}
	*i = Interval{Months: months, Days: days, Microseconds: micros}
	return nil
}

// MarshalCBOR encodes i as tag 191 around [months, days, microseconds]. An
// encoder for plain cbor widens it.
func (i Interval) MarshalCBOR() ([]byte, error) {
	b := cborwire.AppendArrayHead(cborwire.AppendTag(make([]byte, 0, 24), TagInterval), 3)
	b = cborwire.AppendInt(cborwire.AppendInt(b, int64(i.Months)), int64(i.Days))
	return cborwire.AppendInt(b, i.Microseconds), nil
}

// UnmarshalCBOR decodes an interval (tag 191) into i, or its ISO 8601 text,
// which is how plain cbor carries one. NULL and NONE leave i as it is.
func (i *Interval) UnmarshalCBOR(data []byte) error {
	const into = "models.Interval"
	item, absent, err := readKindItem(data, TagInterval, into)
	if err != nil || absent {
		return err
	}
	var parsed Interval
	switch {
	case item.tagged && item.tag == TagInterval:
		parsed.Months, parsed.Days, parsed.Microseconds, err = cborwire.ReadInterval(item.content)
	case !item.tagged && item.content[0]>>5 == cborMajorText:
		var text string
		if text, err = cborwire.ReadText(item.content); err == nil {
			err = parsed.UnmarshalText([]byte(text))
		}
	default:
		return item.mismatch(into)
	}
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	*i = parsed
	return nil
}

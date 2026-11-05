package models

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// PlainDate is a calendar date with no time of day and no time zone, such as
// 2024-02-29: SurrealDB 4.0's date kind.
//
// It holds the days since 1970-01-01 in the proleptic Gregorian calendar,
// from -262143-01-01 to +262142-12-31. Those two days stand for -infinity
// and infinity, as in PostgreSQL: see [PlainDateNegInfinity] and
// [PlainDateInfinity]. PlainDate values can be compared with ==, and
// [PlainDate.Compare] orders them.
//
// Over cbor.2 a PlainDate is sent as tag 100 and decoded from it. Over plain
// cbor it is sent as the datetime at its midnight UTC, the way the server
// widens it, and decoded from such a datetime. The server widens infinity to
// its latest datetime, +262142-12-31T23:59:59.999999999Z, rather than to
// midnight, and so does the SDK.
//
// The zero value is 1970-01-01, a real date rather than NONE. For a field
// that may be left out, use a *PlainDate with omitempty: omitempty never drops
// a PlainDate, and omitzero always drops 1970-01-01.
type PlainDate struct {
	days int32
}

var (
	// PlainDateInfinity is infinity: the last day a date holds, +262142-12-31.
	PlainDateInfinity = PlainDate{days: int32(cborwire.MaxDateDays)}
	// PlainDateNegInfinity is -infinity: the first day a date holds, -262143-01-01.
	PlainDateNegInfinity = PlainDate{days: int32(cborwire.MinDateDays)}
)

// NewPlainDate returns the date year-month-day, where year 0 is 1 BCE. It
// refuses a day or month that does not exist, such as February 30, rather
// than rolling it over, and the first and last days, which stand for the
// infinities.
func NewPlainDate(year int, month time.Month, day int) (PlainDate, error) {
	days, ok := daysOfCivil(year, month, day)
	if !ok || days == cborwire.MinDateDays || days == cborwire.MaxDateDays {
		return PlainDate{}, fmt.Errorf("invalid date: year %d, month %d, day %d", year, month, day)
	}
	return PlainDate{days: int32(days)}, nil //nolint:gosec // within the range of a date
}

// PlainDateFromDays returns the date days after 1970-01-01, or before it for
// a negative number, which is how cbor.2 carries a date. The first and last
// days a date holds are the infinities.
func PlainDateFromDays(days int64) (PlainDate, error) {
	if days < cborwire.MinDateDays || days > cborwire.MaxDateDays {
		return PlainDate{}, fmt.Errorf("%d days from 1970-01-01 is outside the range of a date", days)
	}
	return PlainDate{days: int32(days)}, nil //nolint:gosec // range-checked above
}

// PlainDateOf returns the date on which t falls in t's own location. A time
// on the first or the last day a date holds gives -infinity or infinity.
func PlainDateOf(t time.Time) (PlainDate, error) {
	year, month, day := t.Date()
	days, ok := daysOfCivil(year, month, day)
	if !ok {
		return PlainDate{}, fmt.Errorf("%v is outside the range of a date", t)
	}
	return PlainDateFromDays(days)
}

// ParsePlainDate parses a date as the server does, reading YYYY-MM-DD as
// chrono does: a year of up to four digits, or with a sign and more, as in
// +10000-01-01 or -0001-12-31; a month and a day of one or two digits; and
// spaces allowed around each number. It also reads "infinity", "+infinity"
// or "-infinity" in any letter case, and "epoch" for 1970-01-01. Like the
// server, it refuses the first and last days written as dates: write them as
// the infinities.
func ParsePlainDate(s string) (PlainDate, error) {
	text := strings.TrimSpace(s)
	switch sign := parseInfinity(text); {
	case sign > 0:
		return PlainDateInfinity, nil
	case sign < 0:
		return PlainDateNegInfinity, nil
	case strings.EqualFold(text, textEpoch):
		return PlainDate{}, nil
	}
	days, err := parseCivilDate(text)
	if err != nil || days == cborwire.MinDateDays || days == cborwire.MaxDateDays {
		return PlainDate{}, fmt.Errorf("invalid date %q", s)
	}
	return PlainDate{days: int32(days)}, nil //nolint:gosec // within the range of a date
}

// MustParsePlainDate is like [ParsePlainDate] but panics when s is not a date.
func MustParsePlainDate(s string) PlainDate {
	d, err := ParsePlainDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Days returns the days since 1970-01-01, negative before it.
func (d PlainDate) Days() int64 {
	return int64(d.days)
}

// Date returns the year, month and day of d. Year 0 is 1 BCE.
func (d PlainDate) Date() (year int, month time.Month, day int) {
	return civilOfDays(int64(d.days))
}

// In returns the time at the start of d in loc. Where loc has no midnight
// that day, as when daylight saving time starts at midnight, it follows
// time.Date, so the result can fall in the day before. Use time.UTC to read
// the same date back with [PlainDateOf].
func (d PlainDate) In(loc *time.Location) time.Time {
	year, month, day := d.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// IsInfinite reports whether d is infinity or -infinity.
func (d PlainDate) IsInfinite() bool {
	return d == PlainDateInfinity || d == PlainDateNegInfinity
}

// Compare returns -1 if d is before o, +1 if it is after, and 0 if they are
// the same date. -infinity is before every other date, and infinity after.
func (d PlainDate) Compare(o PlainDate) int {
	return cmp.Compare(d.days, o.days)
}

// String returns d as the server writes it: YYYY-MM-DD, with a sign and at
// least four digits for a year outside 0000 to 9999, or "infinity" or
// "-infinity".
func (d PlainDate) String() string {
	switch d {
	case PlainDateInfinity:
		return textInfinity
	case PlainDateNegInfinity:
		return textNegInfinity
	}
	return string(appendCivilDate(make([]byte, 0, 16), int64(d.days)))
}

// SurrealString returns d as a SurrealQL date literal, such as pd'2024-02-29'.
func (d PlainDate) SurrealString() string {
	return "pd'" + d.String() + "'"
}

// MarshalText returns d as String writes it.
func (d PlainDate) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText parses text as ParsePlainDate does.
func (d *PlainDate) UnmarshalText(text []byte) error {
	parsed, err := ParsePlainDate(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalCBOR encodes d as tag 100 around its days since 1970-01-01. An
// encoder for plain cbor widens it.
func (d PlainDate) MarshalCBOR() ([]byte, error) {
	b := cborwire.AppendTag(make([]byte, 0, 8), TagPlainDate)
	return cborwire.AppendInt(b, int64(d.days)), nil
}

// UnmarshalCBOR decodes a date (tag 100) into d, or a datetime (tag 12) at
// midnight UTC, which is how plain cbor carries one. The server's latest
// datetime is infinity, as the server widens infinity to it. A timestamp at
// midnight decodes as its date too. NULL and NONE leave d as it is.
func (d *PlainDate) UnmarshalCBOR(data []byte) error {
	const into = "models.PlainDate"
	item, absent, err := readKindItem(data, TagPlainDate, into)
	if err != nil || absent {
		return err
	}
	var days int64
	switch {
	case item.tagged && item.tag == TagPlainDate:
		days, err = cborwire.ReadInt(item.content)
	case item.tagged && item.tag == TagCustomDatetime:
		days, err = daysAtMidnight(item.content)
	default:
		return item.mismatch(into)
	}
	var date PlainDate
	if err == nil {
		date, err = PlainDateFromDays(days)
	}
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	*d = date
	return nil
}

// daysAtMidnight reads the content of a datetime (tag 12), which must be at
// midnight UTC or be the server's latest datetime, as days since 1970-01-01.
func daysAtMidnight(content []byte) (int64, error) {
	sec, nsec, err := cborwire.ReadDatetime(content)
	switch {
	case err != nil:
		return 0, err
	case sec == cborwire.MaxTimestampSeconds && nsec == cborwire.MaxNanosecond:
		return cborwire.MaxDateDays, nil
	case nsec != 0 || sec%secondsPerDay != 0:
		return 0, fmt.Errorf("the datetime %s is not at midnight UTC", time.Unix(sec, nsec).UTC().Format(time.RFC3339Nano))
	}
	return sec / secondsPerDay, nil
}

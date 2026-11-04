package models

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// PlainTime is a time of day with no date and no time zone, such as
// 12:34:56.789: SurrealDB 4.0's time kind.
//
// It holds the nanoseconds since midnight, from 00:00:00 up to and including
// 24:00:00, the end of the day, as in PostgreSQL. PlainTime values can be
// compared with ==, and [PlainTime.Compare] orders them.
//
// Over cbor.2 a PlainTime is sent as tag 189 and decoded from it. Over plain
// cbor it is sent as the duration since midnight (tag 14), the way the server
// widens it, and decoded from such a duration.
//
// The zero value is 00:00:00, a real time rather than NONE. For a field that
// may be left out, use a *PlainTime with omitempty: omitempty never drops a
// PlainTime, and omitzero always drops 00:00:00.
type PlainTime struct {
	nanos int64
}

// NewPlainTime returns the time hour:minute:sec and nsec nanoseconds. Each
// field must be within its range, and the only time past 23:59:59.999999999
// is 24:00:00.
func NewPlainTime(hour, minute, sec, nsec int) (PlainTime, error) {
	nanos := (int64(hour)*3600+int64(minute)*60+int64(sec))*nsPerSecond + int64(nsec)
	if hour < 0 || hour > 24 || minute < 0 || minute > 59 || sec < 0 || sec > 59 ||
		nsec < 0 || int64(nsec) >= nsPerSecond || nanos > nsPerDay {
		return PlainTime{}, fmt.Errorf("invalid time: hour %d, minute %d, second %d, nanosecond %d", hour, minute, sec, nsec)
	}
	return PlainTime{nanos: nanos}, nil
}

// PlainTimeFromDuration returns the time d after midnight. d must be from 0
// to 24 hours.
func PlainTimeFromDuration(d time.Duration) (PlainTime, error) {
	if d < 0 || int64(d) > nsPerDay {
		return PlainTime{}, fmt.Errorf("a time of day cannot be %v after midnight", d)
	}
	return PlainTime{nanos: int64(d)}, nil
}

// PlainTimeOf returns the time of day of t in t's own location.
func PlainTimeOf(t time.Time) PlainTime {
	hour, minute, sec := t.Clock()
	return PlainTime{nanos: (int64(hour)*3600+int64(minute)*60+int64(sec))*nsPerSecond + int64(t.Nanosecond())}
}

// ParsePlainTime parses a time as the server does: H:MM, H:MM:SS or
// H:MM:SS.fraction, with one or two digits in each field and up to nine in
// the fraction, up to and including 24:00:00. It ignores spaces around the
// text.
func ParsePlainTime(s string) (PlainTime, error) {
	nanos, err := parseTimeOfDay(strings.TrimSpace(s))
	if err != nil {
		return PlainTime{}, fmt.Errorf("invalid time %q", s)
	}
	return PlainTime{nanos: nanos}, nil
}

// MustParsePlainTime is like [ParsePlainTime] but panics when s is not a time.
func MustParsePlainTime(s string) PlainTime {
	t, err := ParsePlainTime(s)
	if err != nil {
		panic(err)
	}
	return t
}

// SinceMidnight returns the time since midnight: 24h for 24:00:00.
func (t PlainTime) SinceMidnight() time.Duration {
	return time.Duration(t.nanos)
}

// Clock returns the hour, minute and second of t. The hour is 24 for
// 24:00:00 and below 24 otherwise.
func (t PlainTime) Clock() (hour, minute, sec int) {
	seconds := int(t.nanos / nsPerSecond) //nolint:gosec // at most 86400
	return seconds / 3600, seconds / 60 % 60, seconds % 60
}

// Nanosecond returns the nanoseconds within the second of t.
func (t PlainTime) Nanosecond() int {
	return int(t.nanos % nsPerSecond) //nolint:gosec // below a second
}

// Compare returns -1 if t is before o, +1 if it is after, and 0 if they are
// the same time.
func (t PlainTime) Compare(o PlainTime) int {
	return cmp.Compare(t.nanos, o.nanos)
}

// String returns t as the server writes it: HH:MM:SS, followed by a fraction
// of three, six or nine digits when t has one, whichever is the shortest
// exact.
func (t PlainTime) String() string {
	return string(appendTimeOfDay(make([]byte, 0, 18), t.nanos))
}

// SurrealString returns t as a SurrealQL time literal, such as pt'12:34:56'.
func (t PlainTime) SurrealString() string {
	return "pt'" + t.String() + "'"
}

// MarshalText returns t as String writes it.
func (t PlainTime) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText parses text as ParsePlainTime does.
func (t *PlainTime) UnmarshalText(text []byte) error {
	parsed, err := ParsePlainTime(string(text))
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// MarshalCBOR encodes t as tag 189 around its nanoseconds since midnight. An
// encoder for plain cbor widens it.
func (t PlainTime) MarshalCBOR() ([]byte, error) {
	b := cborwire.AppendTag(make([]byte, 0, 11), TagPlainTime)
	return cborwire.AppendUint(b, uint64(t.nanos)), nil //nolint:gosec // never negative
}

// UnmarshalCBOR decodes a time (tag 189) into t, or a duration (tag 14) of at
// most 24 hours, which is how plain cbor carries one. NULL and NONE leave t as
// it is.
func (t *PlainTime) UnmarshalCBOR(data []byte) error {
	const into = "models.PlainTime"
	item, absent, err := readKindItem(data, TagPlainTime, into)
	if err != nil || absent {
		return err
	}
	var nanos int64
	switch {
	case item.tagged && item.tag == TagPlainTime:
		var u uint64
		if u, err = cborwire.ReadUint(item.content); err == nil && u > uint64(nsPerDay) {
			err = fmt.Errorf("%d nanoseconds is past the end of the day", u)
		}
		nanos = int64(u) //nolint:gosec // at most a day, checked above
	case item.tagged && item.tag == TagCustomDuration:
		if nanos, err = cborwire.ReadDuration(item.content); err == nil && nanos > nsPerDay {
			err = fmt.Errorf("the duration %v is longer than a day", time.Duration(nanos))
		}
	default:
		return item.mismatch(into)
	}
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	t.nanos = nanos
	return nil
}

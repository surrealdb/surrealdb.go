package models

import (
	"cmp"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// Timestamp is a date and time of day with no time zone, such as
// 2024-02-29T12:34:56.789: SurrealDB 4.0's timestamp kind, as in PostgreSQL.
//
// A Timestamp is the reading of a wall clock, not an instant: the same
// reading is a different instant in each time zone. For an instant,
// SurrealDB's datetime, use a time.Time.
//
// It holds the seconds and nanoseconds since 1970-01-01T00:00:00 of that
// reading, from -262143-01-01T00:00:00 to +262142-12-31T23:59:59.999999999.
// Those two stand for -infinity and infinity: see [TimestampNegInfinity] and
// [TimestampInfinity]. Timestamp values can be compared with ==, and
// [Timestamp.Compare] orders them.
//
// Over cbor.2 a Timestamp is sent as tag 190 and decoded from it. Over plain
// cbor it is sent as a datetime (tag 12), reading the wall clock as UTC, the
// way the server widens it, and decoded from a datetime.
//
// The zero value is 1970-01-01T00:00:00, a real timestamp rather than NONE.
// For a field that may be left out, use a *Timestamp with omitempty:
// omitempty never drops a Timestamp, and omitzero always drops the zero value.
type Timestamp struct {
	sec  int64
	nsec int32
}

var (
	// TimestampInfinity is infinity: the last instant a timestamp holds,
	// +262142-12-31T23:59:59.999999999.
	TimestampInfinity = Timestamp{sec: cborwire.MaxTimestampSeconds, nsec: int32(cborwire.MaxNanosecond)}
	// TimestampNegInfinity is -infinity: the first instant a timestamp holds,
	// -262143-01-01T00:00:00.
	TimestampNegInfinity = Timestamp{sec: cborwire.MinTimestampSeconds}
)

// maxNanosCarry bounds the seconds a count of nanoseconds holds.
const maxNanosCarry = math.MaxInt64/nsPerSecond + 1

// TimestampFromUnix returns the timestamp sec seconds and nsec nanoseconds
// after 1970-01-01T00:00:00, which is how cbor.2 carries one. As with
// time.Unix, nsec may be outside 0 to 999999999. The first and last instants
// a timestamp holds are the infinities.
func TimestampFromUnix(sec, nsec int64) (Timestamp, error) {
	// Bound sec first, so adding the seconds in nsec cannot overflow.
	if sec >= cborwire.MinTimestampSeconds-maxNanosCarry && sec <= cborwire.MaxTimestampSeconds+maxNanosCarry {
		s, ns := sec+nsec/nsPerSecond, nsec%nsPerSecond
		if ns < 0 {
			s, ns = s-1, ns+nsPerSecond
		}
		if s >= cborwire.MinTimestampSeconds && s <= cborwire.MaxTimestampSeconds {
			return Timestamp{sec: s, nsec: int32(ns)}, nil //nolint:gosec // 0 to 999999999
		}
	}
	return Timestamp{}, fmt.Errorf("%d seconds and %d nanoseconds since 1970 is outside the range of a timestamp", sec, nsec)
}

// TimestampOf returns the reading of t's wall clock in t's own location. Call
// t.UTC() first for the reading in UTC.
func TimestampOf(t time.Time) (Timestamp, error) {
	sec := t.Unix()
	if sec < cborwire.MinTimestampSeconds-secondsPerDay || sec > cborwire.MaxTimestampSeconds+secondsPerDay {
		return Timestamp{}, fmt.Errorf("%v is outside the range of a timestamp", t)
	}
	_, offset := t.Zone()
	return TimestampFromUnix(sec+int64(offset), int64(t.Nanosecond()))
}

// ParseTimestamp parses a timestamp as the server does: a date, then T or a
// space, then a time of day before 24:00:00, as in 2024-02-29T12:34:56.789.
// The date is read as chrono reads one for the server: a year of up to four
// digits, or with a sign and more, a month and a day of one or two digits,
// and the first and last days included. The time is H:MM, H:MM:SS or
// H:MM:SS.fraction, with up to nine digits in the fraction. It also reads
// "infinity", "+infinity" or "-infinity" in any letter case, and "epoch" for
// 1970-01-01T00:00:00, and ignores spaces around the text and the time. Like
// the server, it refuses the first and last instants written as timestamps:
// write them as the infinities.
func ParseTimestamp(s string) (Timestamp, error) {
	text := strings.TrimSpace(s)
	switch sign := parseInfinity(text); {
	case sign > 0:
		return TimestampInfinity, nil
	case sign < 0:
		return TimestampNegInfinity, nil
	case strings.EqualFold(text, textEpoch):
		return Timestamp{}, nil
	}
	at := strings.IndexAny(text, "T ")
	if at < 0 {
		return Timestamp{}, fmt.Errorf("invalid timestamp %q", s)
	}
	days, err := parseCivilDate(text[:at])
	if err != nil {
		return Timestamp{}, fmt.Errorf("invalid timestamp %q", s)
	}
	nanos, err := parseTimeOfDay(strings.TrimSpace(text[at+1:]))
	if err != nil || nanos >= nsPerDay {
		return Timestamp{}, fmt.Errorf("invalid timestamp %q", s)
	}
	ts := Timestamp{sec: days*secondsPerDay + nanos/nsPerSecond, nsec: int32(nanos % nsPerSecond)} //nolint:gosec // below a second
	if ts.IsInfinite() {
		return Timestamp{}, fmt.Errorf("invalid timestamp %q", s)
	}
	return ts, nil
}

// MustParseTimestamp is like [ParseTimestamp] but panics when s is not a
// timestamp.
func MustParseTimestamp(s string) Timestamp {
	ts, err := ParseTimestamp(s)
	if err != nil {
		panic(err)
	}
	return ts
}

// Unix returns the whole seconds since 1970-01-01T00:00:00 of ts, negative
// before it.
func (ts Timestamp) Unix() int64 {
	return ts.sec
}

// Nanosecond returns the nanoseconds within the second of ts.
func (ts Timestamp) Nanosecond() int {
	return int(ts.nsec)
}

// In returns the instant at which a clock in loc reads ts. Where loc skips
// or repeats that reading, as when daylight saving time starts or ends, it
// follows time.Date. In(time.UTC) reads the wall clock as UTC, as plain cbor
// does, and [TimestampOf] reads it back.
func (ts Timestamp) In(loc *time.Location) time.Time {
	utc := time.Unix(ts.sec, int64(ts.nsec)).UTC()
	if loc == time.UTC {
		return utc
	}
	year, month, day := utc.Date()
	hour, minute, sec := utc.Clock()
	return time.Date(year, month, day, hour, minute, sec, int(ts.nsec), loc)
}

// IsInfinite reports whether ts is infinity or -infinity.
func (ts Timestamp) IsInfinite() bool {
	return ts == TimestampInfinity || ts == TimestampNegInfinity
}

// Compare returns -1 if ts is before o, +1 if it is after, and 0 if they are
// the same timestamp. -infinity is before every other timestamp, and
// infinity after.
func (ts Timestamp) Compare(o Timestamp) int {
	if c := cmp.Compare(ts.sec, o.sec); c != 0 {
		return c
	}
	return cmp.Compare(ts.nsec, o.nsec)
}

// String returns ts as the server writes it: YYYY-MM-DD, with a sign and at
// least four digits for a year outside 0000 to 9999, then T and HH:MM:SS
// with a fraction of three, six or nine digits when ts has one, such as
// 2024-02-29T12:34:56.789, or "infinity" or "-infinity".
func (ts Timestamp) String() string {
	switch ts {
	case TimestampInfinity:
		return textInfinity
	case TimestampNegInfinity:
		return textNegInfinity
	}
	days := ts.sec / secondsPerDay
	if ts.sec%secondsPerDay < 0 {
		days--
	}
	b := append(appendCivilDate(make([]byte, 0, 32), days), 'T')
	return string(appendTimeOfDay(b, (ts.sec-days*secondsPerDay)*nsPerSecond+int64(ts.nsec)))
}

// SurrealString returns ts as a SurrealQL timestamp literal, such as
// ts'2024-02-29T12:34:56'.
func (ts Timestamp) SurrealString() string {
	return "ts'" + ts.String() + "'"
}

// MarshalText returns ts as String writes it.
func (ts Timestamp) MarshalText() ([]byte, error) {
	return []byte(ts.String()), nil
}

// UnmarshalText parses text as ParseTimestamp does.
func (ts *Timestamp) UnmarshalText(text []byte) error {
	parsed, err := ParseTimestamp(string(text))
	if err != nil {
		return err
	}
	*ts = parsed
	return nil
}

// MarshalCBOR encodes ts as tag 190 around [seconds, nanoseconds]. An encoder
// for plain cbor widens it.
func (ts Timestamp) MarshalCBOR() ([]byte, error) {
	b := cborwire.AppendArrayHead(cborwire.AppendTag(make([]byte, 0, 16), TagTimestamp), 2)
	b = cborwire.AppendInt(b, ts.sec)
	return cborwire.AppendUint(b, uint64(ts.nsec)), nil //nolint:gosec // never negative
}

// UnmarshalCBOR decodes a timestamp (tag 190) into ts, or a datetime (tag 12),
// which is how plain cbor carries one. As the server does, it carries
// nanoseconds of a second or more, as a leap second holds, into the seconds;
// a datetime may hold them only in a leap second, at the end of a minute. A
// date decodes as its midnight. NULL and NONE leave ts as it is.
func (ts *Timestamp) UnmarshalCBOR(data []byte) error {
	const into = "models.Timestamp"
	item, absent, err := readKindItem(data, TagTimestamp, into)
	if err != nil || absent {
		return err
	}
	var sec, nsec int64
	switch {
	case item.tagged && item.tag == TagTimestamp:
		sec, nsec, err = cborwire.ReadTimestamp(item.content)
	case item.tagged && item.tag == TagCustomDatetime:
		sec, nsec, err = cborwire.ReadDatetime(item.content)
	default:
		return item.mismatch(into)
	}
	var parsed Timestamp
	if err == nil {
		parsed, err = TimestampFromUnix(sec, nsec)
	}
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	*ts = parsed
	return nil
}

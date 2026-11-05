package cborwire

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
)

// SurrealDB 4.0's date, time, timestamp, interval, bigdecimal and regex kinds
// exist only in cbor.2, which carries each as its own tag. Plain cbor has no
// form for them, so Normalize widens each, under [Plain], the way the server
// does. The tag numbers are listed in spec/cbor-tags.yaml of surrealdb-protocol.
const (
	tagDate       = 100
	tagTime       = 189
	tagTimestamp  = 190
	tagInterval   = 191
	tagBigDecimal = 192
	tagRegex      = 193
)

// The ranges of the date and timestamp kinds, which are those of the server's
// calendar: from -262143-01-01 to +262142-12-31. The first and the last day,
// and the first and the last instant, stand for -infinity and infinity.
const (
	MinDateDays         int64 = -96465292
	MaxDateDays         int64 = 95026236
	MinTimestampSeconds       = MinDateDays * secondsPerDay
	MaxTimestampSeconds       = MaxDateDays*secondsPerDay + secondsPerDay - 1
	// MaxNanosecond is the nanosecond of the last instant.
	MaxNanosecond = nanosPerSecond - 1
	// NanosPerDay is 24:00:00, the latest time of day, in nanoseconds.
	NanosPerDay = secondsPerDay * nanosPerSecond

	secondsPerDay int64 = 86400
)

const (
	simpleNull      = 0xf6
	simpleUndefined = 0xf7
	tagNone         = 6

	microsPerSecond = 1_000_000
	microsPerMinute = 60 * microsPerSecond
	microsPerHour   = 60 * microsPerMinute

	// The limits of a bigdecimal, those of PostgreSQL's numeric: 131072 digits
	// before the point and 16383 after.
	bigDecimalMaxDigits         = 131072 + 16383
	bigDecimalMaxExponent       = bigDecimalMaxDigits
	bigDecimalMaxScale          = 2 * bigDecimalMaxDigits
	bigDecimalMaxExponentDigits = 18

	// twoTo96 is the smallest coefficient a decimal cannot hold.
	twoTo96 = "79228162514264337593543950336"
)

var (
	errBadInterval = errors.New("invalid interval")
	errNotDecimal  = errors.New("it is not a decimal number")
)

// isKindTag reports whether tag carries a date, time, timestamp, interval,
// bigdecimal or regex.
func isKindTag(tag uint64) bool {
	return tag == tagDate || tag >= tagTime && tag <= tagRegex
}

// appendWidened appends the plain cbor form of a kind only cbor.2 carries,
// given the content of its tag, as the server writes that form.
func appendWidened(b []byte, tag uint64, content []byte) ([]byte, error) {
	switch tag {
	case tagDate:
		return appendWidenedDate(b, content)
	case tagTime:
		return appendWidenedTime(b, content)
	case tagTimestamp:
		sec, nsec, err := ReadTimestamp(content)
		if err != nil {
			return nil, fmt.Errorf("cborwire: tag 190: %w", err)
		}
		b = appendInt(appendPair(b, tagDatetime), sec)
		return appendHead(b, majorUint, uint64(nsec)), nil //nolint:gosec // 0 to 999999999
	case tagInterval:
		months, days, micros, err := ReadInterval(content)
		if err != nil {
			return nil, fmt.Errorf("cborwire: tag 191: %w", err)
		}
		return AppendText(b, FormatInterval(months, days, micros)), nil
	case tagBigDecimal:
		return appendWidenedBigDecimal(b, content)
	default:
		return nil, fmt.Errorf("cborwire: a regex (CBOR tag 193) has no plain CBOR form: %w", constants.ErrNeedsCBOR2)
	}
}

// appendWidenedDate appends a date as the datetime at its midnight UTC. The
// server widens the last day, infinity, to its latest datetime instead.
func appendWidenedDate(b, content []byte) ([]byte, error) {
	days, err := ReadInt(content)
	if err != nil || days < MinDateDays || days > MaxDateDays {
		return nil, fmt.Errorf(errInvalidContent, tagDate, "an integer within the range of a date")
	}
	b = appendPair(b, tagDatetime)
	if days == MaxDateDays {
		return appendHead(appendInt(b, MaxTimestampSeconds), majorUint, uint64(MaxNanosecond)), nil
	}
	return appendHead(appendInt(b, days*secondsPerDay), majorUint, 0), nil
}

// appendWidenedTime appends a time of day as the duration since midnight, in
// the shortest form of tag 14: [], [seconds] or [seconds, nanoseconds].
func appendWidenedTime(b, content []byte) ([]byte, error) {
	nanos, err := ReadUint(content)
	if err != nil || nanos > uint64(NanosPerDay) {
		return nil, fmt.Errorf(errInvalidContent, tagTime, "an unsigned integer of at most 86400000000000")
	}
	seconds, rest := nanos/uint64(nanosPerSecond), nanos%uint64(nanosPerSecond)
	b = appendHead(b, majorTag, tagDuration)
	switch {
	case nanos == 0:
		return appendHead(b, majorArray, 0), nil
	case rest == 0:
		return appendHead(appendHead(b, majorArray, 1), majorUint, seconds), nil
	}
	b = appendHead(appendHead(b, majorArray, 2), majorUint, seconds)
	return appendHead(b, majorUint, rest), nil
}

// appendWidenedBigDecimal appends a bigdecimal as a decimal (tag 10) when it
// fits one, and as text otherwise.
func appendWidenedBigDecimal(b, content []byte) ([]byte, error) {
	text, err := ReadText(content)
	if err != nil {
		return nil, fmt.Errorf("cborwire: tag 192: %w", err)
	}
	canonical, err := CanonicalBigDecimal(text)
	if err != nil {
		return nil, fmt.Errorf("cborwire: tag 192: %w", err)
	}
	if decimal, ok := BigDecimalAsDecimal(canonical); ok {
		return AppendText(appendHead(b, majorTag, tagDecimalText), decimal), nil
	}
	return AppendText(b, canonical), nil
}

// AppendTag appends the head of tag number tag.
func AppendTag(b []byte, tag uint64) []byte {
	return appendHead(b, majorTag, tag)
}

// AppendArrayHead appends the head of a definite-length array of n items.
func AppendArrayHead(b []byte, n uint64) []byte {
	return appendHead(b, majorArray, n)
}

// AppendInt appends v as a CBOR integer.
func AppendInt(b []byte, v int64) []byte {
	return appendInt(b, v)
}

// AppendUint appends v as an unsigned CBOR integer.
func AppendUint(b []byte, v uint64) []byte {
	return appendHead(b, majorUint, v)
}

// AppendText appends s as a definite-length CBOR text string.
func AppendText(b []byte, s string) []byte {
	return append(appendHead(b, majorText, uint64(len(s))), s...)
}

// IsAbsent reports whether b is CBOR null or undefined, or NONE (tag 6)
// around one of them.
func IsAbsent(b []byte) bool {
	if len(b) == 2 && b[0] == majorTag<<5|tagNone {
		b = b[1:]
	}
	return len(b) == 1 && (b[0] == simpleNull || b[0] == simpleUndefined)
}

// SplitTag returns the number and the content of b when b is a tagged CBOR
// item. tagged is false for any other item.
func SplitTag(b []byte) (tag uint64, content []byte, tagged bool, err error) {
	r := &walker{in: b}
	major, info, arg, err := r.head()
	if err != nil {
		return 0, nil, false, err
	}
	if major != majorTag {
		return 0, nil, false, nil
	}
	if info == infoIndefinite {
		return 0, nil, false, errors.New("a tag cannot have an indefinite length")
	}
	return arg, b[r.pos:], true, nil
}

// ReadInt reads b, which must hold one CBOR integer within the range of an
// int64 and nothing else.
func ReadInt(b []byte) (int64, error) {
	r := &walker{in: b}
	v, err := r.readInt()
	if err != nil {
		return 0, err
	}
	return v, r.done()
}

// ReadUint reads b, which must hold one unsigned CBOR integer and nothing else.
func ReadUint(b []byte) (uint64, error) {
	r := &walker{in: b}
	major, info, arg, err := r.head()
	if err != nil {
		return 0, err
	}
	if major != majorUint || info == infoIndefinite {
		return 0, fmt.Errorf("want an unsigned integer, got major type %d", major)
	}
	return arg, r.done()
}

// ReadText reads b, which must hold one definite-length CBOR text string and
// nothing else.
func ReadText(b []byte) (string, error) {
	r := &walker{in: b}
	major, info, n, err := r.head()
	if err != nil {
		return "", err
	}
	if major != majorText || info == infoIndefinite {
		return "", fmt.Errorf("want a definite-length text string, got major type %d", major)
	}
	text, err := r.take(n)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(text) {
		return "", errors.New("text is not valid UTF-8")
	}
	return string(text), r.done()
}

// ReadInts reads b, which must hold one definite-length CBOR array of at most
// three integers within the range of an int64, and nothing else. It returns
// the integers and how many there were. A float or a tag is not an integer.
func ReadInts(b []byte) (ints [3]int64, n int, err error) {
	r := &walker{in: b}
	major, info, count, err := r.head()
	if err != nil {
		return ints, 0, err
	}
	if major != majorArray || info == infoIndefinite || count > uint64(len(ints)) {
		return ints, 0, errors.New("want a definite-length array of at most 3 integers")
	}
	n = int(count) //nolint:gosec // at most 3, checked above
	for i := range n {
		if ints[i], err = r.readInt(); err != nil {
			return ints, 0, err
		}
	}
	return ints, n, r.done()
}

// ReadTimestamp reads the content of a timestamp (tag 190): [seconds,
// nanoseconds] since 1970-01-01T00:00:00. As the server does, it takes
// nanoseconds up to the largest uint32 and carries the whole seconds in them,
// so a leap second becomes the first instant of the next second. Then the
// result must be within the range of a timestamp.
func ReadTimestamp(content []byte) (sec, nsec int64, err error) {
	if sec, nsec, err = readSecondsNanos(content); err != nil {
		return 0, 0, err
	}
	if carry := nsec / nanosPerSecond; sec <= math.MaxInt64-carry {
		sec += carry
	} else {
		sec = math.MaxInt64
	}
	nsec %= nanosPerSecond
	if sec < MinTimestampSeconds || sec > MaxTimestampSeconds {
		return 0, 0, fmt.Errorf("%d seconds since 1970 is outside the range of a timestamp", sec)
	}
	return sec, nsec, nil
}

// ReadDatetime reads the content of a datetime (tag 12): [seconds,
// nanoseconds] since 1970-01-01T00:00:00 UTC, as the server reads one. The
// nanoseconds are below a second, except in a leap second, the last second
// of a minute, where they are below two seconds; they are not carried. The
// result must be within the range of a datetime, the same as a timestamp's.
func ReadDatetime(content []byte) (sec, nsec int64, err error) {
	if sec, nsec, err = readSecondsNanos(content); err != nil {
		return 0, 0, err
	}
	switch {
	case nsec >= 2*nanosPerSecond, nsec >= nanosPerSecond && (sec%60+60)%60 != 59:
		return 0, 0, fmt.Errorf("%d nanoseconds are a second or more, outside a leap second", nsec)
	case sec < MinTimestampSeconds || sec > MaxTimestampSeconds:
		return 0, 0, fmt.Errorf("%d seconds since 1970 is outside the range of a datetime", sec)
	}
	return sec, nsec, nil
}

// readSecondsNanos reads [seconds, nanoseconds], exactly two integers, with
// the nanoseconds within a uint32, as the server reads tags 12 and 190.
func readSecondsNanos(content []byte) (sec, nsec int64, err error) {
	ints, n, err := ReadInts(content)
	if err != nil {
		return 0, 0, err
	}
	if n != 2 {
		return 0, 0, fmt.Errorf("want an array of 2 integers, got %d", n)
	}
	if ints[1] < 0 || ints[1] > math.MaxUint32 {
		return 0, 0, fmt.Errorf("nanoseconds %d are outside the range of a uint32", ints[1])
	}
	return ints[0], ints[1], nil
}

// ReadDuration reads the content of a duration (tag 14), [seconds,
// nanoseconds] where either may be left out, as nanoseconds. Nanoseconds of
// a second or more carry into the seconds, as on the server. The duration
// must fit an int64 of nanoseconds.
func ReadDuration(content []byte) (int64, error) {
	ints, n, err := ReadInts(content)
	if err != nil {
		return 0, err
	}
	if n > 2 || ints[0] < 0 || ints[1] < 0 {
		return 0, errors.New("want an array of at most 2 unsigned integers")
	}
	total, ok := mulExact(ints[0], nanosPerSecond)
	if ok {
		total, ok = addExact(total, ints[1])
	}
	if !ok {
		return 0, errors.New("duration does not fit an int64 of nanoseconds")
	}
	return total, nil
}

// ReadInterval reads the content of an interval (tag 191): [months, days,
// microseconds], each within its width.
func ReadInterval(content []byte) (months, days int32, micros int64, err error) {
	ints, n, err := ReadInts(content)
	if err != nil {
		return 0, 0, 0, err
	}
	if n != 3 {
		return 0, 0, 0, fmt.Errorf("want an array of 3 integers, got %d", n)
	}
	months, okMonths := toInt32(ints[0])
	days, okDays := toInt32(ints[1])
	if !okMonths || !okDays {
		return 0, 0, 0, errors.New("interval months and days must fit 32 bits")
	}
	return months, days, ints[2], nil
}

// toInt32 converts v to an int32, reporting false when it does not fit. The
// check sits beside the conversion so code scanning sees it is bounded.
func toInt32(v int64) (int32, bool) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, false
	}
	return int32(v), true
}

// readInt reads one CBOR integer within the range of an int64.
func (w *walker) readInt() (int64, error) {
	major, info, arg, err := w.head()
	if err != nil {
		return 0, err
	}
	switch {
	case info == infoIndefinite || major != majorUint && major != majorNegInt:
		return 0, fmt.Errorf("want an integer, got major type %d", major)
	case arg > math.MaxInt64:
		return 0, errors.New("integer outside the range of an int64")
	case major == majorNegInt:
		return -1 - int64(arg), nil //nolint:gosec // arg <= math.MaxInt64, checked above
	}
	return int64(arg), nil //nolint:gosec // arg <= math.MaxInt64, checked above
}

// done fails when anything is left after the item read.
func (w *walker) done() error {
	if w.pos != len(w.in) {
		return errors.New("unexpected data after the item")
	}
	return nil
}

// FormatInterval formats an interval as the server displays it, in ISO 8601
// with the years folded out of the months, such as "P1Y2M3DT4H5M6.5S", and
// "PT0S" for zero. Each time part carries the sign of the microseconds, even
// when its whole number is zero, as in "PT-0.5S".
func FormatInterval(months, days int32, micros int64) string {
	b := make([]byte, 0, 48)
	b = append(b, 'P')
	b = appendIntervalPart(b, int64(months/12), 'Y')
	b = appendIntervalPart(b, int64(months%12), 'M')
	b = appendIntervalPart(b, int64(days), 'D')
	if micros != 0 {
		sign := ""
		magnitude := uint64(micros) //nolint:gosec // negated below for a negative value
		if micros < 0 {
			sign = "-"
			magnitude = -magnitude // also right for math.MinInt64
		}
		b = append(b, 'T')
		if hours := magnitude / microsPerHour; hours != 0 {
			b = append(strconv.AppendUint(append(b, sign...), hours, 10), 'H')
		}
		if minutes := magnitude % microsPerHour / microsPerMinute; minutes != 0 {
			b = append(strconv.AppendUint(append(b, sign...), minutes, 10), 'M')
		}
		if rest := magnitude % microsPerMinute; rest != 0 {
			b = strconv.AppendUint(append(b, sign...), rest/microsPerSecond, 10)
			if frac := rest % microsPerSecond; frac != 0 {
				digits := strconv.FormatUint(frac+microsPerSecond, 10)[1:] // padded to six digits
				b = append(append(b, '.'), strings.TrimRight(digits, "0")...)
			}
			b = append(b, 'S')
		}
	}
	if len(b) == 1 {
		return "PT0S"
	}
	return string(b)
}

func appendIntervalPart(b []byte, n int64, unit byte) []byte {
	if n == 0 {
		return b
	}
	return append(strconv.AppendInt(b, n, 10), unit)
}

// ParseInterval parses an interval in the form FormatInterval writes: the
// designators Y, M and D, then T and the designators H, M and S, each
// optional, but at least one present. Each number may carry its own sign, and
// the seconds may have up to six digits after the point.
func ParseInterval(s string) (months, days int32, micros int64, err error) {
	if len(s) < 3 || s[0] != 'P' {
		return 0, 0, 0, fmt.Errorf("%w %q", errBadInterval, s)
	}
	var totalMonths, totalDays int64
	i, parts := 1, 0
	for _, unit := range []byte{'Y', 'M', 'D'} {
		p, next, found := readIntervalPart(s, i, unit, false)
		if !found {
			continue
		}
		i, parts = next, parts+1
		ok := true
		switch unit {
		case 'Y':
			totalMonths, ok = mulExact(p.whole, 12)
		case 'M':
			totalMonths, ok = addExact(totalMonths, p.whole)
		default:
			totalDays = p.whole
		}
		if !ok {
			return 0, 0, 0, fmt.Errorf("%w %q: out of range", errBadInterval, s)
		}
	}
	if i < len(s) && s[i] == 'T' {
		if micros, i, err = parseIntervalTime(s, i+1); err != nil {
			return 0, 0, 0, err
		}
		parts++
	}
	if i != len(s) || parts == 0 {
		return 0, 0, 0, fmt.Errorf("%w %q", errBadInterval, s)
	}
	months, okMonths := toInt32(totalMonths)
	days, okDays := toInt32(totalDays)
	if !okMonths || !okDays {
		return 0, 0, 0, fmt.Errorf("%w %q: months and days must fit 32 bits", errBadInterval, s)
	}
	return months, days, micros, nil
}

// parseIntervalTime parses the hours, minutes and seconds after the T of an
// interval, from s[i:], into microseconds.
func parseIntervalTime(s string, i int) (micros int64, next int, err error) {
	parts := 0
	for _, unit := range []struct {
		designator byte
		micros     int64
	}{{'H', microsPerHour}, {'M', microsPerMinute}, {'S', microsPerSecond}} {
		p, after, found := readIntervalPart(s, i, unit.designator, unit.designator == 'S')
		if !found {
			continue
		}
		i, parts = after, parts+1
		value, ok := mulExact(p.whole, unit.micros)
		if ok {
			value, ok = addExact(value, p.frac)
		}
		if ok {
			micros, ok = addExact(micros, value)
		}
		if !ok {
			return 0, 0, fmt.Errorf("%w %q: out of range", errBadInterval, s)
		}
	}
	if parts == 0 {
		return 0, 0, fmt.Errorf("%w %q", errBadInterval, s)
	}
	return micros, i, nil
}

// intervalPart is one signed number of an interval: whole units, and for
// seconds the microseconds after the point, both carrying the sign.
type intervalPart struct {
	whole, frac int64
}

// readIntervalPart reads a number and the designator unit from s[i:]. found
// is false when s[i:] does not start with such a part.
func readIntervalPart(s string, i int, unit byte, fraction bool) (p intervalPart, next int, found bool) {
	negative := i < len(s) && s[i] == '-'
	if negative {
		i++
	}
	end := scanDigits(s, i)
	whole, err := strconv.ParseInt(s[i:end], 10, 64)
	if err != nil {
		return p, 0, false
	}
	i = end
	var frac int64
	if fraction && i < len(s) && s[i] == '.' {
		if frac, i, found = readMicros(s, i+1); !found {
			return p, 0, false
		}
	}
	if i >= len(s) || s[i] != unit {
		return p, 0, false
	}
	if negative {
		whole, frac = -whole, -frac
	}
	return intervalPart{whole: whole, frac: frac}, i + 1, true
}

// readMicros reads the one to six digits after the point of a number of
// seconds, from s[i:], as microseconds.
func readMicros(s string, i int) (micros int64, next int, ok bool) {
	end := scanDigits(s, i)
	digits := end - i
	if digits == 0 || digits > 6 {
		return 0, 0, false
	}
	micros, _ = strconv.ParseInt(s[i:end]+strings.Repeat("0", 6-digits), 10, 64)
	return micros, end, true
}

// scanDigits returns the index of the first byte of s at or after i that is
// not a digit.
func scanDigits(s string, i int) int {
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// mulExact returns a*b, and false when that overflows an int64.
func mulExact(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	c := a * b
	if c/b != a || a == -1 && b == math.MinInt64 || b == -1 && a == math.MinInt64 {
		return 0, false
	}
	return c, true
}

// addExact returns a+b, and false when that overflows an int64.
func addExact(a, b int64) (int64, bool) {
	c := a + b
	if (c > a) != (b > 0) {
		return 0, false
	}
	return c, true
}

// CanonicalBigDecimal parses a bigdecimal as the server does and returns it
// in the positional notation the server writes: no leading zeros, a minus sign
// only on a value other than zero, and the scale as written. An exponent is
// folded in, so "1.5e3" is "1500" and "1e-3" is "0.001". It refuses what the
// server refuses: an empty string, whitespace, NaN, the infinities, and a
// value past the server's digit, exponent or scale limits.
func CanonicalBigDecimal(s string) (string, error) {
	negative, coefficient, scale, err := splitBigDecimal(s)
	if err != nil {
		return "", err
	}
	return formatPositional(negative, coefficient, scale), nil
}

// splitBigDecimal parses s into its sign, its coefficient (without leading
// zeros; empty for zero) and its scale. It checks the server's limits before
// anything their size is built: digits are counted without the leading zeros.
func splitBigDecimal(s string) (negative bool, coefficient string, scale int, err error) {
	text, err := parseBigDecimal(s)
	if err == nil {
		coefficient = strings.TrimLeft(text.intDigits+text.fracDigits, "0")
		scale = len(text.fracDigits) - text.exponent
		switch {
		case len(coefficient) > bigDecimalMaxDigits,
			coefficient != "" && scale < 0 && len(coefficient)-scale > bigDecimalMaxDigits:
			err = errors.New("it has more digits than the supported maximum")
		case scale > bigDecimalMaxScale:
			err = errors.New("its scale is past the supported maximum")
		}
	}
	if err != nil {
		return false, "", 0, fmt.Errorf("invalid bigdecimal %q: %w", s, err)
	}
	return text.negative, coefficient, scale, nil
}

// bigDecimalText is a bigdecimal as written: a sign, the digits before and
// after the point, and an exponent.
type bigDecimalText struct {
	negative              bool
	intDigits, fracDigits string
	exponent              int
}

// parseBigDecimal reads a bigdecimal in the server's grammar: an optional
// sign, digits with an optional point, and an optional exponent.
func parseBigDecimal(s string) (text bigDecimalText, err error) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		text.negative = s[i] == '-'
		i++
	}
	end := scanDigits(s, i)
	text.intDigits, i = s[i:end], end
	if i < len(s) && s[i] == '.' {
		end = scanDigits(s, i+1)
		text.fracDigits, i = s[i+1:end], end
	}
	if text.intDigits == "" && text.fracDigits == "" {
		return text, errNotDecimal
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		if text.exponent, i, err = readBigDecimalExponent(s, i+1); err != nil {
			return text, err
		}
	}
	if i != len(s) {
		return text, errNotDecimal
	}
	return text, nil
}

// readBigDecimalExponent reads the signed exponent of a bigdecimal from s[i:].
func readBigDecimalExponent(s string, i int) (exponent, next int, err error) {
	negative := i < len(s) && s[i] == '-'
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	i = scanDigits(s, i)
	if i == start {
		return 0, 0, errors.New("its exponent has no digits")
	}
	if digits := strings.TrimLeft(s[start:i], "0"); digits != "" {
		if len(digits) > bigDecimalMaxExponentDigits {
			return 0, 0, errors.New("its exponent is out of range")
		}
		if exponent, err = strconv.Atoi(digits); err != nil || exponent > bigDecimalMaxExponent {
			return 0, 0, errors.New("its exponent is out of range")
		}
	}
	if negative {
		exponent = -exponent
	}
	return exponent, i, nil
}

// formatPositional writes a coefficient (without leading zeros; empty for
// zero) and a scale in positional notation, with exactly scale digits after
// the point, or none for a scale of zero or less.
func formatPositional(negative bool, coefficient string, scale int) string {
	if coefficient == "" {
		if scale <= 0 {
			return "0"
		}
		return "0." + strings.Repeat("0", scale)
	}
	if scale < 0 {
		coefficient += strings.Repeat("0", -scale)
		scale = 0
	}
	var b strings.Builder
	if negative {
		b.WriteByte('-')
	}
	switch n := len(coefficient); {
	case scale == 0:
		b.WriteString(coefficient)
	case n > scale:
		b.WriteString(coefficient[:n-scale])
		b.WriteByte('.')
		b.WriteString(coefficient[n-scale:])
	default:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", scale-n))
		b.WriteString(coefficient)
	}
	return b.String()
}

// BigDecimalAsDecimal returns a bigdecimal, given in the form
// CanonicalBigDecimal writes, as the decimal the server writes for it when it
// fits one: with no trailing zeros after the point. ok is false when it does
// not fit. Once all trailing zeros are stripped from the coefficient, the
// value is c·10^e, and it fits only when -28 <= e <= 28 and c·10^max(e,0) is
// below 2^96. Zero fits, as "0".
func BigDecimalAsDecimal(canonical string) (decimal string, ok bool) {
	negative := strings.HasPrefix(canonical, "-")
	intDigits, fracDigits, _ := strings.Cut(strings.TrimPrefix(canonical, "-"), ".")
	coefficient := strings.TrimLeft(intDigits+fracDigits, "0")
	stripped := strings.TrimRight(coefficient, "0")
	exponent := len(coefficient) - len(stripped) - len(fracDigits)
	if stripped == "" {
		return "0", true
	}
	if exponent < -28 || exponent > 28 {
		return "", false
	}
	magnitude := stripped
	if exponent > 0 {
		magnitude += strings.Repeat("0", exponent)
	}
	if len(magnitude) > len(twoTo96) || len(magnitude) == len(twoTo96) && magnitude >= twoTo96 {
		return "", false
	}
	return formatPositional(negative, stripped, -exponent), true
}

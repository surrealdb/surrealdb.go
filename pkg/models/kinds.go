package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// CBOR tags for SurrealDB 4.0's date, time, timestamp, interval, bigdecimal
// and regex kinds, which it shares with PostgreSQL and MySQL.
//
// Like the sized numbers, they exist only in the cbor.2 dialect. Over plain
// cbor the server sends these kinds widened, refuses these tags, and cannot
// send a regex at all: see each type for its widened form.
//
// See https://github.com/surrealdb/surrealdb-protocol/blob/v0.15.0/spec/cbor-tags.yaml.
const (
	TagPlainDate  uint64 = 100
	TagPlainTime  uint64 = 189
	TagTimestamp  uint64 = 190
	TagInterval   uint64 = 191
	TagBigDecimal uint64 = 192
	TagRegex      uint64 = 193
)

// IsCBOR2OnlyTag reports whether tag exists only in cbor.2: a sized number
// (180 to 188), or a date, time, timestamp, interval, bigdecimal or regex
// (100 and 189 to 193).
func IsCBOR2OnlyTag(tag uint64) bool {
	return tag == TagPlainDate || tag >= TagI8 && tag <= TagRegex
}

// isKindTag reports whether tag carries a date, time, timestamp, interval,
// bigdecimal or regex.
func isKindTag(tag uint64) bool {
	return tag == TagPlainDate || tag >= TagPlainTime && tag <= TagRegex
}

const (
	textInfinity    = "infinity"
	textNegInfinity = "-infinity"
	textEpoch       = "epoch"

	secondsPerDay = 86400
	microsPerDay  = 86_400_000_000
	cborMajorText = 3
)

// kindItem is the CBOR item a 4.0 kind decodes from: its tag, if it has one,
// and the content of that tag, or the item itself when untagged.
type kindItem struct {
	tag     uint64
	tagged  bool
	content []byte
}

// readKindItem reads data, the item the kind of tag own, named into, decodes
// from. absent reports NULL, undefined or NONE, which leave a value as it is.
// Another kind only cbor.2 carries is first widened the way plain cbor
// carries it, so a value decodes into a type the same way in either dialect.
func readKindItem(data []byte, own uint64, into string) (item kindItem, absent bool, err error) {
	if cborwire.IsAbsent(data) {
		return item, true, nil
	}
	tag, content, tagged, err := cborwire.SplitTag(data)
	switch {
	case err != nil:
		return item, false, fmt.Errorf("cannot decode %s: %w", into, err)
	case !tagged:
		return kindItem{content: data}, false, nil
	case tag == own || !isKindTag(tag):
		return kindItem{tag: tag, tagged: true, content: content}, false, nil
	case tag == TagRegex:
		return item, false, fmt.Errorf("cannot decode a regex (CBOR tag %d) into %s", tag, into)
	}
	widened, wideErr := cborwire.Normalize(data, cborwire.Plain)
	if wideErr != nil {
		return item, false, fmt.Errorf("cannot decode %s: %w", into, wideErr)
	}
	return readKindItem(widened, own, into)
}

// mismatch is the error for an item a kind does not decode from.
func (item kindItem) mismatch(into string) error {
	if item.tagged {
		return fmt.Errorf("cannot decode CBOR tag %d into %s", item.tag, into)
	}
	return fmt.Errorf("cannot decode CBOR major type %d into %s", item.content[0]>>5, into)
}

// parseInfinity reads "infinity", "+infinity" or "-infinity" in any letter
// case, as the server does. sign is +1 or -1, and 0 for any other text.
func parseInfinity(text string) (sign int) {
	switch {
	case strings.EqualFold(text, textInfinity), strings.EqualFold(text, "+"+textInfinity):
		return 1
	case strings.EqualFold(text, textNegInfinity):
		return -1
	}
	return 0
}

// daysOfCivil returns the days since 1970-01-01 of a date in the proleptic
// Gregorian calendar, where year 0 is 1 BCE, as time.Date counts them. ok is
// false for a day or month that does not exist, such as February 30, rather
// than rolling it over, and for a date outside the range of the date kind.
func daysOfCivil(year int, month time.Month, day int) (days int64, ok bool) {
	if year < -262143 || year > 262142 {
		return 0, false
	}
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if y, m, d := t.Date(); y != year || m != month || d != day {
		return 0, false
	}
	return t.Unix() / secondsPerDay, true
}

// civilOfDays returns the date days after 1970-01-01.
func civilOfDays(days int64) (year int, month time.Month, day int) {
	return time.Unix(days*secondsPerDay, 0).UTC().Date()
}

// parseCivilDate parses a date into days since 1970-01-01 as the server does,
// through chrono's NaiveDate::from_str: a year, '-', a month, '-' and a day,
// with optional whitespace before each number and after the day. A year
// without a sign has one to four digits, so a later year needs a sign; a
// year with one may have any number. The month and the day have one or two
// digits. The date must exist, and be within the range of the date kind; its
// first and last days are allowed.
func parseCivilDate(s string) (int64, error) {
	year, rest, ok := readYear(trimSpaceLeft(s))
	var month, day int
	if ok {
		month, rest, ok = readDateField(rest)
	}
	if ok {
		day, rest, ok = readDateField(rest)
	}
	if ok && trimSpaceLeft(rest) == "" {
		if days, valid := daysOfCivil(year, time.Month(month), day); valid {
			return days, nil
		}
	}
	return 0, fmt.Errorf("invalid date %q", s)
}

// readYear reads a year as chrono does: with a sign, one or more digits;
// without one, one to four.
func readYear(s string) (year int, rest string, ok bool) {
	sign, maxDigits := 1, 4
	if s != "" && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		s, maxDigits = s[1:], len(s)
	}
	end := min(scanDigits(s, 0), maxDigits)
	digits := strings.TrimLeft(s[:end], "0")
	if end == 0 || len(digits) > 7 { // past 7 digits a year is out of range anyway
		return 0, "", false
	}
	year, _ = strconv.Atoi("0" + digits)
	return sign * year, s[end:], true
}

// readDateField reads the month or the day after a year as chrono does:
// optional whitespace, '-', optional whitespace, and one or two digits.
func readDateField(s string) (field int, rest string, ok bool) {
	s = trimSpaceLeft(s)
	if !strings.HasPrefix(s, "-") {
		return 0, "", false
	}
	s = trimSpaceLeft(s[1:])
	end := min(scanDigits(s, 0), 2)
	if end == 0 {
		return 0, "", false
	}
	field, _ = strconv.Atoi(s[:end])
	return field, s[end:], true
}

func trimSpaceLeft(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

// appendCivilDate appends the date days after 1970-01-01 as YYYY-MM-DD, with a
// sign and at least four digits for a year outside 0000 to 9999, as the
// server writes one. Go's own 2006 layout leaves out the sign of a year of
// 10000 or more.
func appendCivilDate(b []byte, days int64) []byte {
	year, month, day := civilOfDays(days)
	switch {
	case year > 9999:
		b = append(b, '+')
	case year < 0:
		b = append(b, '-')
		year = -year
	}
	b = appendPadded(b, int64(year), 4)
	b = appendPadded(append(b, '-'), int64(month), 2)
	return appendPadded(append(b, '-'), int64(day), 2)
}

// parseTimeOfDay parses H:MM, H:MM:SS or H:MM:SS.fraction into nanoseconds
// since midnight, as the server reads a time: one or two digits in each field
// and up to nine in the fraction, up to and including 24:00:00.
func parseTimeOfDay(s string) (int64, error) {
	invalid := fmt.Errorf("invalid time %q", s)
	hour, i, ok := readClockField(s, 0)
	if !ok || i >= len(s) || s[i] != ':' {
		return 0, invalid
	}
	minute, i, ok := readClockField(s, i+1)
	if !ok {
		return 0, invalid
	}
	var seconds int64
	if i < len(s) && s[i] == ':' {
		if seconds, i, ok = readSeconds(s, i+1); !ok {
			return 0, invalid
		}
	}
	total := (hour*3600+minute*60)*nsPerSecond + seconds
	if i != len(s) || hour > 24 || minute > 59 || total > nsPerDay {
		return 0, invalid
	}
	return total, nil
}

// readSeconds reads SS or SS.fraction from s[i:], with up to nine digits in
// the fraction, as nanoseconds.
func readSeconds(s string, i int) (nanos int64, next int, ok bool) {
	second, i, ok := readClockField(s, i)
	if !ok || second > 59 {
		return 0, 0, false
	}
	nanos = second * nsPerSecond
	if i < len(s) && s[i] == '.' {
		end := scanDigits(s, i+1)
		digits := end - i - 1
		if digits == 0 || digits > 9 {
			return 0, 0, false
		}
		fraction, _ := strconv.ParseInt(s[i+1:end]+strings.Repeat("0", 9-digits), 10, 64)
		nanos, i = nanos+fraction, end
	}
	return nanos, i, true
}

// readClockField reads a field of one or two digits from s[i:].
func readClockField(s string, i int) (field int64, next int, ok bool) {
	end := scanDigits(s, i)
	if end == i || end-i > 2 {
		return 0, 0, false
	}
	field, _ = strconv.ParseInt(s[i:end], 10, 64)
	return field, end, true
}

// appendTimeOfDay appends nanoseconds since midnight as HH:MM:SS, followed by
// the fraction the server writes: none for a whole second, otherwise
// milliseconds, microseconds or nanoseconds, whichever is the shortest exact.
func appendTimeOfDay(b []byte, nanos int64) []byte {
	seconds, rest := nanos/nsPerSecond, nanos%nsPerSecond
	b = appendPadded(b, seconds/3600, 2)
	b = appendPadded(append(b, ':'), seconds/60%60, 2)
	b = appendPadded(append(b, ':'), seconds%60, 2)
	switch {
	case rest == 0:
		return b
	case rest%nsPerMilliSecond == 0:
		return appendPadded(append(b, '.'), rest/nsPerMilliSecond, 3)
	case rest%nsPerMicroSecond == 0:
		return appendPadded(append(b, '.'), rest/nsPerMicroSecond, 6)
	}
	return appendPadded(append(b, '.'), rest, 9)
}

// appendPadded appends n, which is not negative, with leading zeros up to
// width digits.
func appendPadded(b []byte, n int64, width int) []byte {
	digits := strconv.FormatInt(n, 10)
	for i := len(digits); i < width; i++ {
		b = append(b, '0')
	}
	return append(b, digits...)
}

// scanDigits returns the index of the first byte of s at or after i that is
// not a digit.
func scanDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

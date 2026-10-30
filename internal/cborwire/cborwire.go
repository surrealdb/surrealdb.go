// Package cborwire rewrites encoded CBOR into the forms SurrealDB accepts.
//
// fxamacker/cbor, which both of the SDK's encoders use, can only write a
// time.Time as tag 0 (RFC 3339 text) or tag 1, and the SDK's UUIDString and
// CustomDurationString types are registered as the text tags 9 and 13.
// SurrealDB 4.0 refuses tags 0, 9 and 13, and refuses an integer outside the
// range of an i64, which SurrealDB 3.x silently wrapped. Every SurrealDB
// release since 1.5 accepts the compact forms, so Normalize rewrites:
//
//   - tag 0 (a datetime as text) to tag 12 [seconds, nanoseconds];
//   - tag 9 (a UUID as text) to tag 37 (its 16 bytes);
//   - tag 13 (a duration as text) to tag 14 [seconds, nanoseconds];
//   - an integer outside the range of an i64 to tag 10 (a decimal as text).
//
// The tag numbers are listed in spec/cbor-tags.yaml of surrealdb-protocol.
package cborwire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
)

const (
	majorUint   = 0
	majorNegInt = 1
	majorBytes  = 2
	majorText   = 3
	majorArray  = 4
	majorMap    = 5
	majorTag    = 6
	majorSimple = 7

	infoIndefinite = 31
	breakByte      = 0xff

	tagDatetimeText   = 0
	tagUUIDText       = 9
	tagDecimalText    = 10
	tagDatetime       = 12
	tagDurationText   = 13
	tagDuration       = 14
	tagUUID           = 37
	maxNestingDepth   = 1 << 12
	nanosPerSecond    = int64(time.Second)
	errInvalidContent = "cborwire: tag %d must contain %s"

	// Tags 180 to 188 are the sized numbers of cbor.2, such as 186 for a u64.
	tagSizedNumberFirst = 180
	tagSizedNumberLast  = 188
)

var errTruncated = errors.New("cborwire: truncated CBOR")

// Normalize returns b with the forms SurrealDB refuses rewritten as described
// in the package documentation. It returns b itself when nothing needs
// rewriting.
func Normalize(b []byte) ([]byte, error) {
	w := &walker{in: b}
	if err := w.item(0); err != nil {
		return nil, err
	}
	if w.pos != len(b) {
		// More than one top-level item: a stream. Rewrite each in turn.
		for w.pos < len(b) {
			if err := w.item(0); err != nil {
				return nil, err
			}
		}
	}
	if w.out == nil {
		return b, nil
	}
	return append(w.out, b[w.copied:]...), nil
}

// walker reads in, copying it lazily to out from the first rewrite on.
type walker struct {
	in     []byte
	pos    int
	out    []byte
	copied int // in[:copied] is already in out
	leaf   int // above 0 inside a tag whose content must not be rewritten
}

// begin copies in[copied:from] to out. A rewrite then appends its replacement
// for in[from:pos] to out and calls end.
func (w *walker) begin(from int) {
	if w.out == nil {
		w.out = make([]byte, 0, len(w.in)+len(w.in)/8)
	}
	w.out = append(w.out, w.in[w.copied:from]...)
}

// end marks in[:pos] as copied, after a rewrite.
func (w *walker) end() {
	w.copied = w.pos
}

// head reads an item's initial byte and argument.
func (w *walker) head() (major, info byte, arg uint64, err error) {
	if w.pos >= len(w.in) {
		return 0, 0, 0, errTruncated
	}
	b := w.in[w.pos]
	w.pos++
	major, info = b>>5, b&0x1f
	switch {
	case info < 24:
		return major, info, uint64(info), nil
	case info == 24, info == 25, info == 26, info == 27:
		n := 1 << (info - 24)
		if w.pos+n > len(w.in) {
			return 0, 0, 0, errTruncated
		}
		for _, c := range w.in[w.pos : w.pos+n] {
			arg = arg<<8 | uint64(c)
		}
		w.pos += n
		return major, info, arg, nil
	case info == infoIndefinite:
		return major, info, 0, nil
	}
	return 0, 0, 0, fmt.Errorf("cborwire: invalid additional information %d", info)
}

func (w *walker) item(depth int) error {
	if depth > maxNestingDepth {
		return errors.New("cborwire: CBOR nested too deeply")
	}
	start := w.pos
	major, info, arg, err := w.head()
	if err != nil {
		return err
	}
	if info == infoIndefinite {
		switch major {
		case majorBytes, majorText:
			return w.chunks(major)
		case majorArray, majorMap:
			return w.indefinite(depth)
		}
		return fmt.Errorf("cborwire: invalid indefinite length for major type %d", major)
	}

	switch major {
	case majorUint, majorNegInt:
		if arg > math.MaxInt64 && w.leaf == 0 {
			w.begin(start)
			w.out = appendBigIntDecimal(w.out, major, arg)
			w.end()
		}
	case majorBytes, majorText:
		return w.skip(arg)
	case majorArray:
		return w.items(arg, depth)
	case majorMap:
		if arg > math.MaxUint64/2 {
			return errTruncated
		}
		return w.items(2*arg, depth)
	case majorTag:
		return w.tag(start, arg, depth)
	}
	// majorSimple: a simple value or a float, read whole by head.
	return nil
}

// tag reads the content of a tag, rewriting the tag if it needs it.
func (w *walker) tag(start int, tag uint64, depth int) error {
	switch {
	case w.leaf > 0:
	case tag == tagDatetimeText, tag == tagUUIDText, tag == tagDurationText:
		return w.textTag(start, tag)
	case tag >= tagSizedNumberFirst && tag <= tagSizedNumberLast:
		// The content of a sized number must stay as it is, such as the u64
		// in tag 186. Walk it without rewriting anything.
		w.leaf++
		err := w.item(depth + 1)
		w.leaf--
		return err
	}
	return w.item(depth + 1)
}

// items reads n items one level down.
func (w *walker) items(n uint64, depth int) error {
	for i := uint64(0); i < n; i++ {
		if err := w.item(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

// indefinite reads the items of an indefinite-length array or map.
func (w *walker) indefinite(depth int) error {
	for {
		if w.pos >= len(w.in) {
			return errTruncated
		}
		if w.in[w.pos] == breakByte {
			w.pos++
			return nil
		}
		if err := w.item(depth + 1); err != nil {
			return err
		}
	}
}

// take returns the next n bytes.
func (w *walker) take(n uint64) ([]byte, error) {
	if n > uint64(len(w.in)-w.pos) { //nolint:gosec // pos never passes len(in)
		return nil, errTruncated
	}
	b := w.in[w.pos : w.pos+int(n)] //nolint:gosec // n fits: it is at most len(in)-pos
	w.pos += len(b)
	return b, nil
}

func (w *walker) skip(n uint64) error {
	_, err := w.take(n)
	return err
}

// chunks skips the definite-length chunks of an indefinite-length string.
func (w *walker) chunks(major byte) error {
	for {
		if w.pos >= len(w.in) {
			return errTruncated
		}
		if w.in[w.pos] == breakByte {
			w.pos++
			return nil
		}
		m, info, n, err := w.head()
		if err != nil {
			return err
		}
		if m != major || info == infoIndefinite {
			return fmt.Errorf("cborwire: invalid chunk in an indefinite-length string")
		}
		if err := w.skip(n); err != nil {
			return err
		}
	}
}

// textTag rewrites a tag whose content is text into its compact form.
func (w *walker) textTag(start int, tag uint64) error {
	m, info, n, err := w.head()
	if err != nil {
		return err
	}
	if m != majorText || info == infoIndefinite {
		return fmt.Errorf(errInvalidContent, tag, "a definite-length text string")
	}
	b, err := w.take(n)
	if err != nil {
		return err
	}

	switch tag {
	case tagDatetimeText:
		t, err := parseDatetime(string(b))
		if err != nil {
			return fmt.Errorf("cborwire: invalid datetime: %w", err)
		}
		w.begin(start)
		w.out = appendPair(w.out, tagDatetime)
		w.out = appendInt(w.out, t.Unix())
		w.out = appendHead(w.out, majorUint, uint64(t.Nanosecond())) //nolint:gosec // 0 to 999999999
	case tagUUIDText:
		var u uuid.UUID
		if err := u.UnmarshalText(b); err != nil {
			return fmt.Errorf("cborwire: invalid UUID: %w", err)
		}
		w.begin(start)
		w.out = appendHead(w.out, majorTag, tagUUID)
		w.out = appendHead(w.out, majorBytes, uint64(len(u)))
		w.out = append(w.out, u[:]...)
	case tagDurationText:
		seconds, nanos, err := ParseDuration(b)
		if err != nil {
			return fmt.Errorf("cborwire: %w", err)
		}
		w.begin(start)
		w.out = appendPair(w.out, tagDuration)
		w.out = appendHead(w.out, majorUint, seconds)
		w.out = appendHead(w.out, majorUint, uint64(nanos))
	}
	w.end()
	return nil
}

// parseDatetime parses an RFC 3339 datetime. Go writes a time.Time outside the
// years 0000 to 9999 with more digits or a minus sign, as in
// "10000-01-01T00:00:00Z" or "-0001-01-01T00:00:00Z", and time.Parse refuses
// those. So the rest is parsed with a stand-in year that has the same leap-year
// status, then moved to the real year.
func parseDatetime(text string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, text)
	if err == nil || len(text) < 2 {
		return t, err
	}
	i := strings.IndexByte(text[1:], '-') + 1 // the first byte can be a minus sign
	if i < 1 {
		return t, err
	}
	year, yearErr := strconv.Atoi(text[:i])
	if yearErr != nil {
		return t, err
	}
	standIn := 2001
	if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		standIn = 2000
	}
	moved, standInErr := time.Parse(time.RFC3339Nano, strconv.Itoa(standIn)+text[i:])
	if standInErr != nil {
		return t, err
	}
	return moved.AddDate(year-standIn, 0, 0), nil
}

// appendPair appends the head of tag [_, _], such as [seconds, nanoseconds].
func appendPair(b []byte, tag uint64) []byte {
	return appendHead(appendHead(b, majorTag, tag), majorArray, 2)
}

// appendBigIntDecimal appends an integer outside the range of an i64 as tag 10.
func appendBigIntDecimal(b []byte, major byte, arg uint64) []byte {
	var buf [len("-18446744073709551616")]byte
	text := buf[:0]
	switch {
	case major == majorUint:
		text = strconv.AppendUint(text, arg, 10)
	case arg == math.MaxUint64:
		// A negative integer is -1 - arg; arg + 1 can overflow a uint64.
		text = append(text, "-18446744073709551616"...)
	default:
		text = strconv.AppendUint(append(text, '-'), arg+1, 10)
	}
	b = appendHead(b, majorTag, tagDecimalText)
	b = appendHead(b, majorText, uint64(len(text)))
	return append(b, text...)
}

func appendInt(b []byte, v int64) []byte {
	if v < 0 {
		return appendHead(b, majorNegInt, uint64(-1-v))
	}
	return appendHead(b, majorUint, uint64(v))
}

// appendHead appends the shortest head for major and arg.
func appendHead(b []byte, major byte, arg uint64) []byte {
	m := major << 5
	switch {
	case arg < 24:
		return append(b, m|byte(arg))
	case arg <= math.MaxUint8:
		return append(b, m|24, byte(arg))
	case arg <= math.MaxUint16:
		return binary.BigEndian.AppendUint16(append(b, m|25), uint16(arg))
	case arg <= math.MaxUint32:
		return binary.BigEndian.AppendUint32(append(b, m|26), uint32(arg))
	}
	return binary.BigEndian.AppendUint64(append(b, m|27), arg)
}

// Seconds in each duration unit from a year down to a second, and nanoseconds
// in each unit below a second.
var (
	durationUnits = []struct {
		name    string
		seconds uint64
	}{
		{"y", 365 * 24 * 3600}, {"w", 7 * 24 * 3600}, {"d", 24 * 3600}, {"h", 3600}, {"m", 60}, {"s", 1},
	}
	subsecondUnits = []struct {
		name  string
		nanos uint32
	}{
		{"ms", 1e6}, {"µs", 1e3}, {"ns", 1},
	}
)

var (
	errDurationTooLong = errors.New("its seconds must fit in 64 bits")
	errDurationUnit    = errors.New("unknown unit")
)

// ParseDuration parses a SurrealQL duration such as "1h30m" or "2w3d" into
// whole seconds and the nanoseconds left over. Like SurrealDB, it holds the
// seconds in a uint64, so it fails only for a duration longer than that.
func ParseDuration[T ~string | ~[]byte](duration T) (seconds uint64, nanos uint32, err error) {
	var sum durationSum
	for i := 0; i < len(duration); {
		value, unit, next, overflow, ok := durationPart(duration, i)
		if !ok {
			return 0, 0, fmt.Errorf("invalid duration %q", duration)
		}
		sum.tooLong = sum.tooLong || overflow
		if err := sum.add(value, string(duration[unit:next])); err != nil {
			return 0, 0, fmt.Errorf("invalid duration %q: %w %q", duration, err, duration[unit:next])
		}
		i = next
	}
	if sum.tooLong {
		return 0, 0, fmt.Errorf("duration %q is too long: %w", duration, errDurationTooLong)
	}
	return sum.seconds, uint32(sum.nanos), nil //nolint:gosec // below a second
}

// durationPart reads a number and a unit, such as "30m", from duration[i:].
// The unit is duration[unit:next]. overflow reports a number past a uint64.
func durationPart[T ~string | ~[]byte](duration T, i int) (value uint64, unit, next int, overflow, ok bool) {
	digits := i
	for ; i < len(duration) && duration[i] >= '0' && duration[i] <= '9'; i++ {
		hi, lo := bits.Mul64(value, 10)
		sum, carry := bits.Add64(lo, uint64(duration[i]-'0'), 0)
		overflow = overflow || hi != 0 || carry != 0
		value = sum
	}
	unit = i
	for i < len(duration) {
		if c := duration[i]; c >= 'a' && c <= 'z' {
			i++
		} else if c == 0xc2 && i+1 < len(duration) && duration[i+1] == 0xb5 { // µ
			i += 2
		} else {
			break
		}
	}
	return value, unit, i, overflow, unit > digits && i > unit
}

// durationSum adds up the parts of a duration.
type durationSum struct {
	seconds uint64
	nanos   uint64 // always below a second
	tooLong bool
}

func (s *durationSum) add(value uint64, unit string) error {
	unitSeconds, unitNanos := durationUnit(unit)
	var add uint64 // whole seconds to add
	switch {
	case unitSeconds != 0:
		hi, lo := bits.Mul64(value, unitSeconds)
		s.tooLong = s.tooLong || hi != 0
		add = lo
	case unitNanos != 0:
		perSecond := uint64(nanosPerSecond) / unitNanos
		add = value / perSecond
		s.nanos += value % perSecond * unitNanos
		if s.nanos >= uint64(nanosPerSecond) {
			s.nanos -= uint64(nanosPerSecond)
			add++ // cannot wrap: value / perSecond is far below MaxUint64
		}
	default:
		return errDurationUnit
	}
	var carry uint64
	s.seconds, carry = bits.Add64(s.seconds, add, 0)
	s.tooLong = s.tooLong || carry != 0
	return nil
}

// durationUnit returns the seconds in a unit from a year down to a second, or
// the nanoseconds in a unit below a second. It returns zeros for an unknown
// unit.
func durationUnit(name string) (seconds, nanos uint64) {
	if name == "us" {
		name = "µs"
	}
	for _, u := range durationUnits {
		if u.name == name {
			return u.seconds, 0
		}
	}
	for _, u := range subsecondUnits {
		if u.name == name {
			return 0, uint64(u.nanos)
		}
	}
	return 0, 0
}

// FormatDuration formats seconds and nanoseconds as a SurrealQL duration in
// its canonical form, such as "1h30m". A zero duration is "". nanos must be
// below a second.
func FormatDuration(seconds uint64, nanos uint32) string {
	var b []byte
	for _, u := range durationUnits {
		if n := seconds / u.seconds; n > 0 {
			b = append(strconv.AppendUint(b, n, 10), u.name...)
		}
		seconds %= u.seconds
	}
	for _, u := range subsecondUnits {
		if n := nanos / u.nanos; n > 0 {
			b = append(strconv.AppendUint(b, uint64(n), 10), u.name...)
		}
		nanos %= u.nanos
	}
	return string(b)
}

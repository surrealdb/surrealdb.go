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
	"regexp"
	"strconv"
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
}

// replace writes repl in place of in[from:w.pos].
func (w *walker) replace(from int, repl []byte) {
	if w.out == nil {
		w.out = make([]byte, 0, len(w.in)+16)
	}
	w.out = append(w.out, w.in[w.copied:from]...)
	w.out = append(w.out, repl...)
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
		if arg > math.MaxInt64 {
			w.replace(start, bigIntDecimal(major, arg))
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
		switch arg {
		case tagDatetimeText, tagUUIDText, tagDurationText:
			return w.textTag(start, arg)
		}
		return w.item(depth + 1)
	}
	// majorSimple: a simple value or a float, read whole by head.
	return nil
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
	text := string(b)

	var repl []byte
	switch tag {
	case tagDatetimeText:
		t, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return fmt.Errorf("cborwire: tag 0 contains an invalid datetime: %w", err)
		}
		repl = secondsAndNanos(tagDatetime, t.Unix(), int64(t.Nanosecond()))
	case tagUUIDText:
		u, err := uuid.FromString(text)
		if err != nil {
			return fmt.Errorf("cborwire: tag 9 contains an invalid UUID: %w", err)
		}
		repl = appendHead(appendHead(nil, majorTag, tagUUID), majorBytes, uint64(len(u)))
		repl = append(repl, u.Bytes()...)
	case tagDurationText:
		ns, err := ParseDuration(text)
		if err != nil {
			return fmt.Errorf("cborwire: tag 13 contains an invalid duration: %w", err)
		}
		repl = secondsAndNanos(tagDuration, ns/nanosPerSecond, ns%nanosPerSecond)
	}
	w.replace(start, repl)
	return nil
}

// secondsAndNanos encodes tag [seconds, nanoseconds].
func secondsAndNanos(tag uint64, seconds, nanos int64) []byte {
	b := appendHead(nil, majorTag, tag)
	b = appendHead(b, majorArray, 2)
	b = appendInt(b, seconds)
	return appendInt(b, nanos)
}

// bigIntDecimal encodes an integer outside the range of an i64 as tag 10.
func bigIntDecimal(major byte, arg uint64) []byte {
	var text string
	if major == majorUint {
		text = strconv.FormatUint(arg, 10)
	} else {
		// A negative integer is -1 - arg; arg + 1 can overflow a uint64.
		if arg == math.MaxUint64 {
			text = "-18446744073709551616"
		} else {
			text = "-" + strconv.FormatUint(arg+1, 10)
		}
	}
	b := appendHead(nil, majorTag, tagDecimalText)
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

var (
	durationUnits  = regexp.MustCompile(`(\d+)([a-zµ]+)`)
	durationSyntax = regexp.MustCompile(`^(\d+[a-zµ]+)*$`)
)

// ParseDuration parses a SurrealQL duration such as "1h30m" or "2w3d" into
// nanoseconds.
func ParseDuration(duration string) (int64, error) {
	const (
		ns     = int64(1)
		us     = 1000 * ns
		ms     = 1000 * us
		sec    = 1000 * ms
		minute = 60 * sec
		hr     = 60 * minute
		day    = 24 * hr
		wk     = 7 * day
		yr     = 365 * day
	)
	if !durationSyntax.MatchString(duration) {
		return 0, fmt.Errorf("invalid duration %q", duration)
	}
	var total int64
	for _, match := range durationUnits.FindAllStringSubmatch(duration, -1) {
		value, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return 0, err
		}
		var unit int64
		switch match[2] {
		case "y":
			unit = yr
		case "w":
			unit = wk
		case "d":
			unit = day
		case "h":
			unit = hr
		case "m":
			unit = minute
		case "s":
			unit = sec
		case "ms":
			unit = ms
		case "µs", "us":
			unit = us
		case "ns":
			unit = ns
		default:
			return 0, fmt.Errorf("unknown duration unit %q", match[2])
		}
		total += value * unit
	}
	return total, nil
}

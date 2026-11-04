package models

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/v2/internal/cborwire"
)

// BigDecimal is a decimal of arbitrary precision, such as
// 79228162514264337593543950336.50: SurrealDB 4.0's bigdecimal kind. It
// holds what a decimal cannot: a coefficient of 2^96 or more, or more than 28
// digits after the point.
//
// It keeps the text the server writes: positional, with no leading zeros,
// and with the digits after the point as written, so 1.50 stays 1.50.
// [ParseBigDecimal] folds an exponent in, so 1.5e3 is 1500. For arithmetic,
// read String with math/big or a decimal library, as in
// new(big.Rat).SetString(b.String()).
//
// Like the server, [BigDecimal.Equal] and [BigDecimal.Compare] compare
// values, so 1.50 equals 1.5, while == compares the digits as written.
//
// Over cbor.2 a BigDecimal is sent as tag 192 and decoded from it. Over plain
// cbor it is sent as a decimal (tag 10) when it fits one, the way the server
// widens it, and as text otherwise, and decoded from either.
//
// The zero value is 0, a real number rather than NONE. For a field that may
// be left out, use a *BigDecimal with omitempty: omitempty never drops a
// BigDecimal, and omitzero always drops 0.
type BigDecimal struct {
	// text is the canonical text, and "" for 0, so that BigDecimal{} is the
	// same value as ParseBigDecimal("0").
	text string
}

// ParseBigDecimal parses a decimal as the server does: an optional sign,
// digits with an optional point, which may come first or last as in .5 or
// 5., and an optional exponent such as e-3. It refuses spaces, NaN and the
// infinities, and a value past the server's limits: 147455 significant digits,
// counting an exponent folded into them, an exponent of 147455, and 294910
// digits after the point.
func ParseBigDecimal(s string) (BigDecimal, error) {
	text, err := cborwire.CanonicalBigDecimal(s)
	if err != nil {
		return BigDecimal{}, err
	}
	if text == "0" {
		return BigDecimal{}, nil
	}
	return BigDecimal{text: text}, nil
}

// MustParseBigDecimal is like [ParseBigDecimal] but panics when s is not a
// decimal.
func MustParseBigDecimal(s string) BigDecimal {
	b, err := ParseBigDecimal(s)
	if err != nil {
		panic(err)
	}
	return b
}

// Compare returns -1 if b is below o, +1 if it is above, and 0 if they are
// equal in value, whatever their digits after the point.
func (b BigDecimal) Compare(o BigDecimal) int {
	bNegative, bWhole, bFrac := decimalParts(b.String())
	oNegative, oWhole, oFrac := decimalParts(o.String())
	if bNegative != oNegative {
		if bNegative {
			return -1
		}
		return 1
	}
	// Without leading zeros the longer whole part is the larger; without
	// trailing zeros the digits after the point compare as text.
	c := cmp.Compare(len(bWhole), len(oWhole))
	if c == 0 {
		c = cmp.Compare(bWhole, oWhole)
	}
	if c == 0 {
		c = cmp.Compare(bFrac, oFrac)
	}
	if bNegative {
		return -c
	}
	return c
}

// decimalParts splits a decimal in canonical form into its sign, its whole
// part and the digits after the point without the trailing zeros. Zero has
// no sign.
func decimalParts(text string) (negative bool, whole, frac string) {
	negative = strings.HasPrefix(text, "-")
	whole, frac, _ = strings.Cut(strings.TrimPrefix(text, "-"), ".")
	frac = strings.TrimRight(frac, "0")
	if whole == "0" && frac == "" {
		negative = false
	}
	return negative, whole, frac
}

// Equal reports whether b and o are equal in value, as the server compares
// them: 1.50 equals 1.5.
func (b BigDecimal) Equal(o BigDecimal) bool {
	return b.Compare(o) == 0
}

// FitsDecimal reports whether a SurrealDB decimal holds b exactly: written
// with no trailing zeros after the point, b has at most 28 digits after it,
// and its digits, read as a whole number with the point dropped, are below
// 2^96. So 1e28 fits, but 9e28 does not. The server sends such a bigdecimal
// over plain cbor as a decimal.
func (b BigDecimal) FitsDecimal() bool {
	_, ok := cborwire.BigDecimalAsDecimal(b.String())
	return ok
}

// String returns b as the server writes it: positional, with no leading zeros
// and with the digits after the point as written.
func (b BigDecimal) String() string {
	if b.text == "" {
		return "0"
	}
	return b.text
}

// SurrealString returns b as a SurrealQL bigdecimal literal, such as 1.50bigdec.
func (b BigDecimal) SurrealString() string {
	return b.String() + "bigdec"
}

// MarshalText returns b as String writes it.
func (b BigDecimal) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// UnmarshalText parses text as ParseBigDecimal does.
func (b *BigDecimal) UnmarshalText(text []byte) error {
	parsed, err := ParseBigDecimal(string(text))
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

// MarshalCBOR encodes b as tag 192 around its text. An encoder for plain
// cbor widens it.
func (b BigDecimal) MarshalCBOR() ([]byte, error) {
	text := b.String()
	return cborwire.AppendText(cborwire.AppendTag(make([]byte, 0, len(text)+12), TagBigDecimal), text), nil
}

// UnmarshalCBOR decodes a bigdecimal (tag 192) into b, or a decimal (tag 10)
// or text, which is how plain cbor carries one. NULL and NONE leave b as it
// is.
func (b *BigDecimal) UnmarshalCBOR(data []byte) error {
	const into = "models.BigDecimal"
	item, absent, err := readKindItem(data, TagBigDecimal, into)
	if err != nil || absent {
		return err
	}
	switch {
	case item.tagged && (item.tag == TagBigDecimal || item.tag == TagStringDecimal),
		!item.tagged && item.content[0]>>5 == cborMajorText:
	default:
		return item.mismatch(into)
	}
	text, err := cborwire.ReadText(item.content)
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	parsed, err := ParseBigDecimal(text)
	if err != nil {
		return fmt.Errorf("cannot decode %s: %w", into, err)
	}
	*b = parsed
	return nil
}

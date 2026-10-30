package models

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/x448/float16"
)

// CBOR tags for SurrealDB 4.0's sized numbers.
//
// They exist only in the cbor.2 dialect (WebSocket subprotocol "cbor.2", HTTP
// "application/cbor; v=2"), which a connection uses when the server offers it.
// Over plain "cbor" the server sends sized numbers widened, as plain integers
// and floats, and refuses these tags.
//
// See https://github.com/surrealdb/surrealdb-protocol/blob/v0.14.0/spec/cbor-tags.yaml.
const (
	TagI8  uint64 = 180
	TagU8  uint64 = 181
	TagI16 uint64 = 182
	TagU16 uint64 = 183
	TagI32 uint64 = 184
	TagU32 uint64 = 185
	TagU64 uint64 = 186
	TagF16 uint64 = 187
	TagF32 uint64 = 188
)

// IsSizedNumberTag reports whether tag is one of the cbor.2 sized-number tags (180 to 188).
func IsSizedNumberTag(tag uint64) bool {
	return tag >= TagI8 && tag <= TagF32
}

// Float16 is an IEEE 754 half-precision float: SurrealDB 4.0's f16 kind.
//
// Go has no half-precision float, so Float16 holds the 16 bits. Build one with
// [NewFloat16] (which refuses values a half cannot hold exactly) or
// [Float16FromBits], and read it with [Float16.Float64] or [Float16.Float32].
// The zero value is +0.
//
// Over cbor.2 a Float16 is sent as tag 187 and decoded from it. Over plain
// cbor it is sent as a plain CBOR float, the way the server widens it.
type Float16 struct {
	bits uint16
}

// NewFloat16 returns f as a Float16. It returns an error when f cannot be
// represented exactly as a half-precision float, such as 0.1 or 65536.
// NaN and the infinities are accepted.
func NewFloat16(f float64) (Float16, error) {
	h, ok := float16Exact(f)
	if !ok {
		return Float16{}, fmt.Errorf("float %v cannot be losslessly represented as f16", f)
	}
	return h, nil
}

// MustFloat16 is like [NewFloat16] but panics when f is not exactly representable.
func MustFloat16(f float64) Float16 {
	h, err := NewFloat16(f)
	if err != nil {
		panic(err)
	}
	return h
}

// Float16FromBits returns the Float16 with the given IEEE 754 binary16 bits.
func Float16FromBits(bits uint16) Float16 {
	return Float16{bits: bits}
}

// Bits returns the IEEE 754 binary16 bits of f.
func (f Float16) Bits() uint16 {
	return f.bits
}

// Float32 returns f as a float32. The conversion is exact.
func (f Float16) Float32() float32 {
	return float16.Frombits(f.bits).Float32()
}

// Float64 returns f as a float64. The conversion is exact.
func (f Float16) Float64() float64 {
	return float64(f.Float32())
}

// String formats f like strconv.FormatFloat with the shortest representation.
func (f Float16) String() string {
	return strconv.FormatFloat(f.Float64(), 'g', -1, 32)
}

// MarshalCBOR encodes f as a plain CBOR half-precision float (major type 7,
// additional information 25). A cbor.2 encoder wraps it in tag 187.
func (f Float16) MarshalCBOR() ([]byte, error) {
	return []byte{0xf9, byte(f.bits >> 8), byte(f.bits)}, nil //nolint:gosec // splitting the bits into bytes
}

// UnmarshalCBOR decodes a tag 187 f16, or a plain CBOR float of any width,
// into f. The value must be exactly representable as a half-precision float.
func (f *Float16) UnmarshalCBOR(data []byte) error {
	if len(data) >= 3 && data[0] == 0xd8 && data[1] == byte(TagF16) {
		data = data[2:]
	}
	v, err := plainCBORFloat(data)
	if err != nil {
		return fmt.Errorf("cannot decode f16: %w", err)
	}
	h, ok := float16Exact(v)
	if !ok {
		return fmt.Errorf("float %v cannot be losslessly represented as f16", v)
	}
	*f = h
	return nil
}

// plainCBORFloat decodes a single CBOR float item of any width.
func plainCBORFloat(data []byte) (float64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty input")
	}
	switch data[0] {
	case 0xf9:
		if len(data) != 3 {
			return 0, fmt.Errorf("malformed half-precision float")
		}
		return float64(float16.Frombits(uint16(data[1])<<8 | uint16(data[2])).Float32()), nil
	case 0xfa:
		if len(data) != 5 {
			return 0, fmt.Errorf("malformed single-precision float")
		}
		bits := uint32(data[1])<<24 | uint32(data[2])<<16 | uint32(data[3])<<8 | uint32(data[4])
		return float64(math.Float32frombits(bits)), nil
	case 0xfb:
		if len(data) != 9 {
			return 0, fmt.Errorf("malformed double-precision float")
		}
		var bits uint64
		for _, b := range data[1:] {
			bits = bits<<8 | uint64(b)
		}
		return math.Float64frombits(bits), nil
	default:
		return 0, fmt.Errorf("expected a CBOR float, got initial byte 0x%02x", data[0])
	}
}

// float16Exact converts f to a Float16 when no precision is lost.
func float16Exact(f float64) (Float16, bool) {
	if math.IsNaN(f) {
		return Float16{bits: float16.NaN().Bits()}, true
	}
	f32 := float32(f)
	if float64(f32) != f {
		return Float16{}, false
	}
	h := float16.Fromfloat32(f32)
	if h.Float32() != f32 {
		return Float16{}, false
	}
	return Float16{bits: h.Bits()}, true
}

// Float32Exact reports whether f converts to a float32 without losing precision.
// NaN and the infinities convert exactly.
func Float32Exact(f float64) bool {
	return math.IsNaN(f) || float64(float32(f)) == f
}

// sizedTagToGo converts a cbor.Tag holding a cbor.2 sized number, as
// fxamacker/cbor decodes one into an interface, into its Go kind (int8, uint8,
// int16, uint16, int32, uint32, uint64, Float16 or float32), checking that the
// value fits. Other values are returned as they are, with arrays and maps
// converted element by element.
func sizedTagToGo(v any) (any, error) {
	switch t := v.(type) {
	case cbor.Tag:
		if !IsSizedNumberTag(t.Number) {
			return v, nil
		}
		return sizedTagContentToGo(t.Number, t.Content)
	case []any:
		for i, e := range t {
			c, err := sizedTagToGo(e)
			if err != nil {
				return nil, err
			}
			t[i] = c
		}
		return t, nil
	case map[any]any:
		for k, e := range t {
			c, err := sizedTagToGo(e)
			if err != nil {
				return nil, err
			}
			t[k] = c
		}
		return t, nil
	case map[string]any:
		for k, e := range t {
			c, err := sizedTagToGo(e)
			if err != nil {
				return nil, err
			}
			t[k] = c
		}
		return t, nil
	default:
		return v, nil
	}
}

func sizedTagContentToGo(tag uint64, content any) (any, error) {
	if tag == TagF16 || tag == TagF32 {
		return sizedFloatContentToGo(tag, content)
	}

	var i *big.Int
	switch c := content.(type) {
	case uint64:
		i = new(big.Int).SetUint64(c)
	case int64:
		i = big.NewInt(c)
	case big.Int:
		i = &c
	default:
		return nil, fmt.Errorf("CBOR tag %d must hold an integer, got %T", tag, content)
	}
	lo, hi := sizedIntBounds(tag)
	if i.Cmp(lo) < 0 || i.Cmp(hi) > 0 {
		return nil, fmt.Errorf("integer %s is outside the range of CBOR tag %d", i, tag)
	}
	return sizedIntToGo(tag, i), nil
}

func sizedFloatContentToGo(tag uint64, content any) (any, error) {
	var f float64
	switch c := content.(type) {
	case float64:
		f = c
	case float32:
		f = float64(c)
	default:
		return nil, fmt.Errorf("CBOR tag %d must hold a float, got %T", tag, content)
	}
	if tag == TagF16 {
		return NewFloat16(f)
	}
	if !Float32Exact(f) {
		return nil, fmt.Errorf("float %v cannot be losslessly represented as f32", f)
	}
	return float32(f), nil
}

// sizedIntToGo converts i, already range-checked for tag, to the tag's Go kind.
//
//nolint:gosec // the conversions cannot overflow: the caller checked the range
func sizedIntToGo(tag uint64, i *big.Int) any {
	switch tag {
	case TagI8:
		return int8(i.Int64())
	case TagU8:
		return uint8(i.Uint64())
	case TagI16:
		return int16(i.Int64())
	case TagU16:
		return uint16(i.Uint64())
	case TagI32:
		return int32(i.Int64())
	case TagU32:
		return uint32(i.Uint64())
	default:
		return i.Uint64()
	}
}

func sizedIntBounds(tag uint64) (lo, hi *big.Int) {
	switch tag {
	case TagI8:
		return big.NewInt(math.MinInt8), big.NewInt(math.MaxInt8)
	case TagU8:
		return big.NewInt(0), big.NewInt(math.MaxUint8)
	case TagI16:
		return big.NewInt(math.MinInt16), big.NewInt(math.MaxInt16)
	case TagU16:
		return big.NewInt(0), big.NewInt(math.MaxUint16)
	case TagI32:
		return big.NewInt(math.MinInt32), big.NewInt(math.MaxInt32)
	case TagU32:
		return big.NewInt(0), big.NewInt(math.MaxUint32)
	default:
		return big.NewInt(0), new(big.Int).SetUint64(math.MaxUint64)
	}
}

// recordIDKeyFromTags converts the sized numbers in a record id key decoded by
// fxamacker/cbor. The server reads a sized integer used as the key itself as
// a plain int (so it decodes as fxamacker decodes a plain integer: uint64 when
// non-negative, int64 when negative), and keeps the widths inside an array or
// object key. A sized float is not a valid key.
func recordIDKeyFromTags(key any) (any, error) {
	v, err := sizedTagToGo(key)
	if err != nil {
		return nil, err
	}
	switch k := v.(type) {
	case int8:
		return plainInt(int64(k)), nil
	case int16:
		return plainInt(int64(k)), nil
	case int32:
		return plainInt(int64(k)), nil
	case uint8:
		return uint64(k), nil
	case uint16:
		return uint64(k), nil
	case uint32:
		return uint64(k), nil
	case uint64:
		if t, ok := key.(cbor.Tag); ok && t.Number == TagU64 && k > math.MaxInt64 {
			return nil, fmt.Errorf("record id key %d is outside the 64-bit signed range", k)
		}
		return k, nil
	case float32, Float16:
		return nil, fmt.Errorf("a %T is not a valid record id key", v)
	default:
		return v, nil
	}
}

func plainInt(i int64) any {
	if i < 0 {
		return i
	}
	return uint64(i)
}

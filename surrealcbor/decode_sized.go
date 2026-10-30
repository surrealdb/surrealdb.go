package surrealcbor

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// sizedTagName names the SurrealDB kind each sized-number tag carries.
var sizedTagName = map[uint64]string{
	models.TagI8:  "i8",
	models.TagU8:  "u8",
	models.TagI16: "i16",
	models.TagU16: "u16",
	models.TagI32: "i32",
	models.TagU32: "u32",
	models.TagU64: "u64",
	models.TagF16: "f16",
	models.TagF32: "f32",
}

// sizedIntRange is the inclusive range of each sized integer tag. u64 is
// handled separately because its maximum does not fit in int64.
var sizedIntRange = map[uint64][2]int64{
	models.TagI8:  {math.MinInt8, math.MaxInt8},
	models.TagU8:  {0, math.MaxUint8},
	models.TagI16: {math.MinInt16, math.MaxInt16},
	models.TagU16: {0, math.MaxUint16},
	models.TagI32: {math.MinInt32, math.MaxInt32},
	models.TagU32: {0, math.MaxUint32},
}

var float16Type = reflect.TypeOf(models.Float16{})

// decodeSizedNumberTag decodes the content of tag 180 to 188 (the tag number
// has been read). Plain cbor refuses these tags, as the server does.
func (d *decoder) decodeSizedNumberTag(tagNum uint64, v reflect.Value) error {
	name := sizedTagName[tagNum]
	if !d.v2() {
		return fmt.Errorf("CBOR tag %d (%s) is only valid in cbor.2, not plain cbor", tagNum, name)
	}
	if d.pos >= len(d.data) {
		return io.ErrUnexpectedEOF
	}
	if tagNum == models.TagF16 || tagNum == models.TagF32 {
		return d.decodeSizedFloat(tagNum, name, v)
	}
	return d.decodeSizedInt(tagNum, name, v)
}

func (d *decoder) decodeSizedInt(tagNum uint64, name string, v reflect.Value) error {
	major := d.data[d.pos] >> 5
	if major != 0 && major != 1 {
		return fmt.Errorf("CBOR tag %d (%s) must hold an integer, got major type %d", tagNum, name, major)
	}
	arg, err := d.readUint()
	if err != nil {
		return err
	}
	negative := major == 1

	// Range-check against the tag, whatever the destination.
	if tagNum == models.TagU64 {
		if negative {
			return fmt.Errorf("integer -%s is outside the u64 range", negMagnitude(arg))
		}
	} else {
		r := sizedIntRange[tagNum]
		if negative {
			if r[0] == 0 || arg > uint64(-(r[0]+1)) { //nolint:gosec // r[0] < 0 here, so -(r[0]+1) >= 0
				return fmt.Errorf("integer -%s is outside the %s range", negMagnitude(arg), name)
			}
		} else if arg > uint64(r[1]) { //nolint:gosec // r[1] is positive
			return fmt.Errorf("integer %d is outside the %s range", arg, name)
		}
	}

	if v.Kind() == reflect.Interface {
		v.Set(sizedIntValue(tagNum, negative, arg))
		return nil
	}
	return setSizedInt(v, name, negative, arg)
}

// negMagnitude renders the magnitude of the CBOR negative integer -1-arg without overflow.
func negMagnitude(arg uint64) string {
	if arg == math.MaxUint64 {
		return "18446744073709551616"
	}
	return fmt.Sprint(arg + 1)
}

// sizedIntValue returns the Go value of the exact kind a sized integer tag carries.
// The range has already been checked.
func sizedIntValue(tagNum uint64, negative bool, arg uint64) reflect.Value {
	var i int64
	if negative {
		i = -1 - int64(arg) //nolint:gosec // range-checked by the caller
	} else if arg <= math.MaxInt64 {
		i = int64(arg) //nolint:gosec // checked just above
	}
	switch tagNum {
	case models.TagI8:
		return reflect.ValueOf(int8(i))
	case models.TagU8:
		return reflect.ValueOf(uint8(arg)) //nolint:gosec // range-checked by the caller
	case models.TagI16:
		return reflect.ValueOf(int16(i))
	case models.TagU16:
		return reflect.ValueOf(uint16(arg)) //nolint:gosec // range-checked by the caller
	case models.TagI32:
		return reflect.ValueOf(int32(i))
	case models.TagU32:
		return reflect.ValueOf(uint32(arg)) //nolint:gosec // range-checked by the caller
	default:
		return reflect.ValueOf(arg)
	}
}

func setSizedInt(v reflect.Value, name string, negative bool, arg uint64) error {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !negative && arg > math.MaxInt64 {
			return fmt.Errorf("%s %d overflows %v", name, arg, v.Type())
		}
		var i int64
		if negative {
			i = -1 - int64(arg) //nolint:gosec // range-checked by the caller
		} else {
			i = int64(arg) //nolint:gosec // checked just above
		}
		if v.OverflowInt(i) {
			return fmt.Errorf("%s %d overflows %v", name, i, v.Type())
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if negative {
			return fmt.Errorf("%s -%s overflows %v", name, negMagnitude(arg), v.Type())
		}
		if v.OverflowUint(arg) {
			return fmt.Errorf("%s %d overflows %v", name, arg, v.Type())
		}
		v.SetUint(arg)
	case reflect.Float32, reflect.Float64:
		var f float64
		if negative {
			f = -1 - float64(arg)
		} else {
			f = float64(arg)
		}
		if v.OverflowFloat(f) {
			return fmt.Errorf("%s %v overflows %v", name, f, v.Type())
		}
		v.SetFloat(f)
	default:
		return fmt.Errorf("cannot decode %s (CBOR tag) into Go value of type %v", name, v.Type())
	}
	return nil
}

func (d *decoder) decodeSizedFloat(tagNum uint64, name string, v reflect.Value) error {
	b := d.data[d.pos]
	if b != 0xf9 && b != 0xfa && b != 0xfb {
		return fmt.Errorf("CBOR tag %d (%s) must hold a float, got initial byte 0x%02x", tagNum, name, b)
	}
	f, err := d.readPlainFloat()
	if err != nil {
		return err
	}

	if tagNum == models.TagF16 {
		h, err := models.NewFloat16(f)
		if err != nil {
			return err
		}
		switch {
		case v.Kind() == reflect.Interface:
			v.Set(reflect.ValueOf(h))
		case v.Type() == float16Type:
			v.Set(reflect.ValueOf(h))
		case v.Kind() == reflect.Float32 || v.Kind() == reflect.Float64:
			v.SetFloat(f)
		default:
			return fmt.Errorf("cannot decode f16 (CBOR tag 187) into Go value of type %v", v.Type())
		}
		return nil
	}

	if !models.Float32Exact(f) {
		return fmt.Errorf("float %v cannot be losslessly represented as f32", f)
	}
	switch v.Kind() {
	case reflect.Interface:
		v.Set(reflect.ValueOf(float32(f)))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(f)
	default:
		return fmt.Errorf("cannot decode f32 (CBOR tag 188) into Go value of type %v", v.Type())
	}
	return nil
}

// readPlainFloat reads a CBOR float of any width as a float64.
func (d *decoder) readPlainFloat() (float64, error) {
	b := d.data[d.pos]
	d.pos++
	switch b {
	case 0xf9:
		if d.pos+2 > len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		bits := binary.BigEndian.Uint16(d.data[d.pos:])
		d.pos += 2
		return float64(float16ToFloat32(bits)), nil
	case 0xfa:
		if d.pos+4 > len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		bits := binary.BigEndian.Uint32(d.data[d.pos:])
		d.pos += 4
		return float64(math.Float32frombits(bits)), nil
	default:
		if d.pos+8 > len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		bits := binary.BigEndian.Uint64(d.data[d.pos:])
		d.pos += 8
		return math.Float64frombits(bits), nil
	}
}

// recordIDKey normalises a decoded record id key. A record id key holds an
// i64, so the server reads a sized integer key (cbor.2) as a plain int key;
// this decodes it the way a plain CBOR integer decodes into an interface
// (uint64 when non-negative, int64 when negative). Sized floats are not valid
// keys. Keys inside an array or object key keep their width.
func recordIDKey(key any) (any, error) {
	switch k := key.(type) {
	case int8:
		return plainIntValue(int64(k)), nil
	case int16:
		return plainIntValue(int64(k)), nil
	case int32:
		return plainIntValue(int64(k)), nil
	case uint8:
		return uint64(k), nil
	case uint16:
		return uint64(k), nil
	case uint32:
		return uint64(k), nil
	case uint64:
		if k > math.MaxInt64 {
			return nil, fmt.Errorf("record id key %d is outside the 64-bit signed range", k)
		}
		return k, nil
	case float32, models.Float16:
		return nil, fmt.Errorf("a %T is not a valid record id key", key)
	default:
		return key, nil
	}
}

func plainIntValue(i int64) any {
	if i < 0 {
		return i
	}
	return uint64(i)
}

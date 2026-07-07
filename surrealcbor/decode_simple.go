package surrealcbor

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
)

// decodeSimple decodes a CBOR simple value (Major Type 7) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.16
func (d *decoder) decodeSimple(v reflect.Value, info byte) error {
	switch info {
	case 20: // false
		return d.decodeBool(v, false)
	case 21: // true
		return d.decodeBool(v, true)
	case 22, 23: // null or undefined
		return d.decodeNil(v)
	case 25: // float16
		return d.decodeFloat16(v)
	case 26: // float32
		return d.decodeFloat32(v)
	case 27: // float64
		return d.decodeFloat64(v)
	default:
		return d.decodeSimpleValue(v, info)
	}
}

func (d *decoder) decodeBool(v reflect.Value, val bool) error {
	if v.Kind() == reflect.Bool {
		v.SetBool(val)
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(val))
	}
	d.pos++
	return nil
}

func (d *decoder) decodeNil(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		v.Set(reflect.Zero(v.Type()))
	}
	d.pos++
	return nil
}

func (d *decoder) decodeFloat16(v reflect.Value) error {
	d.pos++
	if d.pos+1 >= len(d.data) {
		return io.ErrUnexpectedEOF
	}
	// Read 2 bytes for float16
	bits := uint16(d.data[d.pos])<<8 | uint16(d.data[d.pos+1])
	d.pos += 2
	f := float16ToFloat32(bits)
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(f))
		return nil
	}
	return setFloatValue(v, float64(f), "float16")
}

func float16ToFloat32(bits uint16) float32 {
	// Extract components
	sign := uint32(bits>>15) << 31
	exp := (bits >> 10) & 0x1f
	mant := uint32(bits & 0x3ff)

	switch exp {
	case 0:
		// Zero or subnormal
		if mant == 0 {
			return math.Float32frombits(sign)
		}
		// Subnormal - convert to normalized
		exp = 1
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		mant &= 0x3ff
	case 31:
		// Inf or NaN
		if mant == 0 {
			return math.Float32frombits(sign | 0x7f800000)
		}
		return math.Float32frombits(sign | 0x7f800000 | (mant << 13))
	}

	// Normal number
	exp = uint16(int(exp) + 127 - 15) // #nosec G115 - exp range is 0-30, safe conversion
	return math.Float32frombits(sign | (uint32(exp) << 23) | (mant << 13))
}

func (d *decoder) decodeFloat32(v reflect.Value) error {
	d.pos++
	if d.pos+3 >= len(d.data) {
		return io.ErrUnexpectedEOF
	}
	bits := binary.BigEndian.Uint32(d.data[d.pos : d.pos+4])
	d.pos += 4
	f := math.Float32frombits(bits)
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(f))
		return nil
	}
	return setFloatValue(v, float64(f), "float32")
}

func (d *decoder) decodeFloat64(v reflect.Value) error {
	d.pos++
	if d.pos+7 >= len(d.data) {
		return io.ErrUnexpectedEOF
	}
	bits := binary.BigEndian.Uint64(d.data[d.pos : d.pos+8])
	d.pos += 8
	f := math.Float64frombits(bits)
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(f))
		return nil
	}
	return setFloatValue(v, f, "float64")
}

func setFloatValue(v reflect.Value, f float64, floatKind string) error {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		v.SetFloat(f)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setFloatToInt(v, f, floatKind)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setFloatToUint(v, f, floatKind)
	default:
		if v.CanSet() {
			return fmt.Errorf("cannot unmarshal CBOR %s into Go value of type %v", floatKind, v.Type())
		}
		return nil
	}
}

func setFloatToInt(v reflect.Value, f float64, floatKind string) error {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return fmt.Errorf("cannot unmarshal CBOR %s into Go value of type %v", floatKind, v.Type())
	}
	i := int64(f)
	if float64(i) != f {
		return fmt.Errorf("cannot unmarshal CBOR %s into Go value of type %v", floatKind, v.Type())
	}
	switch v.Kind() {
	case reflect.Int8:
		if i < math.MinInt8 || i > math.MaxInt8 {
			return fmt.Errorf("value %d overflows int8", i)
		}
	case reflect.Int16:
		if i < math.MinInt16 || i > math.MaxInt16 {
			return fmt.Errorf("value %d overflows int16", i)
		}
	case reflect.Int32:
		if i < math.MinInt32 || i > math.MaxInt32 {
			return fmt.Errorf("value %d overflows int32", i)
		}
	}
	v.SetInt(i)
	return nil
}

func setFloatToUint(v reflect.Value, f float64, floatKind string) error {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || f < 0 {
		return fmt.Errorf("cannot unmarshal CBOR %s into Go value of type %v", floatKind, v.Type())
	}
	u := uint64(f)
	if float64(u) != f {
		return fmt.Errorf("cannot unmarshal CBOR %s into Go value of type %v", floatKind, v.Type())
	}
	switch v.Kind() {
	case reflect.Uint8:
		if u > math.MaxUint8 {
			return fmt.Errorf("value %d overflows uint8", u)
		}
	case reflect.Uint16:
		if u > math.MaxUint16 {
			return fmt.Errorf("value %d overflows uint16", u)
		}
	case reflect.Uint32:
		if u > math.MaxUint32 {
			return fmt.Errorf("value %d overflows uint32", u)
		}
	}
	v.SetUint(u)
	return nil
}

func (d *decoder) decodeSimpleValue(v reflect.Value, info byte) error {
	if info < 20 {
		// Simple value 0-19
		d.pos++
		if v.Kind() == reflect.Interface {
			v.Set(reflect.ValueOf(uint64(info)))
		}
		return nil
	} else if info == 24 {
		// Simple value with 1-byte argument
		d.pos++
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		val := d.data[d.pos]
		d.pos++
		if v.Kind() == reflect.Interface {
			v.Set(reflect.ValueOf(uint64(val)))
		}
		return nil
	}
	return fmt.Errorf("unknown simple value %d", info)
}

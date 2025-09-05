package surrealcbor

import (
	"fmt"
	"io"
	"math"
	"reflect"
)

// decodeMap decodes a CBOR map (Major Type 5) into the given reflect.Value.
// https://www.rfc-editor.org/rfc/rfc8949.html#section-3.1-2.12
func (d *decoder) decodeMap(v reflect.Value) error {
	// Check for indefinite length
	if d.pos >= len(d.data) {
		return io.EOF
	}
	if d.data[d.pos]&0x1f == 31 {
		d.pos++ // Skip the indefinite length marker
		return d.decodeIndefiniteMap(v)
	}

	length, err := d.readUint()
	if err != nil {
		return err
	}

	if length > math.MaxInt {
		return fmt.Errorf("map length %d overflows int", length)
	}

	switch v.Kind() {
	case reflect.Map:
		return d.decodeMapIntoMap(v, int(length))
	case reflect.Struct:
		return d.decodeMapIntoStruct(v, int(length))
	case reflect.Interface:
		return d.decodeMapIntoInterface(v, int(length))
	default:
		return fmt.Errorf("cannot decode map into %v", v.Type())
	}
}

func (d *decoder) decodeIndefiniteMap(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Map:
		return d.decodeIndefiniteMapIntoMap(v)
	case reflect.Struct:
		return d.decodeIndefiniteMapIntoStruct(v)
	case reflect.Interface:
		return d.decodeIndefiniteMapIntoInterface(v)
	default:
		return fmt.Errorf("cannot decode map into %v", v.Kind())
	}
}

func (d *decoder) decodeIndefiniteMapIntoMap(v reflect.Value) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	return d.decodeIndefiniteMapItems(v, v.Type().Key(), v.Type().Elem())
}

func (d *decoder) decodeIndefiniteMapIntoStruct(v reflect.Value) error {
	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Get key bytes without allocating a string
		keyBytes, err := d.decodeStringBytes()
		if err != nil {
			return err
		}

		// Find field using borrowed bytes - no string allocation!
		field := d.getFieldResolver().FindFieldBytes(v, keyBytes)
		if field.IsValid() && field.CanSet() {
			if err := d.decodeValue(field); err != nil {
				return err
			}
		} else {
			// Skip unknown field value without allocating
			if err := d.skipCBORItem(); err != nil {
				return err
			}
		}
	}

	return nil
}

// getFieldResolver returns the field resolver, creating a default one if needed
func (d *decoder) getFieldResolver() FieldResolver {
	if d.fieldResolver == nil {
		d.fieldResolver = NewCachedFieldResolver()
	}
	return d.fieldResolver
}

func (d *decoder) decodeIndefiniteMapIntoInterface(v reflect.Value) error {
	// Create map based on defaultMapType or use default map[string]any
	var m reflect.Value
	if d.defaultMapType != nil && d.defaultMapType.Kind() == reflect.Map {
		m = reflect.MakeMap(d.defaultMapType)
	} else {
		// Default to map[string]any for backward compatibility
		m = reflect.ValueOf(make(map[string]any))
	}

	if err := d.decodeIndefiniteMapItems(m, m.Type().Key(), m.Type().Elem()); err != nil {
		return err
	}

	v.Set(m)
	return nil
}

// decodeIndefiniteMapItems decodes indefinite map items with reusable reflect.Values
func (d *decoder) decodeIndefiniteMapItems(m reflect.Value, keyType, elemType reflect.Type) error {
	// Reuse reflect.Values to avoid allocations in the loop
	var key, value reflect.Value

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Create or reset key
		if !key.IsValid() {
			key = reflect.New(keyType).Elem()
		} else {
			key.SetZero()
		}

		// Create or reset value
		if !value.IsValid() {
			value = reflect.New(elemType).Elem()
		} else {
			value.SetZero()
		}

		if err := d.decodeValue(key); err != nil {
			return err
		}

		if err := d.decodeValue(value); err != nil {
			return err
		}

		m.SetMapIndex(key, value)
	}

	return nil
}

func (d *decoder) decodeMapIntoMap(v reflect.Value, length int) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	keyType := v.Type().Key()
	valType := v.Type().Elem()

	// Reuse reflect.Values to avoid allocations in the loop
	// Only create them once, then reuse by calling SetZero()
	var key, val reflect.Value

	for i := 0; i < length; i++ {
		// Create or reset key
		if !key.IsValid() {
			key = reflect.New(keyType).Elem()
		} else {
			key.SetZero()
		}

		// Create or reset value
		if !val.IsValid() {
			val = reflect.New(valType).Elem()
		} else {
			val.SetZero()
		}

		if err := d.decodeValue(key); err != nil {
			return err
		}
		if err := d.decodeValue(val); err != nil {
			return err
		}

		v.SetMapIndex(key, val)
	}
	return nil
}

func (d *decoder) decodeMapIntoStruct(v reflect.Value, length int) error {
	for i := 0; i < length; i++ {
		// Get key bytes without allocating a string
		keyBytes, err := d.decodeStringBytes()
		if err != nil {
			return err
		}

		// Find field using borrowed bytes - no string allocation!
		field := d.getFieldResolver().FindFieldBytes(v, keyBytes)
		if field.IsValid() && field.CanSet() {
			if err := d.decodeValue(field); err != nil {
				return err
			}
		} else {
			// Skip unknown field value without allocating
			if err := d.skipCBORItem(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *decoder) decodeMapIntoInterface(v reflect.Value, length int) error {
	// Create map based on defaultMapType or use default map[string]any
	var m reflect.Value
	if d.defaultMapType != nil && d.defaultMapType.Kind() == reflect.Map {
		m = reflect.MakeMap(d.defaultMapType)
	} else {
		// Default to map[string]any for backward compatibility
		m = reflect.ValueOf(make(map[string]any))
	}

	keyType := m.Type().Key()
	elemType := m.Type().Elem()

	// Reuse reflect.Values to avoid allocations in the loop
	var key, value reflect.Value

	for i := 0; i < length; i++ {
		// Create or reset key
		if !key.IsValid() {
			key = reflect.New(keyType).Elem()
		} else {
			key.SetZero()
		}

		// Create or reset value
		if !value.IsValid() {
			value = reflect.New(elemType).Elem()
		} else {
			value.SetZero()
		}

		if err := d.decodeValue(key); err != nil {
			return err
		}

		if err := d.decodeValue(value); err != nil {
			return err
		}

		m.SetMapIndex(key, value)
	}
	v.Set(m)
	return nil
}

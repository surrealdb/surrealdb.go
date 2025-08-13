package surrealcbor

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
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

	keyType := v.Type().Key()
	elemType := v.Type().Elem()

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Decode key
		key := reflect.New(keyType).Elem()
		if err := d.decodeValue(key); err != nil {
			return err
		}

		// Decode value
		value := reflect.New(elemType).Elem()
		if err := d.decodeValue(value); err != nil {
			return err
		}

		v.SetMapIndex(key, value)
	}

	return nil
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

		// Decode key (field name)
		var fieldName string
		if err := d.decodeValue(reflect.ValueOf(&fieldName).Elem()); err != nil {
			return err
		}

		// Find the struct field
		field := d.findStructField(v, fieldName)
		if field.IsValid() && field.CanSet() {
			if err := d.decodeValue(field); err != nil {
				return err
			}
		} else {
			// Skip unknown field value
			var skip any
			if err := d.decodeValue(reflect.ValueOf(&skip).Elem()); err != nil {
				return err
			}
		}
	}

	return nil
}

// findStructField returns the struct field with the given name, if it exists.
// It searches for the field by the following order of precedence:
// 1. Exact match on CBOR/JSON tags
// 2. Exact match on field names
// 3. Case-insensitive match on field names
//
// It does a depth-first search for embedded structs.
func (d *decoder) findStructField(v reflect.Value, name string) reflect.Value {
	if field := d.findFieldByTag(v, name); field.IsValid() {
		return field
	}

	if field := d.findFieldByName(v, name); field.IsValid() {
		return field
	}

	return d.findFieldByNameCaseInsensitive(v, name)
}

// findFieldByTag returns the struct field with the given CBOR or JSON tag name, if it exists.
func (d *decoder) findFieldByTag(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if embeddedField := d.findFieldByTag(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}

		// Check for matching tag
		if tagName := d.getFieldTagName(&field); tagName == name {
			return v.Field(i)
		}
	}

	return reflect.Value{}
}

func (d *decoder) findFieldByName(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if embeddedField := d.findFieldByName(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}

		if field.Name == name {
			return v.Field(i)
		}
	}

	return reflect.Value{}
}

// getFieldTagName returns the CBOR or JSON tag name for a struct field.
func (d *decoder) getFieldTagName(field *reflect.StructField) string {
	if tag := field.Tag.Get("cbor"); tag != "" {
		// Parse tag to handle comma-separated options like "name,omitempty"
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		return tag
	}

	if tag := field.Tag.Get("json"); tag != "" {
		// Parse tag to handle comma-separated options
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		return tag
	}

	return ""
}

// findFieldByNameCaseInsensitive returns the struct field with a case-insensitive match on field name
func (d *decoder) findFieldByNameCaseInsensitive(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if embeddedField := d.findFieldByNameCaseInsensitive(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}

		if strings.EqualFold(field.Name, name) {
			return v.Field(i)
		}
	}

	return reflect.Value{}
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

	keyType := m.Type().Key()
	elemType := m.Type().Elem()

	for {
		// Check for break marker (0xFF)
		if d.pos >= len(d.data) {
			return io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 0xFF {
			d.pos++ // Skip break marker
			break
		}

		// Decode key
		key := reflect.New(keyType).Elem()
		if err := d.decodeValue(key); err != nil {
			return err
		}

		// Decode value
		value := reflect.New(elemType).Elem()
		if err := d.decodeValue(value); err != nil {
			return err
		}

		m.SetMapIndex(key, value)
	}

	v.Set(m)
	return nil
}

func (d *decoder) decodeMapIntoMap(v reflect.Value, length int) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	keyType := v.Type().Key()
	valType := v.Type().Elem()

	for i := 0; i < length; i++ {
		key := reflect.New(keyType).Elem()
		val := reflect.New(valType).Elem()

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
		var key string
		if err := d.decodeValue(reflect.ValueOf(&key).Elem()); err != nil {
			return err
		}

		// Find field by json tag or name
		field := d.findStructField(v, key)
		if field.IsValid() && field.CanSet() {
			if err := d.decodeValue(field); err != nil {
				return err
			}
		} else {
			// Skip unknown field
			var discard any
			if err := d.decodeValue(reflect.ValueOf(&discard).Elem()); err != nil {
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

	for i := 0; i < length; i++ {
		// Decode key
		key := reflect.New(keyType).Elem()
		if err := d.decodeValue(key); err != nil {
			return err
		}

		// Decode value
		value := reflect.New(elemType).Elem()
		if err := d.decodeValue(value); err != nil {
			return err
		}

		m.SetMapIndex(key, value)
	}
	v.Set(m)
	return nil
}

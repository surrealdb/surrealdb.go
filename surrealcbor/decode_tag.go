package surrealcbor

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func (d *decoder) decodeTag(v reflect.Value) error {
	tagNum, err := d.readUint()
	if err != nil {
		return err
	}

	// Special handling for Tag 6 (NONE)
	if tagNum == models.TagNone {
		return d.decodeNoneTag(v)
	}

	// Delegate to specific decoder based on tag
	return d.decodeSpecificTag(tagNum, v)
}

func (d *decoder) decodeSpecificTag(tagNum uint64, v reflect.Value) error {
	// Core types
	if tagNum <= 15 {
		return d.decodeCoreTag(tagNum, v)
	}
	// UUID type
	if tagNum == models.TagSpecBinaryUUID {
		return d.decodeBinaryUUIDTag(v)
	}
	// Range/Bound types
	if tagNum >= 49 && tagNum <= 51 {
		return d.decodeRangeBoundTag(tagNum, v)
	}
	// Geometry types
	if tagNum >= 88 && tagNum <= 94 {
		return d.decodeGeometryTag(tagNum, v)
	}
	return d.decodeUnknownTag(v)
}

func (d *decoder) decodeCoreTag(tagNum uint64, v reflect.Value) error {
	switch tagNum {
	case 0: // DateTime (ISO 8601 string)
		return d.decodeDateTimeStringTag(v)
	case models.TagTable: // Table
		return d.decodeTableTag(v)
	case models.TagRecordID: // RecordID
		return d.decodeRecordIDTag(v)
	case models.TagStringUUID: // UUIDString (Tag 9)
		return d.decodeStringUUIDTag(v)
	case models.TagStringDecimal: // DecimalString (Tag 10)
		return d.decodeStringDecimalTag(v)
	case models.TagCustomDatetime: // CustomDateTime (binary format)
		return d.decodeDateTimeTag(v)
	case models.TagStringDuration: // CustomDurationString (Tag 13)
		return d.decodeStringDurationTag(v)
	case models.TagCustomDuration: // CustomDuration (Tag 14)
		return d.decodeCustomDurationTag(v)
	case models.TagFuture: // Future
		return d.decodeFutureTag(v)
	default:
		return d.decodeUnknownTag(v)
	}
}

func (d *decoder) decodeRangeBoundTag(tagNum uint64, v reflect.Value) error {
	switch tagNum {
	case models.TagRange: // Range (Tag 49)
		return d.decodeRangeTag(v)
	case models.TagBoundIncluded: // BoundIncluded (Tag 50)
		return d.decodeBoundIncludedTag(v)
	case models.TagBoundExcluded: // BoundExcluded (Tag 51)
		return d.decodeBoundExcludedTag(v)
	default:
		return d.decodeUnknownTag(v)
	}
}

func (d *decoder) decodeGeometryTag(tagNum uint64, v reflect.Value) error {
	switch tagNum {
	case models.TagGeometryPoint: // GeometryPoint
		return d.decodeGeometryPointTag(v)
	case models.TagGeometryLine: // GeometryLine (Tag 89)
		return d.decodeGeometryLineTag(v)
	case models.TagGeometryPolygon: // GeometryPolygon (Tag 90)
		return d.decodeGeometryPolygonTag(v)
	case models.TagGeometryMultiPoint: // GeometryMultiPoint (Tag 91)
		return d.decodeGeometryMultiPointTag(v)
	case models.TagGeometryMultiLine: // GeometryMultiLine (Tag 92)
		return d.decodeGeometryMultiLineTag(v)
	case models.TagGeometryMultiPolygon: // GeometryMultiPolygon (Tag 93)
		return d.decodeGeometryMultiPolygonTag(v)
	case models.TagGeometryCollection: // GeometryCollection (Tag 94)
		return d.decodeGeometryCollectionTag(v)
	default:
		return d.decodeUnknownTag(v)
	}
}

func (d *decoder) decodeNoneTag(v reflect.Value) error {
	// Read the content (should be null/undefined)
	if d.pos >= len(d.data) {
		return io.ErrUnexpectedEOF
	}

	nextByte := d.data[d.pos]
	if nextByte == 0xf6 || nextByte == 0xf7 { // null or undefined
		d.pos++
		// Set the value to nil/zero
		v.Set(reflect.Zero(v.Type()))
		return nil
	}
	return nil
}

// toInt64 converts various numeric types to int64
func toInt64(val any, name string) (int64, error) {
	switch v := val.(type) {
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("%s overflow: %d", name, v)
		}
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid %s type: %T", name, val)
	}
}

func (d *decoder) decodeDateTimeStringTag(v reflect.Value) error {
	// Tag 0 - DateTime as ISO 8601 string
	var str string
	if err := d.decodeValue(reflect.ValueOf(&str).Elem()); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return err
	}
	if v.Type() == reflect.TypeOf(time.Time{}) {
		v.Set(reflect.ValueOf(t))
	} else if v.Type() == reflect.TypeOf(models.CustomDateTime{}) {
		v.Set(reflect.ValueOf(models.CustomDateTime{Time: t}))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(t))
	}
	return nil
}

func (d *decoder) decodeDateTimeTag(v reflect.Value) error {
	// CustomDateTime is encoded as [seconds, nanoseconds]
	var arr []any
	if err := d.decodeValue(reflect.ValueOf(&arr).Elem()); err != nil {
		return fmt.Errorf("failed to decode datetime array: %w", err)
	}
	if len(arr) != 2 {
		return fmt.Errorf("invalid datetime array length: %d", len(arr))
	}

	// Convert elements to int64
	seconds, err := toInt64(arr[0], "datetime seconds")
	if err != nil {
		return err
	}
	nanoseconds, err := toInt64(arr[1], "datetime nanoseconds")
	if err != nil {
		return err
	}

	// Convert to time.Time
	t := time.Unix(seconds, nanoseconds).UTC()

	// Check if value is settable
	if !v.CanSet() {
		return fmt.Errorf("cannot set value of type %v", v.Type())
	}

	if v.Type() == reflect.TypeOf(time.Time{}) {
		v.Set(reflect.ValueOf(t))
	} else if v.Type() == reflect.TypeOf(models.CustomDateTime{}) {
		v.Set(reflect.ValueOf(models.CustomDateTime{Time: t}))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(models.CustomDateTime{Time: t}))
	} else {
		// Try to set as time.Time for any other type
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(t))
		} else {
			return fmt.Errorf("cannot decode datetime into %v", v.Type())
		}
	}
	return nil
}

func (d *decoder) decodeTableTag(v reflect.Value) error {
	var tableName string
	if err := d.decodeValue(reflect.ValueOf(&tableName).Elem()); err != nil {
		return err
	}
	if v.Type() == reflect.TypeOf(models.Table("")) {
		v.Set(reflect.ValueOf(models.Table(tableName)))
	} else if v.Kind() == reflect.Interface {
		// When decoding to interface{}, always create the proper models.Table type
		v.Set(reflect.ValueOf(models.Table(tableName)))
	} else {
		v.Set(reflect.ValueOf(tableName))
	}
	return nil
}

func (d *decoder) decodeRecordIDTag(v reflect.Value) error {
	// RecordID is encoded as an array [table, id]
	var arr []any
	if err := d.decodeValue(reflect.ValueOf(&arr).Elem()); err != nil {
		return err
	}
	if len(arr) == 2 {
		table, _ := arr[0].(string)
		recordID := models.RecordID{
			Table: table,
			ID:    arr[1],
		}
		if v.Type() == reflect.TypeOf(models.RecordID{}) {
			v.Set(reflect.ValueOf(recordID))
		} else if v.Kind() == reflect.Interface {
			v.Set(reflect.ValueOf(recordID))
		}
	}
	return nil
}

func (d *decoder) decodeCustomDurationTag(v reflect.Value) error {
	// Per SurrealDB spec:
	// - CustomDuration is encoded as [optional seconds, optional nanoseconds]
	// - Empty array represents duration of 0
	var arr [2]int64
	if err := d.decodeValue(reflect.ValueOf(&arr).Elem()); err != nil {
		return fmt.Errorf("failed to decode duration array: %w", err)
	}

	seconds, nanoseconds := arr[0], arr[1]

	duration := time.Duration(seconds*int64(time.Second) + nanoseconds)

	if v.Type() == reflect.TypeOf(time.Duration(0)) {
		v.Set(reflect.ValueOf(duration))
	} else if v.Type() == reflect.TypeOf(models.CustomDuration{}) {
		v.Set(reflect.ValueOf(models.CustomDuration{Duration: duration}))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(models.CustomDuration{Duration: duration}))
	} else {
		v.Set(reflect.ValueOf(duration))
	}
	return nil
}

func (d *decoder) decodeGeometryPointTag(v reflect.Value) error {
	// GeometryPoint is encoded as [longitude, latitude]
	var coords [2]float64
	if err := d.decodeValue(reflect.ValueOf(&coords).Elem()); err != nil {
		return err
	}

	point := models.GeometryPoint{
		Longitude: coords[0],
		Latitude:  coords[1],
	}

	if v.Type() == reflect.TypeOf(models.GeometryPoint{}) {
		v.Set(reflect.ValueOf(point))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(point))
	}
	return nil
}

func (d *decoder) decodeFutureTag(v reflect.Value) error {
	// Future has private fields, we'll just decode as empty struct for now
	// Skip the tag content
	var content any
	if err := d.decodeValue(reflect.ValueOf(&content).Elem()); err != nil {
		return err
	}

	// Create an empty Future
	var future models.Future

	if v.Type() == reflect.TypeOf(models.Future{}) {
		v.Set(reflect.ValueOf(future))
	} else if v.Type() == reflect.TypeOf(&models.Future{}) {
		v.Set(reflect.ValueOf(&future))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(&future))
	}

	return nil
}

func (d *decoder) decodeBinaryUUIDTag(v reflect.Value) error {
	// Binary UUID is 16 raw bytes (not a CBOR byte string)
	// Read the byte string header
	if d.pos >= len(d.data) {
		return io.ErrUnexpectedEOF
	}

	b := d.data[d.pos]
	d.pos++

	// Should be a byte string (major type 2)
	if b>>5 != 2 {
		return fmt.Errorf("expected byte string for UUID, got major type %d", b>>5)
	}

	// Get the length (should be 16)
	info := b & 0x1f
	var length int
	if info < 24 {
		length = int(info)
	} else {
		return fmt.Errorf("unexpected UUID byte string format")
	}

	if length != 16 {
		return fmt.Errorf("UUID should be 16 bytes, got %d", length)
	}

	// Read the 16 bytes
	if d.pos+16 > len(d.data) {
		return io.ErrUnexpectedEOF
	}

	var uuidBytes [16]byte
	copy(uuidBytes[:], d.data[d.pos:d.pos+16])
	d.pos += 16

	// Create uuid.UUID from bytes
	var uuid models.UUID
	copy(uuid.UUID[:], uuidBytes[:])

	if v.Type() == reflect.TypeOf(models.UUID{}) {
		v.Set(reflect.ValueOf(uuid))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(uuid))
	}
	return nil
}

// decodeStringTypeTag is a generic function to decode string-based custom types
func (d *decoder) decodeStringTypeTag(v reflect.Value, targetType reflect.Type, makeValue func(string) reflect.Value) error {
	var str string
	if err := d.decodeValue(reflect.ValueOf(&str).Elem()); err != nil {
		return err
	}
	if v.Type() == targetType {
		v.Set(makeValue(str))
	} else if v.Kind() == reflect.Interface {
		v.Set(makeValue(str))
	} else {
		v.Set(reflect.ValueOf(str))
	}
	return nil
}

func (d *decoder) decodeStringUUIDTag(v reflect.Value) error {
	return d.decodeStringTypeTag(v, reflect.TypeOf(models.UUIDString("")),
		func(s string) reflect.Value { return reflect.ValueOf(models.UUIDString(s)) })
}

func (d *decoder) decodeStringDecimalTag(v reflect.Value) error {
	return d.decodeStringTypeTag(v, reflect.TypeOf(models.DecimalString("")),
		func(s string) reflect.Value { return reflect.ValueOf(models.DecimalString(s)) })
}

func (d *decoder) decodeStringDurationTag(v reflect.Value) error {
	return d.decodeStringTypeTag(v, reflect.TypeOf(models.CustomDurationString("")),
		func(s string) reflect.Value { return reflect.ValueOf(models.CustomDurationString(s)) })
}

func (d *decoder) decodeRangeTag(v reflect.Value) error {
	// Range is encoded as an array [begin, end] where begin and end are also tagged
	// Since Range is generic, we decode as raw data for now
	var arr []any
	if err := d.decodeValue(reflect.ValueOf(&arr).Elem()); err != nil {
		return err
	}
	// For now, just store the array in interface
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(arr))
	}
	return nil
}

func (d *decoder) decodeBoundIncludedTag(v reflect.Value) error {
	// BoundIncluded wraps a value
	var content any
	if err := d.decodeValue(reflect.ValueOf(&content).Elem()); err != nil {
		return err
	}
	// For now, store raw content
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(content))
	}
	return nil
}

func (d *decoder) decodeBoundExcludedTag(v reflect.Value) error {
	// BoundExcluded wraps a value
	var content any
	if err := d.decodeValue(reflect.ValueOf(&content).Elem()); err != nil {
		return err
	}
	// For now, store raw content
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(content))
	}
	return nil
}

// decodeGeometryArrayTag is a generic function for geometry types that are arrays
func decodeGeometryArrayTag[T any](d *decoder, v reflect.Value, targetType reflect.Type, makeValue func(T) reflect.Value) error {
	var data T
	if err := d.decodeValue(reflect.ValueOf(&data).Elem()); err != nil {
		return err
	}
	value := makeValue(data)
	if v.Type() == targetType {
		v.Set(value)
	} else if v.Kind() == reflect.Interface {
		v.Set(value)
	}
	return nil
}

func (d *decoder) decodeGeometryLineTag(v reflect.Value) error {
	return decodeGeometryArrayTag(d, v, reflect.TypeOf(models.GeometryLine{}),
		func(points []models.GeometryPoint) reflect.Value {
			return reflect.ValueOf(models.GeometryLine(points))
		})
}

func (d *decoder) decodeGeometryPolygonTag(v reflect.Value) error {
	return decodeGeometryArrayTag(d, v, reflect.TypeOf(models.GeometryPolygon{}),
		func(lines []models.GeometryLine) reflect.Value {
			return reflect.ValueOf(models.GeometryPolygon(lines))
		})
}

func (d *decoder) decodeGeometryMultiPointTag(v reflect.Value) error {
	return decodeGeometryArrayTag(d, v, reflect.TypeOf(models.GeometryMultiPoint{}),
		func(points []models.GeometryPoint) reflect.Value {
			return reflect.ValueOf(models.GeometryMultiPoint(points))
		})
}

func (d *decoder) decodeGeometryMultiLineTag(v reflect.Value) error {
	return decodeGeometryArrayTag(d, v, reflect.TypeOf(models.GeometryMultiLine{}),
		func(lines []models.GeometryLine) reflect.Value {
			return reflect.ValueOf(models.GeometryMultiLine(lines))
		})
}

func (d *decoder) decodeGeometryMultiPolygonTag(v reflect.Value) error {
	return decodeGeometryArrayTag(d, v, reflect.TypeOf(models.GeometryMultiPolygon{}),
		func(polygons []models.GeometryPolygon) reflect.Value {
			return reflect.ValueOf(models.GeometryMultiPolygon(polygons))
		})
}

func (d *decoder) decodeGeometryCollectionTag(v reflect.Value) error {
	// GeometryCollection is an array of any geometry types
	var collection []any
	if err := d.decodeValue(reflect.ValueOf(&collection).Elem()); err != nil {
		return err
	}
	geomCollection := models.GeometryCollection(collection)
	if v.Type() == reflect.TypeOf(models.GeometryCollection{}) {
		v.Set(reflect.ValueOf(geomCollection))
	} else if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(geomCollection))
	}
	return nil
}

func (d *decoder) decodeUnknownTag(v reflect.Value) error {
	// For unknown tags, decode the content and wrap in a tag structure if needed
	var content any
	if err := d.decodeValue(reflect.ValueOf(&content).Elem()); err != nil {
		return err
	}
	if v.Kind() == reflect.Interface {
		v.Set(reflect.ValueOf(content))
	}
	return nil
}

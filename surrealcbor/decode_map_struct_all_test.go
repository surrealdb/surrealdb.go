package surrealcbor

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_map_structwithAllSupportedTypes_eachWithPointerAndNonPointerVariant
// tests that all supported types can be properly marshaled and unmarshaled with:
// - For primitives: (1) value type, (2) pointer with value, (3) pointer with nil
// - For maps/slices: (1) non-empty value, (2) nil value, (3) pointer to non-empty, (4) pointer to nil
func TestDecode_map_structwithAllSupportedTypes_eachWithPointerAndNonPointerVariant(t *testing.T) {
	type AllTypesComplete struct {
		// ============ BOOLEAN TYPES ============
		// Three variants: value, pointer with value, pointer with nil
		BoolVal    bool  `json:"bool_val"`
		BoolPtr    *bool `json:"bool_ptr"`
		BoolNilPtr *bool `json:"bool_nil_ptr"`

		// ============ STRING TYPES ============
		StringVal    string  `json:"string_val"`
		StringPtr    *string `json:"string_ptr"`
		StringNilPtr *string `json:"string_nil_ptr"`

		// ============ INTEGER TYPES ============
		// int
		IntVal    int  `json:"int_val"`
		IntPtr    *int `json:"int_ptr"`
		IntNilPtr *int `json:"int_nil_ptr"`

		// int8
		Int8Val    int8  `json:"int8_val"`
		Int8Ptr    *int8 `json:"int8_ptr"`
		Int8NilPtr *int8 `json:"int8_nil_ptr"`

		// int16
		Int16Val    int16  `json:"int16_val"`
		Int16Ptr    *int16 `json:"int16_ptr"`
		Int16NilPtr *int16 `json:"int16_nil_ptr"`

		// int32
		Int32Val    int32  `json:"int32_val"`
		Int32Ptr    *int32 `json:"int32_ptr"`
		Int32NilPtr *int32 `json:"int32_nil_ptr"`

		// int64
		Int64Val    int64  `json:"int64_val"`
		Int64Ptr    *int64 `json:"int64_ptr"`
		Int64NilPtr *int64 `json:"int64_nil_ptr"`

		// ============ UNSIGNED INTEGER TYPES ============
		// uint
		UintVal    uint  `json:"uint_val"`
		UintPtr    *uint `json:"uint_ptr"`
		UintNilPtr *uint `json:"uint_nil_ptr"`

		// uint8
		Uint8Val    uint8  `json:"uint8_val"`
		Uint8Ptr    *uint8 `json:"uint8_ptr"`
		Uint8NilPtr *uint8 `json:"uint8_nil_ptr"`

		// uint16
		Uint16Val    uint16  `json:"uint16_val"`
		Uint16Ptr    *uint16 `json:"uint16_ptr"`
		Uint16NilPtr *uint16 `json:"uint16_nil_ptr"`

		// uint32
		Uint32Val    uint32  `json:"uint32_val"`
		Uint32Ptr    *uint32 `json:"uint32_ptr"`
		Uint32NilPtr *uint32 `json:"uint32_nil_ptr"`

		// uint64
		Uint64Val    uint64  `json:"uint64_val"`
		Uint64Ptr    *uint64 `json:"uint64_ptr"`
		Uint64NilPtr *uint64 `json:"uint64_nil_ptr"`

		// ============ FLOATING POINT TYPES ============
		// float32
		Float32Val    float32  `json:"float32_val"`
		Float32Ptr    *float32 `json:"float32_ptr"`
		Float32NilPtr *float32 `json:"float32_nil_ptr"`

		// float64
		Float64Val    float64  `json:"float64_val"`
		Float64Ptr    *float64 `json:"float64_ptr"`
		Float64NilPtr *float64 `json:"float64_nil_ptr"`

		// ============ BYTE SLICE (SPECIAL CASE) ============
		// Four variants for slices
		ByteSliceVal    []byte  `json:"byte_slice_val"`
		ByteSliceNil    []byte  `json:"byte_slice_nil"`
		ByteSlicePtr    *[]byte `json:"byte_slice_ptr"`
		ByteSliceNilPtr *[]byte `json:"byte_slice_nil_ptr"`

		// ============ STRING SLICE ============
		StringSliceVal    []string  `json:"string_slice_val"`
		StringSliceNil    []string  `json:"string_slice_nil"`
		StringSlicePtr    *[]string `json:"string_slice_ptr"`
		StringSliceNilPtr *[]string `json:"string_slice_nil_ptr"`

		// ============ INT SLICE ============
		IntSliceVal    []int  `json:"int_slice_val"`
		IntSliceNil    []int  `json:"int_slice_nil"`
		IntSlicePtr    *[]int `json:"int_slice_ptr"`
		IntSliceNilPtr *[]int `json:"int_slice_nil_ptr"`

		// ============ ANY SLICE ============
		AnySliceVal    []any  `json:"any_slice_val"`
		AnySliceNil    []any  `json:"any_slice_nil"`
		AnySlicePtr    *[]any `json:"any_slice_ptr"`
		AnySliceNilPtr *[]any `json:"any_slice_nil_ptr"`

		// ============ STRING MAP ============
		// Four variants for maps
		StringMapVal    map[string]string  `json:"string_map_val"`
		StringMapNil    map[string]string  `json:"string_map_nil"`
		StringMapPtr    *map[string]string `json:"string_map_ptr"`
		StringMapNilPtr *map[string]string `json:"string_map_nil_ptr"`

		// ============ INT MAP ============
		IntMapVal    map[string]int  `json:"int_map_val"`
		IntMapNil    map[string]int  `json:"int_map_nil"`
		IntMapPtr    *map[string]int `json:"int_map_ptr"`
		IntMapNilPtr *map[string]int `json:"int_map_nil_ptr"`

		// ============ ANY MAP ============
		AnyMapVal    map[string]any  `json:"any_map_val"`
		AnyMapNil    map[string]any  `json:"any_map_nil"`
		AnyMapPtr    *map[string]any `json:"any_map_ptr"`
		AnyMapNilPtr *map[string]any `json:"any_map_nil_ptr"`

		// ============ TIME TYPES ============
		TimeVal    time.Time  `json:"time_val"`
		TimePtr    *time.Time `json:"time_ptr"`
		TimeNilPtr *time.Time `json:"time_nil_ptr"`

		// ============ INTERFACE/ANY TYPE ============
		AnyVal    any  `json:"any_val"`
		AnyNil    any  `json:"any_nil"`
		AnyPtr    *any `json:"any_ptr"`
		AnyNilPtr *any `json:"any_nil_ptr"`

		// ============ NESTED STRUCT ============
		NestedStructVal struct {
			Field1 string `json:"field1"`
			Field2 int    `json:"field2"`
		} `json:"nested_struct_val"`

		NestedStructPtr *struct {
			Field1 string `json:"field1"`
			Field2 int    `json:"field2"`
		} `json:"nested_struct_ptr"`

		NestedStructNilPtr *struct {
			Field1 string `json:"field1"`
			Field2 int    `json:"field2"`
		} `json:"nested_struct_nil_ptr"`

		// ============ SURREALDB TYPES (BASIC) ============
		// These should work correctly
		RecordIDVal    models.RecordID  `json:"record_id_val"`
		RecordIDPtr    *models.RecordID `json:"record_id_ptr"`
		RecordIDNilPtr *models.RecordID `json:"record_id_nil_ptr"`

		TableVal    models.Table  `json:"table_val"`
		TablePtr    *models.Table `json:"table_ptr"`
		TableNilPtr *models.Table `json:"table_nil_ptr"`

		// None type (special case - always represents nil)
		NoneVal models.CustomNil `json:"none_val"`

		// ============ ADDITIONAL SURREALDB TYPES ============
		// UUID
		UUIDVal    models.UUID  `json:"uuid_val"`
		UUIDPtr    *models.UUID `json:"uuid_ptr"`
		UUIDNilPtr *models.UUID `json:"uuid_nil_ptr"`

		// Geometry types
		GeometryPointVal    models.GeometryPoint  `json:"geometry_point_val"`
		GeometryPointPtr    *models.GeometryPoint `json:"geometry_point_ptr"`
		GeometryPointNilPtr *models.GeometryPoint `json:"geometry_point_nil_ptr"`

		// CustomDuration
		CustomDurationVal    models.CustomDuration  `json:"custom_duration_val"`
		CustomDurationPtr    *models.CustomDuration `json:"custom_duration_ptr"`
		CustomDurationNilPtr *models.CustomDuration `json:"custom_duration_nil_ptr"`

		// Future (has private fields, so testing is limited)
		FuturePtr    *models.Future `json:"future_ptr"`
		FutureNilPtr *models.Future `json:"future_nil_ptr"`
	}

	// Create helper values for pointers
	boolTrue := true
	stringVal := "test string"
	intVal := 42
	int8Val := int8(8)
	int16Val := int16(16)
	int32Val := int32(32)
	int64Val := int64(64)
	uintVal := uint(100)
	uint8Val := uint8(8)
	uint16Val := uint16(16)
	uint32Val := uint32(32)
	uint64Val := uint64(64)
	float32Val := float32(3.14)
	float64Val := 2.71828

	byteSlice := []byte("byte data")
	stringSlice := []string{"item1", "item2"}
	intSlice := []int{1, 2, 3}
	anySlice := []any{"mixed", 123, true}

	stringMap := map[string]string{"key1": "value1"}
	intMap := map[string]int{"one": 1, "two": 2}
	anyMap := map[string]any{"str": "string", "num": 42}

	now := time.Now().Truncate(time.Second)
	anyValue := any("interface value")

	recordID := models.NewRecordID("users", "123")
	table := models.Table("products")

	// Create UUID value
	uuidVal, _ := uuid.NewV4()
	uuidModel := models.UUID{UUID: uuidVal}

	// Create GeometryPoint
	geometryPoint := models.NewGeometryPoint(37.7749, -122.4194)

	// Create CustomDuration
	customDuration := models.CustomDuration{Duration: time.Hour * 2}

	// Create Future (can't set private fields, so we'll just test pointer preservation)
	futureVal := &models.Future{}

	// Create the original struct with all values
	original := AllTypesComplete{
		// Boolean types
		BoolVal:    true,
		BoolPtr:    &boolTrue,
		BoolNilPtr: nil,

		// String types
		StringVal:    "hello",
		StringPtr:    &stringVal,
		StringNilPtr: nil,

		// Integer types
		IntVal:      100,
		IntPtr:      &intVal,
		IntNilPtr:   nil,
		Int8Val:     127,
		Int8Ptr:     &int8Val,
		Int8NilPtr:  nil,
		Int16Val:    32767,
		Int16Ptr:    &int16Val,
		Int16NilPtr: nil,
		Int32Val:    2147483647,
		Int32Ptr:    &int32Val,
		Int32NilPtr: nil,
		Int64Val:    9223372036854775807,
		Int64Ptr:    &int64Val,
		Int64NilPtr: nil,

		// Unsigned integer types
		UintVal:      200,
		UintPtr:      &uintVal,
		UintNilPtr:   nil,
		Uint8Val:     255,
		Uint8Ptr:     &uint8Val,
		Uint8NilPtr:  nil,
		Uint16Val:    65535,
		Uint16Ptr:    &uint16Val,
		Uint16NilPtr: nil,
		Uint32Val:    4294967295,
		Uint32Ptr:    &uint32Val,
		Uint32NilPtr: nil,
		Uint64Val:    18446744073709551615,
		Uint64Ptr:    &uint64Val,
		Uint64NilPtr: nil,

		// Float types
		Float32Val:    1.23,
		Float32Ptr:    &float32Val,
		Float32NilPtr: nil,
		Float64Val:    4.56,
		Float64Ptr:    &float64Val,
		Float64NilPtr: nil,

		// Byte slice - four variants
		ByteSliceVal:    []byte("hello bytes"),
		ByteSliceNil:    nil,
		ByteSlicePtr:    &byteSlice,
		ByteSliceNilPtr: nil,

		// String slice - four variants
		StringSliceVal:    []string{"a", "b", "c"},
		StringSliceNil:    nil,
		StringSlicePtr:    &stringSlice,
		StringSliceNilPtr: nil,

		// Int slice - four variants
		IntSliceVal:    []int{1, 2, 3},
		IntSliceNil:    nil,
		IntSlicePtr:    &intSlice,
		IntSliceNilPtr: nil,

		// Any slice - four variants
		AnySliceVal:    []any{"mixed", 123, true},
		AnySliceNil:    nil,
		AnySlicePtr:    &anySlice,
		AnySliceNilPtr: nil,

		// String map - four variants
		StringMapVal:    map[string]string{"k1": "v1", "k2": "v2"},
		StringMapNil:    nil,
		StringMapPtr:    &stringMap,
		StringMapNilPtr: nil,

		// Int map - four variants
		IntMapVal:    map[string]int{"one": 1, "two": 2},
		IntMapNil:    nil,
		IntMapPtr:    &intMap,
		IntMapNilPtr: nil,

		// Any map - four variants
		AnyMapVal:    map[string]any{"str": "string", "num": 42},
		AnyMapNil:    nil,
		AnyMapPtr:    &anyMap,
		AnyMapNilPtr: nil,

		// Time types
		TimeVal:    now,
		TimePtr:    &now,
		TimeNilPtr: nil,

		// Interface/any types
		AnyVal:    "interface value",
		AnyNil:    nil,
		AnyPtr:    &anyValue,
		AnyNilPtr: nil,

		// Nested struct
		NestedStructVal: struct {
			Field1 string `json:"field1"`
			Field2 int    `json:"field2"`
		}{
			Field1: "nested1",
			Field2: 999,
		},
		NestedStructPtr: &struct {
			Field1 string `json:"field1"`
			Field2 int    `json:"field2"`
		}{
			Field1: "nested2",
			Field2: 888,
		},
		NestedStructNilPtr: nil,

		// SurrealDB types
		RecordIDVal:    models.NewRecordID("table", "id"),
		RecordIDPtr:    &recordID,
		RecordIDNilPtr: nil,
		TableVal:       models.Table("mytable"),
		TablePtr:       &table,
		TableNilPtr:    nil,
		NoneVal:        models.None,

		// Additional SurrealDB types
		UUIDVal:              uuidModel,
		UUIDPtr:              &uuidModel,
		UUIDNilPtr:           nil,
		GeometryPointVal:     geometryPoint,
		GeometryPointPtr:     &geometryPoint,
		GeometryPointNilPtr:  nil,
		CustomDurationVal:    customDuration,
		CustomDurationPtr:    &customDuration,
		CustomDurationNilPtr: nil,
		FuturePtr:            futureVal,
		FutureNilPtr:         nil,
	}

	// Marshal
	data, err := Marshal(original)
	require.NoError(t, err, "Marshal failed")

	// Unmarshal
	var decoded AllTypesComplete
	err = Unmarshal(data, &decoded)
	require.NoError(t, err, "Unmarshal failed")

	// Compare most fields with a single assertion
	// Note: We need to handle some known CBOR encoding differences:
	// 1. Numbers in interface{}/any fields may be decoded as uint64 vs int
	// 2. Time locations are not preserved (decoded as nil/UTC)

	// First, ensure that there are differences before the adjustments
	assert.NotEqual(t, original, decoded, "Original and decoded structs should differ")

	// Create a copy of original for comparison to avoid mutating the original
	expected := original

	// For a comprehensive comparison, we'll adjust the expected values for known differences
	// First, let's fix the time values to use UTC for comparison
	expected.TimeVal = expected.TimeVal.UTC()

	// TimePtr should not be nil based on our test setup
	require.NotNil(t, expected.TimePtr, "TimePtr should not be nil in original")
	utcTime := expected.TimePtr.UTC()
	expected.TimePtr = &utcTime

	// Fix the any slice - numbers will be uint64
	// AnySliceVal should have exactly 3 elements based on our test setup
	require.Len(t, expected.AnySliceVal, 3, "AnySliceVal should have 3 elements")
	// Create a new slice to avoid modifying the original
	expected.AnySliceVal = make([]any, len(original.AnySliceVal))
	copy(expected.AnySliceVal, original.AnySliceVal)
	expected.AnySliceVal[1] = uint64(123)

	// AnySlicePtr should not be nil and should have 3 elements
	require.NotNil(t, expected.AnySlicePtr, "AnySlicePtr should not be nil")
	require.Len(t, *expected.AnySlicePtr, 3, "AnySlicePtr should point to slice with 3 elements")
	// Create a new slice to avoid modifying the original
	newSlice := make([]any, len(*original.AnySlicePtr))
	copy(newSlice, *original.AnySlicePtr)
	newSlice[1] = uint64(123)
	expected.AnySlicePtr = &newSlice

	// Fix the any map - numbers will be uint64
	// AnyMapVal should not be nil and should have the expected keys
	require.NotNil(t, expected.AnyMapVal, "AnyMapVal should not be nil")
	require.Contains(t, expected.AnyMapVal, "num", "AnyMapVal should contain 'num' key")
	// Create a new map to avoid modifying the original
	expected.AnyMapVal = make(map[string]any)
	for k, v := range original.AnyMapVal {
		expected.AnyMapVal[k] = v
	}
	expected.AnyMapVal["num"] = uint64(42)

	// AnyMapPtr should not be nil and should have the expected keys
	require.NotNil(t, expected.AnyMapPtr, "AnyMapPtr should not be nil")
	require.NotNil(t, *expected.AnyMapPtr, "AnyMapPtr should point to non-nil map")
	require.Contains(t, *expected.AnyMapPtr, "num", "AnyMapPtr map should contain 'num' key")
	// Create a new map to avoid modifying the original
	newMap := make(map[string]any)
	for k, v := range *original.AnyMapPtr {
		newMap[k] = v
	}
	newMap["num"] = uint64(42)
	expected.AnyMapPtr = &newMap

	// Future has private fields, so we can't compare values, only check pointer preservation
	// FuturePtr should not be nil based on our test setup
	require.NotNil(t, expected.FuturePtr, "FuturePtr should not be nil in original")
	require.NotNil(t, decoded.FuturePtr, "Future pointer should be preserved after decode")
	// Clear Future fields for comparison since we can't set private fields
	expected.FuturePtr = decoded.FuturePtr

	// FutureNilPtr should remain nil
	require.Nil(t, expected.FutureNilPtr, "FutureNilPtr should be nil in original")
	require.Nil(t, decoded.FutureNilPtr, "Nil Future pointer should remain nil after decode")

	// Ensure expected has been adjusted independently of original
	assert.NotEqual(t, expected, original, "Expected and original should not be equal")

	// Now compare
	assert.Equal(t, expected, decoded, "Complete struct mismatch after marshal/unmarshal")
}

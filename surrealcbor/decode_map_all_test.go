package surrealcbor

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_map_withAllSupportedTypes tests that all map value types
// can be properly marshaled and unmarshaled with:
// - Maps with primitive value types: map[string]Type
// - Maps with pointer value types: map[string]*Type with mix of non-nil and nil values
// - Maps with any values: map[string]any with different types and nil
// - Maps with custom struct values: map[string]CustomStruct
// - Maps with SurrealDB types as values
// - Nested maps and complex structures
func TestDecode_map_withAllSupportedTypes(t *testing.T) {
	type CustomStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type AllMapTypes struct {
		// ============ BOOLEAN MAPS ============
		BoolMap    map[string]bool  `json:"bool_map"`
		BoolPtrMap map[string]*bool `json:"bool_ptr_map"`

		// ============ STRING MAPS ============
		StringMap    map[string]string  `json:"string_map"`
		StringPtrMap map[string]*string `json:"string_ptr_map"`

		// ============ INTEGER MAPS ============
		IntMap      map[string]int    `json:"int_map"`
		IntPtrMap   map[string]*int   `json:"int_ptr_map"`
		Int8Map     map[string]int8   `json:"int8_map"`
		Int8PtrMap  map[string]*int8  `json:"int8_ptr_map"`
		Int16Map    map[string]int16  `json:"int16_map"`
		Int16PtrMap map[string]*int16 `json:"int16_ptr_map"`
		Int32Map    map[string]int32  `json:"int32_map"`
		Int32PtrMap map[string]*int32 `json:"int32_ptr_map"`
		Int64Map    map[string]int64  `json:"int64_map"`
		Int64PtrMap map[string]*int64 `json:"int64_ptr_map"`

		// ============ UNSIGNED INTEGER MAPS ============
		UintMap      map[string]uint    `json:"uint_map"`
		UintPtrMap   map[string]*uint   `json:"uint_ptr_map"`
		Uint8Map     map[string]uint8   `json:"uint8_map"`
		Uint8PtrMap  map[string]*uint8  `json:"uint8_ptr_map"`
		Uint16Map    map[string]uint16  `json:"uint16_map"`
		Uint16PtrMap map[string]*uint16 `json:"uint16_ptr_map"`
		Uint32Map    map[string]uint32  `json:"uint32_map"`
		Uint32PtrMap map[string]*uint32 `json:"uint32_ptr_map"`
		Uint64Map    map[string]uint64  `json:"uint64_map"`
		Uint64PtrMap map[string]*uint64 `json:"uint64_ptr_map"`

		// ============ FLOATING POINT MAPS ============
		Float32Map    map[string]float32  `json:"float32_map"`
		Float32PtrMap map[string]*float32 `json:"float32_ptr_map"`
		Float64Map    map[string]float64  `json:"float64_map"`
		Float64PtrMap map[string]*float64 `json:"float64_ptr_map"`

		// ============ BYTE SLICE MAPS ============
		ByteSliceMap    map[string][]byte  `json:"byte_slice_map"`
		ByteSlicePtrMap map[string]*[]byte `json:"byte_slice_ptr_map"`

		// ============ TIME MAPS ============
		TimeMap    map[string]time.Time  `json:"time_map"`
		TimePtrMap map[string]*time.Time `json:"time_ptr_map"`

		// ============ INTERFACE/ANY MAPS ============
		AnyMap    map[string]any  `json:"any_map"`
		AnyPtrMap map[string]*any `json:"any_ptr_map"`

		// ============ CUSTOM STRUCT MAPS ============
		StructMap    map[string]CustomStruct  `json:"struct_map"`
		StructPtrMap map[string]*CustomStruct `json:"struct_ptr_map"`

		// ============ NESTED MAPS ============
		MapOfMaps   map[string]map[string]int `json:"map_of_maps"`
		MapOfSlices map[string][]int          `json:"map_of_slices"`

		// ============ SURREALDB TYPE MAPS ============
		RecordIDMap          map[string]models.RecordID        `json:"record_id_map"`
		RecordIDPtrMap       map[string]*models.RecordID       `json:"record_id_ptr_map"`
		TableMap             map[string]models.Table           `json:"table_map"`
		TablePtrMap          map[string]*models.Table          `json:"table_ptr_map"`
		UUIDMap              map[string]models.UUID            `json:"uuid_map"`
		UUIDPtrMap           map[string]*models.UUID           `json:"uuid_ptr_map"`
		GeometryPointMap     map[string]models.GeometryPoint   `json:"geometry_point_map"`
		GeometryPointPtrMap  map[string]*models.GeometryPoint  `json:"geometry_point_ptr_map"`
		CustomDurationMap    map[string]models.CustomDuration  `json:"custom_duration_map"`
		CustomDurationPtrMap map[string]*models.CustomDuration `json:"custom_duration_ptr_map"`

		// ============ DIFFERENT KEY TYPES ============
		IntKeyMap    map[int]string    `json:"int_key_map"`
		Uint64KeyMap map[uint64]string `json:"uint64_key_map"`
	}

	// Create helper values for pointers
	boolTrue := true
	boolFalse := false
	str1 := "first"
	str2 := "second"
	int1 := 42
	int2 := 100
	int8Val1 := int8(8)
	int8Val2 := int8(16)
	int16Val1 := int16(16)
	int16Val2 := int16(32)
	int32Val1 := int32(32)
	int32Val2 := int32(64)
	int64Val1 := int64(64)
	int64Val2 := int64(128)
	uintVal1 := uint(100)
	uintVal2 := uint(200)
	uint8Val1 := uint8(8)
	uint8Val2 := uint8(16)
	uint16Val1 := uint16(16)
	uint16Val2 := uint16(32)
	uint32Val1 := uint32(32)
	uint32Val2 := uint32(64)
	uint64Val1 := uint64(64)
	uint64Val2 := uint64(128)
	float32Val1 := float32(3.14)
	float32Val2 := float32(2.71)
	float64Val1 := 3.14159
	float64Val2 := 2.71828

	bytes1 := []byte("hello")
	bytes2 := []byte("world")

	time1 := time.Now().Truncate(time.Second)
	time2 := time1.Add(time.Hour)

	anyVal1 := any("any string")
	anyVal2 := any(uint64(999)) // Use uint64 since CBOR decodes integers as uint64 in any

	struct1 := CustomStruct{ID: 1, Name: "First"}
	struct2 := CustomStruct{ID: 2, Name: "Second"}

	recordID1 := models.NewRecordID("users", "001")
	recordID2 := models.NewRecordID("users", "002")
	table1 := models.Table("table1")
	table2 := models.Table("table2")

	uuidVal1, _ := uuid.NewV4()
	uuid1 := models.UUID{UUID: uuidVal1}
	uuidVal2, _ := uuid.NewV4()
	uuid2 := models.UUID{UUID: uuidVal2}

	point1 := models.NewGeometryPoint(37.7749, -122.4194)
	point2 := models.NewGeometryPoint(40.7128, -74.0060)

	duration1 := models.CustomDuration{Duration: time.Hour}
	duration2 := models.CustomDuration{Duration: time.Hour * 2}

	// Create the original struct with all map types
	original := AllMapTypes{
		// Boolean maps
		BoolMap:    map[string]bool{"yes": true, "no": false},
		BoolPtrMap: map[string]*bool{"true": &boolTrue, "nil": nil, "false": &boolFalse},

		// String maps
		StringMap:    map[string]string{"key1": "value1", "key2": "value2"},
		StringPtrMap: map[string]*string{"first": &str1, "nil": nil, "second": &str2},

		// Integer maps
		IntMap:      map[string]int{"one": 1, "two": 2, "three": 3},
		IntPtrMap:   map[string]*int{"val1": &int1, "nil": nil, "val2": &int2},
		Int8Map:     map[string]int8{"a": 8, "b": 16},
		Int8PtrMap:  map[string]*int8{"val1": &int8Val1, "nil": nil, "val2": &int8Val2},
		Int16Map:    map[string]int16{"a": 16, "b": 32},
		Int16PtrMap: map[string]*int16{"val1": &int16Val1, "nil": nil, "val2": &int16Val2},
		Int32Map:    map[string]int32{"a": 32, "b": 64},
		Int32PtrMap: map[string]*int32{"val1": &int32Val1, "nil": nil, "val2": &int32Val2},
		Int64Map:    map[string]int64{"a": 64, "b": 128},
		Int64PtrMap: map[string]*int64{"val1": &int64Val1, "nil": nil, "val2": &int64Val2},

		// Unsigned integer maps
		UintMap:      map[string]uint{"a": 100, "b": 200},
		UintPtrMap:   map[string]*uint{"val1": &uintVal1, "nil": nil, "val2": &uintVal2},
		Uint8Map:     map[string]uint8{"a": 8, "b": 16},
		Uint8PtrMap:  map[string]*uint8{"val1": &uint8Val1, "nil": nil, "val2": &uint8Val2},
		Uint16Map:    map[string]uint16{"a": 16, "b": 32},
		Uint16PtrMap: map[string]*uint16{"val1": &uint16Val1, "nil": nil, "val2": &uint16Val2},
		Uint32Map:    map[string]uint32{"a": 32, "b": 64},
		Uint32PtrMap: map[string]*uint32{"val1": &uint32Val1, "nil": nil, "val2": &uint32Val2},
		Uint64Map:    map[string]uint64{"a": 64, "b": 128},
		Uint64PtrMap: map[string]*uint64{"val1": &uint64Val1, "nil": nil, "val2": &uint64Val2},

		// Float maps
		Float32Map:    map[string]float32{"pi": 3.14, "e": 2.71},
		Float32PtrMap: map[string]*float32{"val1": &float32Val1, "nil": nil, "val2": &float32Val2},
		Float64Map:    map[string]float64{"pi": 3.14159, "e": 2.71828},
		Float64PtrMap: map[string]*float64{"val1": &float64Val1, "nil": nil, "val2": &float64Val2},

		// Byte slice maps
		ByteSliceMap:    map[string][]byte{"greeting": []byte("hello"), "name": []byte("world")},
		ByteSlicePtrMap: map[string]*[]byte{"val1": &bytes1, "nil": nil, "val2": &bytes2},

		// Time maps
		TimeMap:    map[string]time.Time{"now": time1, "later": time2},
		TimePtrMap: map[string]*time.Time{"val1": &time1, "nil": nil, "val2": &time2},

		// Any maps
		AnyMap:    map[string]any{"string": "text", "number": 123, "bool": true, "nil": nil},
		AnyPtrMap: map[string]*any{"val1": &anyVal1, "nil": nil, "val2": &anyVal2},

		// Custom struct maps
		StructMap:    map[string]CustomStruct{"first": struct1, "second": struct2},
		StructPtrMap: map[string]*CustomStruct{"val1": &struct1, "nil": nil, "val2": &struct2},

		// Nested maps
		MapOfMaps: map[string]map[string]int{
			"outer1": {"inner1": 1, "inner2": 2},
			"outer2": {"inner3": 3, "inner4": 4},
		},
		MapOfSlices: map[string][]int{
			"list1": {1, 2, 3},
			"list2": {4, 5, 6},
		},

		// SurrealDB type maps
		RecordIDMap:          map[string]models.RecordID{"rec1": recordID1, "rec2": recordID2},
		RecordIDPtrMap:       map[string]*models.RecordID{"val1": &recordID1, "nil": nil, "val2": &recordID2},
		TableMap:             map[string]models.Table{"t1": table1, "t2": table2},
		TablePtrMap:          map[string]*models.Table{"val1": &table1, "nil": nil, "val2": &table2},
		UUIDMap:              map[string]models.UUID{"id1": uuid1, "id2": uuid2},
		UUIDPtrMap:           map[string]*models.UUID{"val1": &uuid1, "nil": nil, "val2": &uuid2},
		GeometryPointMap:     map[string]models.GeometryPoint{"sf": point1, "ny": point2},
		GeometryPointPtrMap:  map[string]*models.GeometryPoint{"val1": &point1, "nil": nil, "val2": &point2},
		CustomDurationMap:    map[string]models.CustomDuration{"short": duration1, "long": duration2},
		CustomDurationPtrMap: map[string]*models.CustomDuration{"val1": &duration1, "nil": nil, "val2": &duration2},

		// Different key types
		IntKeyMap:    map[int]string{1: "one", 2: "two", 3: "three"},
		Uint64KeyMap: map[uint64]string{100: "hundred", 200: "two hundred"},
	}

	// Marshal
	data, err := Marshal(original)
	require.NoError(t, err, "Marshal failed")

	// Unmarshal
	var decoded AllMapTypes
	err = Unmarshal(data, &decoded)
	require.NoError(t, err, "Unmarshal failed")

	// Create a copy for comparison with adjustments
	expected := original

	// Adjust time values to UTC
	expected.TimeMap = make(map[string]time.Time)
	for k, v := range original.TimeMap {
		expected.TimeMap[k] = v.UTC()
	}

	expected.TimePtrMap = make(map[string]*time.Time)
	for k, v := range original.TimePtrMap {
		if v != nil {
			utc := v.UTC()
			expected.TimePtrMap[k] = &utc
		} else {
			expected.TimePtrMap[k] = nil
		}
	}

	// Fix any map - numbers will be uint64
	expected.AnyMap = make(map[string]any)
	for k, v := range original.AnyMap {
		if num, ok := v.(int); ok {
			if num < 0 {
				require.FailNowf(t, "Negative number found in AnyMap", "key: %s, value: %d", k, num)
			}
			expected.AnyMap[k] = uint64(num)
		} else {
			expected.AnyMap[k] = v
		}
	}

	// Ensure original is intact while building the expected
	assert.NotEqual(t, original, expected, "Map struct mismatch after marshal/unmarshal")

	// Now compare
	assert.Equal(t, expected, decoded, "Map struct mismatch after marshal/unmarshal")

	// Additional validations for nil preservation in pointer maps
	require.Nil(t, decoded.BoolPtrMap["nil"], "BoolPtrMap['nil'] should be nil")
	require.Nil(t, decoded.StringPtrMap["nil"], "StringPtrMap['nil'] should be nil")
	require.Nil(t, decoded.IntPtrMap["nil"], "IntPtrMap['nil'] should be nil")
	require.Nil(t, decoded.TimePtrMap["nil"], "TimePtrMap['nil'] should be nil")
	require.Nil(t, decoded.AnyPtrMap["nil"], "AnyPtrMap['nil'] should be nil")
	require.Nil(t, decoded.StructPtrMap["nil"], "StructPtrMap['nil'] should be nil")
	require.Nil(t, decoded.RecordIDPtrMap["nil"], "RecordIDPtrMap['nil'] should be nil")
}

// TestMarshalUnmarshal_map_edgeCases tests edge cases for map handling
func TestMarshalUnmarshal_map_edgeCases(t *testing.T) {
	t.Run("empty maps", func(t *testing.T) {
		type EmptyMaps struct {
			IntMap    map[string]int    `json:"int_map"`
			StringMap map[string]string `json:"string_map"`
			AnyMap    map[string]any    `json:"any_map"`
		}

		original := EmptyMaps{
			IntMap:    map[string]int{},
			StringMap: map[string]string{},
			AnyMap:    map[string]any{},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded EmptyMaps
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
		assert.NotNil(t, decoded.IntMap, "Empty map should not be nil")
		assert.Len(t, decoded.IntMap, 0, "Empty map should have length 0")
	})

	t.Run("nil maps", func(t *testing.T) {
		type NilMaps struct {
			IntMap    map[string]int    `json:"int_map"`
			StringMap map[string]string `json:"string_map"`
			AnyMap    map[string]any    `json:"any_map"`
		}

		original := NilMaps{
			IntMap:    nil,
			StringMap: nil,
			AnyMap:    nil,
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded NilMaps
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
		assert.Nil(t, decoded.IntMap, "Nil map should remain nil")
		assert.Nil(t, decoded.StringMap, "Nil map should remain nil")
		assert.Nil(t, decoded.AnyMap, "Nil map should remain nil")
	})

	t.Run("deeply nested maps", func(t *testing.T) {
		type NestedMaps struct {
			ThreeLevels map[string]map[string]map[string]int `json:"three_levels"`
		}

		original := NestedMaps{
			ThreeLevels: map[string]map[string]map[string]int{
				"level1": {
					"level2a": {"level3a": 1, "level3b": 2},
					"level2b": {"level3c": 3, "level3d": 4},
				},
				"level1b": {
					"level2c": {"level3e": 5, "level3f": 6},
				},
			},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded NestedMaps
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
	})

	t.Run("map with None values", func(t *testing.T) {
		// When we encode None values in a map, they should decode as nil
		em := getEncMode()
		data, err := em.Marshal(map[string]any{
			"string": "text",
			"none1":  models.None,
			"number": 123,
			"none2":  models.None,
		})
		require.NoError(t, err)

		var decoded map[string]any
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		require.Len(t, decoded, 4)
		assert.Equal(t, "text", decoded["string"])
		assert.Nil(t, decoded["none1"], "None should decode to nil")
		assert.Equal(t, uint64(123), decoded["number"])
		assert.Nil(t, decoded["none2"], "None should decode to nil")
	})

	t.Run("maps with complex keys", func(t *testing.T) {
		type ComplexKeyMaps struct {
			FloatKeyMap map[float64]string `json:"float_key_map"`
			BoolKeyMap  map[bool]string    `json:"bool_key_map"`
		}

		original := ComplexKeyMaps{
			FloatKeyMap: map[float64]string{
				3.14: "pi",
				2.71: "e",
			},
			BoolKeyMap: map[bool]string{
				true:  "yes",
				false: "no",
			},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded ComplexKeyMaps
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
	})
}

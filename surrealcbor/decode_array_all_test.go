package surrealcbor

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_array_withAllSupportedTypes tests that all slice types
// can be properly marshaled and unmarshaled with:
// - For primitive types: (1) []Type with values, (2) []*Type with mix of non-nil and nil values
// - For any types: []any and []*any with mix of different types and nil values
// - For custom structs: []CustomStruct and []*CustomStruct
// - For SurrealDB types: slices of RecordID, Table, UUID, etc.
func TestDecode_array_withAllSupportedTypes(t *testing.T) {
	type CustomStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type AllSliceTypes struct {
		// ============ BOOLEAN SLICES ============
		BoolSlice    []bool  `json:"bool_slice"`
		BoolPtrSlice []*bool `json:"bool_ptr_slice"`

		// ============ STRING SLICES ============
		StringSlice    []string  `json:"string_slice"`
		StringPtrSlice []*string `json:"string_ptr_slice"`

		// ============ INTEGER SLICES ============
		IntSlice      []int    `json:"int_slice"`
		IntPtrSlice   []*int   `json:"int_ptr_slice"`
		Int8Slice     []int8   `json:"int8_slice"`
		Int8PtrSlice  []*int8  `json:"int8_ptr_slice"`
		Int16Slice    []int16  `json:"int16_slice"`
		Int16PtrSlice []*int16 `json:"int16_ptr_slice"`
		Int32Slice    []int32  `json:"int32_slice"`
		Int32PtrSlice []*int32 `json:"int32_ptr_slice"`
		Int64Slice    []int64  `json:"int64_slice"`
		Int64PtrSlice []*int64 `json:"int64_ptr_slice"`

		// ============ UNSIGNED INTEGER SLICES ============
		UintSlice      []uint    `json:"uint_slice"`
		UintPtrSlice   []*uint   `json:"uint_ptr_slice"`
		Uint8Slice     []uint8   `json:"uint8_slice"`
		Uint8PtrSlice  []*uint8  `json:"uint8_ptr_slice"`
		Uint16Slice    []uint16  `json:"uint16_slice"`
		Uint16PtrSlice []*uint16 `json:"uint16_ptr_slice"`
		Uint32Slice    []uint32  `json:"uint32_slice"`
		Uint32PtrSlice []*uint32 `json:"uint32_ptr_slice"`
		Uint64Slice    []uint64  `json:"uint64_slice"`
		Uint64PtrSlice []*uint64 `json:"uint64_ptr_slice"`

		// ============ FLOATING POINT SLICES ============
		Float32Slice    []float32  `json:"float32_slice"`
		Float32PtrSlice []*float32 `json:"float32_ptr_slice"`
		Float64Slice    []float64  `json:"float64_slice"`
		Float64PtrSlice []*float64 `json:"float64_ptr_slice"`

		// ============ BYTE SLICE (special case) ============
		ByteSlice    []byte  `json:"byte_slice"`
		BytePtrSlice []*byte `json:"byte_ptr_slice"`

		// ============ TIME SLICES ============
		TimeSlice    []time.Time  `json:"time_slice"`
		TimePtrSlice []*time.Time `json:"time_ptr_slice"`

		// ============ INTERFACE/ANY SLICES ============
		AnySlice    []any  `json:"any_slice"`
		AnyPtrSlice []*any `json:"any_ptr_slice"`

		// ============ CUSTOM STRUCT SLICES ============
		StructSlice    []CustomStruct  `json:"struct_slice"`
		StructPtrSlice []*CustomStruct `json:"struct_ptr_slice"`

		// ============ NESTED SLICES ============
		SliceOfSlices [][]int          `json:"slice_of_slices"`
		SliceOfMaps   []map[string]int `json:"slice_of_maps"`

		// ============ SURREALDB TYPE SLICES ============
		RecordIDSlice          []models.RecordID        `json:"record_id_slice"`
		RecordIDPtrSlice       []*models.RecordID       `json:"record_id_ptr_slice"`
		TableSlice             []models.Table           `json:"table_slice"`
		TablePtrSlice          []*models.Table          `json:"table_ptr_slice"`
		UUIDSlice              []models.UUID            `json:"uuid_slice"`
		UUIDPtrSlice           []*models.UUID           `json:"uuid_ptr_slice"`
		GeometryPointSlice     []models.GeometryPoint   `json:"geometry_point_slice"`
		GeometryPointPtrSlice  []*models.GeometryPoint  `json:"geometry_point_ptr_slice"`
		CustomDurationSlice    []models.CustomDuration  `json:"custom_duration_slice"`
		CustomDurationPtrSlice []*models.CustomDuration `json:"custom_duration_ptr_slice"`
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
	byte1 := byte(0xAB)
	byte2 := byte(0xCD)
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

	point1 := models.GeometryPoint{
		Latitude:  37.7749,
		Longitude: -122.4194,
	}
	point2 := models.GeometryPoint{
		Latitude:  40.7128,
		Longitude: -74.0060,
	}

	duration1 := models.CustomDuration{Duration: time.Hour}
	duration2 := models.CustomDuration{Duration: time.Hour * 2}

	// Create the original struct with all slice types
	original := AllSliceTypes{
		// Boolean slices
		BoolSlice:    []bool{true, false, true},
		BoolPtrSlice: []*bool{&boolTrue, nil, &boolFalse},

		// String slices
		StringSlice:    []string{"hello", "world", "test"},
		StringPtrSlice: []*string{&str1, nil, &str2},

		// Integer slices
		IntSlice:      []int{1, 2, 3},
		IntPtrSlice:   []*int{&int1, nil, &int2},
		Int8Slice:     []int8{8, 16, 24},
		Int8PtrSlice:  []*int8{&int8Val1, nil, &int8Val2},
		Int16Slice:    []int16{16, 32, 48},
		Int16PtrSlice: []*int16{&int16Val1, nil, &int16Val2},
		Int32Slice:    []int32{32, 64, 96},
		Int32PtrSlice: []*int32{&int32Val1, nil, &int32Val2},
		Int64Slice:    []int64{64, 128, 192},
		Int64PtrSlice: []*int64{&int64Val1, nil, &int64Val2},

		// Unsigned integer slices
		UintSlice:      []uint{100, 200, 300},
		UintPtrSlice:   []*uint{&uintVal1, nil, &uintVal2},
		Uint8Slice:     []uint8{8, 16, 24},
		Uint8PtrSlice:  []*uint8{&uint8Val1, nil, &uint8Val2},
		Uint16Slice:    []uint16{16, 32, 48},
		Uint16PtrSlice: []*uint16{&uint16Val1, nil, &uint16Val2},
		Uint32Slice:    []uint32{32, 64, 96},
		Uint32PtrSlice: []*uint32{&uint32Val1, nil, &uint32Val2},
		Uint64Slice:    []uint64{64, 128, 192},
		Uint64PtrSlice: []*uint64{&uint64Val1, nil, &uint64Val2},

		// Float slices
		Float32Slice:    []float32{1.1, 2.2, 3.3},
		Float32PtrSlice: []*float32{&float32Val1, nil, &float32Val2},
		Float64Slice:    []float64{1.11, 2.22, 3.33},
		Float64PtrSlice: []*float64{&float64Val1, nil, &float64Val2},

		// Byte slices
		ByteSlice:    []byte{0xAA, 0xBB, 0xCC},
		BytePtrSlice: []*byte{&byte1, nil, &byte2},

		// Time slices
		TimeSlice:    []time.Time{time1, time2},
		TimePtrSlice: []*time.Time{&time1, nil, &time2},

		// Any slices
		AnySlice:    []any{"string", 123, true, nil},
		AnyPtrSlice: []*any{&anyVal1, nil, &anyVal2},

		// Custom struct slices
		StructSlice:    []CustomStruct{struct1, struct2},
		StructPtrSlice: []*CustomStruct{&struct1, nil, &struct2},

		// Nested slices
		SliceOfSlices: [][]int{{1, 2}, {3, 4}, {5, 6}},
		SliceOfMaps: []map[string]int{
			{"a": 1, "b": 2},
			{"c": 3, "d": 4},
		},

		// SurrealDB type slices
		RecordIDSlice:          []models.RecordID{recordID1, recordID2},
		RecordIDPtrSlice:       []*models.RecordID{&recordID1, nil, &recordID2},
		TableSlice:             []models.Table{table1, table2},
		TablePtrSlice:          []*models.Table{&table1, nil, &table2},
		UUIDSlice:              []models.UUID{uuid1, uuid2},
		UUIDPtrSlice:           []*models.UUID{&uuid1, nil, &uuid2},
		GeometryPointSlice:     []models.GeometryPoint{point1, point2},
		GeometryPointPtrSlice:  []*models.GeometryPoint{&point1, nil, &point2},
		CustomDurationSlice:    []models.CustomDuration{duration1, duration2},
		CustomDurationPtrSlice: []*models.CustomDuration{&duration1, nil, &duration2},
	}

	// Marshal
	data, err := Marshal(original)
	require.NoError(t, err, "Marshal failed")

	// Unmarshal
	var decoded AllSliceTypes
	err = Unmarshal(data, &decoded)
	require.NoError(t, err, "Unmarshal failed")

	// Create a copy for comparison with adjustments
	expected := original

	// Adjust time values to UTC
	for i := range expected.TimeSlice {
		expected.TimeSlice[i] = expected.TimeSlice[i].UTC()
	}
	for i, ptr := range expected.TimePtrSlice {
		if ptr != nil {
			utc := ptr.UTC()
			expected.TimePtrSlice[i] = &utc
		}
	}

	// Fix any slice - numbers will be uint64
	require.Len(t, expected.AnySlice, 4, "AnySlice should have 4 elements")
	expected.AnySlice = make([]any, len(original.AnySlice))
	copy(expected.AnySlice, original.AnySlice)
	expected.AnySlice[1] = uint64(123) // Convert int to uint64

	// Ensure original is intact while building the expected
	assert.NotEqual(t, expected, original)

	// Now compare
	assert.Equal(t, expected, decoded, "Slice struct mismatch after marshal/unmarshal")

	// Additional validations for nil preservation in pointer slices
	require.Nil(t, decoded.BoolPtrSlice[1], "BoolPtrSlice[1] should be nil")
	require.Nil(t, decoded.StringPtrSlice[1], "StringPtrSlice[1] should be nil")
	require.Nil(t, decoded.IntPtrSlice[1], "IntPtrSlice[1] should be nil")
	require.Nil(t, decoded.TimePtrSlice[1], "TimePtrSlice[1] should be nil")
	require.Nil(t, decoded.AnyPtrSlice[1], "AnyPtrSlice[1] should be nil")
	require.Nil(t, decoded.StructPtrSlice[1], "StructPtrSlice[1] should be nil")
	require.Nil(t, decoded.RecordIDPtrSlice[1], "RecordIDPtrSlice[1] should be nil")
}

// TestMarshalUnmarshal_slice_edgeCases tests edge cases for slice handling
func TestMarshalUnmarshal_slice_edgeCases(t *testing.T) {
	t.Run("empty slices", func(t *testing.T) {
		type EmptySlices struct {
			IntSlice    []int    `json:"int_slice"`
			StringSlice []string `json:"string_slice"`
			AnySlice    []any    `json:"any_slice"`
		}

		original := EmptySlices{
			IntSlice:    []int{},
			StringSlice: []string{},
			AnySlice:    []any{},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded EmptySlices
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
		assert.NotNil(t, decoded.IntSlice, "Empty slice should not be nil")
		assert.Len(t, decoded.IntSlice, 0, "Empty slice should have length 0")
	})

	t.Run("nil slices", func(t *testing.T) {
		type NilSlices struct {
			IntSlice    []int    `json:"int_slice"`
			StringSlice []string `json:"string_slice"`
			AnySlice    []any    `json:"any_slice"`
		}

		original := NilSlices{
			IntSlice:    nil,
			StringSlice: nil,
			AnySlice:    nil,
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded NilSlices
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
		assert.Nil(t, decoded.IntSlice, "Nil slice should remain nil")
		assert.Nil(t, decoded.StringSlice, "Nil slice should remain nil")
		assert.Nil(t, decoded.AnySlice, "Nil slice should remain nil")
	})

	t.Run("deeply nested slices", func(t *testing.T) {
		type NestedSlices struct {
			ThreeDimensional [][][]int `json:"three_dimensional"`
		}

		original := NestedSlices{
			ThreeDimensional: [][][]int{
				{{1, 2}, {3, 4}},
				{{5, 6}, {7, 8}},
			},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded NestedSlices
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
	})

	t.Run("slice with None values", func(t *testing.T) {
		// When we encode None values in a slice, they should decode as nil
		em := getEncMode()
		data, err := em.Marshal([]any{
			"string",
			models.None,
			123,
			models.None,
		})
		require.NoError(t, err)

		var decoded []any
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		require.Len(t, decoded, 4)
		assert.Equal(t, "string", decoded[0])
		assert.Nil(t, decoded[1], "None should decode to nil")
		assert.Equal(t, uint64(123), decoded[2])
		assert.Nil(t, decoded[3], "None should decode to nil")
	})
}

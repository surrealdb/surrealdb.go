package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecode_map_struct(t *testing.T) {
	t.Run("decode struct with unexported field", func(t *testing.T) {
		type TestStruct struct {
			Exported   string `json:"exported"`
			unexported string // No json tag for unexported field
		}

		data, _ := cbor.Marshal(map[string]string{
			"exported":   "value1",
			"unexported": "value2",
		})

		var s TestStruct
		err := Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Equal(t, "value1", s.Exported)
		assert.Equal(t, "", s.unexported) // Should not be set
	})
}

// TestDecode_map_structFieldName tests the field name fallback behavior
// that aligns with fxamacker/cbor.
func TestDecode_map_structFieldName(t *testing.T) {
	t.Run("field name fallback when tag doesn't match", func(t *testing.T) {
		type TestStruct struct {
			FieldName string `json:"fieldname"`
		}

		// Encode with field name (not matching json tag)
		data, err := cbor.Marshal(map[string]string{
			"FieldName": "value", // Matches field name, not json tag
		})
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		// Should match via field name fallback (case-sensitive)
		assert.Equal(t, "value", decoded.FieldName)
	})

	t.Run("exact match preferred", func(t *testing.T) {
		type TestStruct struct {
			Field     string `json:"field"`
			FieldName string `json:"fieldname"`
		}

		data, err := cbor.Marshal(map[string]string{
			"field":     "exact",
			"FieldName": "field-name-match",
		})
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "exact", decoded.Field)
		assert.Equal(t, "field-name-match", decoded.FieldName)
	})

	t.Run("case sensitive matching", func(t *testing.T) {
		type TestStruct struct {
			FieldName string `json:"fieldname"`
		}

		// Test that different cases don't match the json tag
		data, err := cbor.Marshal(map[string]string{
			"FIELDNAME": "uppercase",
			"Fieldname": "mixed-case",
			"fieldname": "lowercase-exact",
		})
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)
		// Only the exact lowercase match should work
		assert.Equal(t, "lowercase-exact", decoded.FieldName)
	})
}

// TestDecode_map_structNoneToNil tests that CBOR Tag 6 (NONE) is unmarshaled as Go nil
func TestDecode_map_structNoneToNil(t *testing.T) {
	t.Run("pointer field with None becomes nil", func(t *testing.T) {
		type TestStruct struct {
			Name  string  `json:"name"`
			Value *string `json:"value"`
		}

		// Use fxamacker to encode with None
		em := getEncMode()
		data, err := em.Marshal(map[string]any{
			"name":  "test",
			"value": models.None,
		})
		require.NoError(t, err, "Marshal failed")

		// Unmarshal using our decoder
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Equal(t, "test", decoded.Name, "Name mismatch")
		assert.Nil(t, decoded.Value, "Value should be nil")
	})

	t.Run("interface field with None becomes nil", func(t *testing.T) {
		type TestStruct struct {
			Data any `json:"data"`
		}

		// Encode with None
		em := getEncMode()
		data, err := em.Marshal(map[string]any{
			"data": models.None,
		})
		require.NoError(t, err, "Marshal failed")

		// Unmarshal
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Nil(t, decoded.Data, "Data should be nil")
	})

	t.Run("slice field with None becomes nil", func(t *testing.T) {
		type TestStruct struct {
			Items []string `json:"items"`
		}

		// Encode with None
		em := getEncMode()
		data, err := em.Marshal(map[string]any{
			"items": models.None,
		})
		require.NoError(t, err, "Marshal failed")

		// Unmarshal
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Nil(t, decoded.Items, "Items should be nil")
	})

	t.Run("map field with None becomes nil", func(t *testing.T) {
		type TestStruct struct {
			Meta map[string]string `json:"meta"`
		}

		// Encode with None
		em := getEncMode()
		data, err := em.Marshal(map[string]any{
			"meta": models.None,
		})
		require.NoError(t, err, "Marshal failed")

		// Unmarshal
		var decoded TestStruct
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Nil(t, decoded.Meta, "Meta should be nil")
	})
}

// TestDecode_map_emptyNilNone tests the decoding of maps with empty, nil, and None values
func TestDecode_map_emptyNilNone(t *testing.T) {
	t.Run("nil pointer preservation", func(t *testing.T) {
		type NilTest struct {
			StringPtr *string            `json:"string_ptr"`
			IntPtr    *int               `json:"int_ptr"`
			SlicePtr  *[]int             `json:"slice_ptr"`
			MapPtr    *map[string]string `json:"map_ptr"`
		}

		original := NilTest{
			StringPtr: nil,
			IntPtr:    nil,
			SlicePtr:  nil,
			MapPtr:    nil,
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded NilTest
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded, "Nil pointers should be preserved")
		assert.Nil(t, decoded.StringPtr)
		assert.Nil(t, decoded.IntPtr)
		assert.Nil(t, decoded.SlicePtr)
		assert.Nil(t, decoded.MapPtr)
	})

	t.Run("empty vs nil slices", func(t *testing.T) {
		type SliceTest struct {
			NilSlice   []int `json:"nil_slice"`
			EmptySlice []int `json:"empty_slice"`
		}

		original := SliceTest{
			NilSlice:   nil,
			EmptySlice: []int{},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded SliceTest
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		// Note: CBOR might not preserve the distinction between nil and empty slice
		// This is a known limitation of CBOR encoding
		assert.Nil(t, decoded.NilSlice, "Nil slice should remain nil")
		assert.NotNil(t, decoded.EmptySlice, "Empty slice should not be nil")
		assert.Len(t, decoded.EmptySlice, 0, "Empty slice should have length 0")
	})

	t.Run("empty vs nil maps", func(t *testing.T) {
		type MapTest struct {
			NilMap   map[string]int `json:"nil_map"`
			EmptyMap map[string]int `json:"empty_map"`
		}

		original := MapTest{
			NilMap:   nil,
			EmptyMap: map[string]int{},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded MapTest
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		// Similar to slices, CBOR might not preserve nil vs empty distinction
		assert.Nil(t, decoded.NilMap, "Nil map should remain nil")
		assert.NotNil(t, decoded.EmptyMap, "Empty map should not be nil")
		assert.Len(t, decoded.EmptyMap, 0, "Empty map should have length 0")
	})

	t.Run("None to nil conversion", func(t *testing.T) {
		type NoneTest struct {
			StringPtr *string          `json:"string_ptr"`
			IntPtr    *int             `json:"int_ptr"`
			NoneVal   models.CustomNil `json:"none_val"`
		}

		// When we marshal with None values, they should unmarshal as nil
		em := getEncMode()
		data, err := em.Marshal(map[string]any{
			"string_ptr": models.None,
			"int_ptr":    models.None,
			"none_val":   models.None,
		})
		require.NoError(t, err)

		var decoded NoneTest
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Nil(t, decoded.StringPtr, "None should decode to nil for *string")
		assert.Nil(t, decoded.IntPtr, "None should decode to nil for *int")
	})
}

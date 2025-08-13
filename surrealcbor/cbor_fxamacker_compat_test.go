package surrealcbor

import (
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestCompatibilityWithFxamacker_types tests that data encoded with fxamacker can be decoded with our implementation
func TestCompatibilityWithFxamacker_types(t *testing.T) {
	// Create test data
	testData := map[string]any{
		"string": "hello",
		"number": 42,
		"float":  3.14,
		"bool":   true,
		"null":   nil,
		"array":  []any{1, 2, 3},
		"object": map[string]any{"key": "value"},
		"table":  models.Table("users"),
		"record": models.NewRecordID("users", 123),
	}

	// Encode with fxamacker
	em := getEncMode()
	fxData, err := em.Marshal(testData)
	require.NoError(t, err, "fxamacker Marshal failed")

	// Decode with our implementation
	var ourDecoded map[string]any
	err = Unmarshal(fxData, &ourDecoded)
	require.NoError(t, err, "Our Unmarshal failed")

	// Verify basic types
	assert.Equal(t, "hello", ourDecoded["string"], "String mismatch")

	// Handle both int64 and uint64 for number comparison
	switch v := ourDecoded["number"].(type) {
	case int64:
		assert.Equal(t, int64(42), v, "Number mismatch")
	case uint64:
		assert.Equal(t, uint64(42), v, "Number mismatch")
	default:
		t.Errorf("Number type mismatch: got %T, want int64 or uint64", ourDecoded["number"])
	}

	assert.Equal(t, true, ourDecoded["bool"], "Bool mismatch")
	assert.Nil(t, ourDecoded["null"], "Null should be nil")

	// Test that we can also encode with our Marshal and decode with fxamacker (except for None)
	ourData, err := Marshal(map[string]any{
		"test": "value",
		"num":  100,
	})
	require.NoError(t, err, "Our Marshal failed")

	dm, _ := cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any(nil)),
	}.DecMode()

	var fxDecoded map[string]any
	err = dm.Unmarshal(ourData, &fxDecoded)
	require.NoError(t, err, "fxamacker Unmarshal of our data failed")

	assert.Equal(t, "value", fxDecoded["test"], "Test value mismatch")
}

// TestCompatibilityWithFxamacker_fieldResolver verifies how fxamacker/cbor handles different field tags
// This serves as a reference for our field resolver behavior
func TestCompatibilityWithFxamacker_fieldResolver(t *testing.T) {
	// Create reusable marshaler and unmarshaler
	marshaler := &models.CborMarshaler{}
	unmarshaler := &models.CborUnmarshaler{}
	t.Run("empty json tag", func(t *testing.T) {
		type EmptyTag struct {
			Field1 string `json:""`
			Field2 int
		}

		original := EmptyTag{
			Field1: "test",
			Field2: 42,
		}

		// Test with fxamacker/cbor via pkg/models
		encoded, err := marshaler.Marshal(original)
		require.NoError(t, err)

		var decoded EmptyTag
		err = unmarshaler.Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		// With empty tag, the field should still be included
		assert.Equal(t, original, decoded)

		// Test with our implementation
		var ourDecoded EmptyTag
		err = Unmarshal(encoded, &ourDecoded)
		require.NoError(t, err)
		assert.Equal(t, original, ourDecoded)
	})

	t.Run("dash json tag", func(t *testing.T) {
		type DashTag struct {
			Field1 string `json:"-"`
			Field2 int
		}

		original := DashTag{
			Field1: "test",
			Field2: 42,
		}

		// Test with fxamacker/cbor
		encoded, err := marshaler.Marshal(original)
		require.NoError(t, err)

		var decoded DashTag
		err = unmarshaler.Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		// With dash tag, Field1 should be omitted during encoding
		assert.Equal(t, "", decoded.Field1) // Field1 should be zero value
		assert.Equal(t, original.Field2, decoded.Field2)

		// Test with our implementation
		var ourDecoded DashTag
		err = Unmarshal(encoded, &ourDecoded)
		require.NoError(t, err)
		assert.Equal(t, "", ourDecoded.Field1)
		assert.Equal(t, original.Field2, ourDecoded.Field2)
	})

	t.Run("omitempty json tag", func(t *testing.T) {
		type OmitEmptyTag struct {
			Field1 string `json:",omitempty"`
			Field2 string `json:",omitempty"`
			Field3 int    `json:",omitempty"`
			Field4 int    `json:",omitempty"`
		}

		// Test with non-empty values
		original := OmitEmptyTag{
			Field1: "test",
			Field2: "", // empty, should be omitted
			Field3: 42,
			Field4: 0, // zero, should be omitted
		}

		// Test with fxamacker/cbor
		encoded, err := marshaler.Marshal(original)
		require.NoError(t, err)

		var decoded OmitEmptyTag
		err = unmarshaler.Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		// Only non-zero fields should be present
		assert.Equal(t, "test", decoded.Field1)
		assert.Equal(t, "", decoded.Field2) // zero value
		assert.Equal(t, 42, decoded.Field3)
		assert.Equal(t, 0, decoded.Field4) // zero value

		// Test with our implementation
		var ourDecoded OmitEmptyTag
		err = Unmarshal(encoded, &ourDecoded)
		require.NoError(t, err)
		assert.Equal(t, decoded, ourDecoded)
	})

	t.Run("named field with omitempty", func(t *testing.T) {
		type NamedOmitEmpty struct {
			Field1 string `json:"field_one,omitempty"`
			Field2 string `json:"field_two,omitempty"`
		}

		original := NamedOmitEmpty{
			Field1: "test",
			Field2: "", // empty, should be omitted during encoding
		}

		// Test with fxamacker/cbor
		encoded, err := marshaler.Marshal(original)
		require.NoError(t, err)

		var decoded NamedOmitEmpty
		err = unmarshaler.Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "test", decoded.Field1)
		assert.Equal(t, "", decoded.Field2)

		// Test with our implementation
		var ourDecoded NamedOmitEmpty
		err = Unmarshal(encoded, &ourDecoded)
		require.NoError(t, err)
		assert.Equal(t, decoded, ourDecoded)
	})

	t.Run("field name resolution priority", func(t *testing.T) {
		// Test how fxamacker/cbor resolves field names when decoding
		type Priority struct {
			Field1 string `cbor:"cbor_name" json:"json_name"`
			Field2 string `json:"json_only"`
			Field3 string // no tags
		}

		// Create a map with different field names to test resolution
		testCases := []struct {
			name              string
			inputMap          map[string]any
			expectedFxamacker Priority
			expectedOurs      Priority
		}{
			{
				name: "cbor tag takes precedence",
				inputMap: map[string]any{
					"cbor_name": "value1",
					"Field2":    "value2",
					"Field3":    "value3",
				},
				expectedFxamacker: Priority{
					Field1: "value1",
					Field2: "", // fxamacker doesn't fallback to field name when tag exists
					Field3: "value3",
				},
				expectedOurs: Priority{
					Field1: "value1",
					Field2: "", // Now matches fxamacker - no fallback when tag exists
					Field3: "value3",
				},
			},
			{
				name: "json tag used when no cbor tag",
				inputMap: map[string]any{
					"Field1":    "value1", // Field name (Field1 has tags, won't match)
					"json_only": "value2",
					"Field3":    "value3",
				},
				expectedFxamacker: Priority{
					Field1: "", // fxamacker doesn't fallback when tags exist
					Field2: "value2",
					Field3: "value3",
				},
				expectedOurs: Priority{
					Field1: "", // Now matches fxamacker - no fallback when tags exist
					Field2: "value2",
					Field3: "value3",
				},
			},
			{
				name: "field name fallback behavior",
				inputMap: map[string]any{
					"Field1": "value1",
					"Field2": "value2",
					"Field3": "value3",
				},
				// This is where the difference is most clear
				expectedFxamacker: Priority{
					Field1: "", // fxamacker: no match because Field1 has tags
					Field2: "", // fxamacker: no match because Field2 has tags
					Field3: "value3",
				},
				expectedOurs: Priority{
					Field1: "", // Now matches fxamacker - no fallback
					Field2: "", // Now matches fxamacker - no fallback
					Field3: "value3",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Encode the map with fxamacker/cbor
				encoded, err := marshaler.Marshal(tc.inputMap)
				require.NoError(t, err)

				// Decode with fxamacker/cbor
				var decoded Priority
				err = unmarshaler.Unmarshal(encoded, &decoded)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedFxamacker, decoded, "fxamacker/cbor decoding")

				// Decode with our implementation
				var ourDecoded Priority
				err = Unmarshal(encoded, &ourDecoded)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedOurs, ourDecoded, "our implementation")
			})
		}
	})

	t.Run("field name conflicts", func(t *testing.T) {
		// Test what happens when a field's tag matches another field's name
		type Conflict struct {
			Field1 string // no tag
			Field2 string `json:"Field1"` // tag conflicts with Field1's name
		}

		testCases := []struct {
			name              string
			inputMap          map[string]any
			expectedFxamacker Conflict
			expectedOurs      Conflict
			desc              string
		}{
			{
				name: "tag matches another field name",
				inputMap: map[string]any{
					"Field1": "value1",
				},
				expectedFxamacker: Conflict{
					Field1: "",       // Field1 has no tag, doesn't match
					Field2: "value1", // Field2's tag "Field1" matches
				},
				expectedOurs: Conflict{
					Field1: "",       // Same as fxamacker
					Field2: "value1", // Same as fxamacker
				},
				desc: "Both implementations agree: tag takes precedence",
			},
			{
				name: "both field names present",
				inputMap: map[string]any{
					"Field1": "value1",
					"Field2": "value2",
				},
				expectedFxamacker: Conflict{
					Field1: "",       // Field1 still doesn't get value
					Field2: "value1", // Field2's tag wins over Field2 name
				},
				expectedOurs: Conflict{
					Field1: "",       // Field1 doesn't get value
					Field2: "value1", // Now matches fxamacker - tag takes precedence
				},
				desc: "Both implementations now agree: tag always takes precedence",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Encode the map with fxamacker/cbor
				encoded, err := marshaler.Marshal(tc.inputMap)
				require.NoError(t, err)

				// Decode with fxamacker/cbor
				var decodedFx Conflict
				err = unmarshaler.Unmarshal(encoded, &decodedFx)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedFxamacker, decodedFx, "fxamacker/cbor result")

				// Decode with our implementation
				var decodedOur Conflict
				err = Unmarshal(encoded, &decodedOur)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedOurs, decodedOur, "our implementation result")
			})
		}
	})

	t.Run("tag precedence conflicts", func(t *testing.T) {
		// More complex scenario with multiple conflicts
		type ComplexConflict struct {
			Name     string `json:"title"` // Field Name, tag "title"
			Title    string `json:"name"`  // Field Title, tag "name"
			Untitled string // Field Untitled, no tag
		}

		// NOTE: We don't test cases where both exact and case-insensitive matches
		// exist (e.g., both "title" and "Title" as keys) because the result depends
		// on the non-deterministic iteration order of Go maps during encoding.

		testCases := []struct {
			name             string
			inputMap         map[string]any
			expectedName     string
			expectedTitle    string
			expectedUntitled string
		}{
			{
				name: "cross-referenced tags",
				inputMap: map[string]any{
					"name":  "name-value",
					"title": "title-value",
				},
				// Tags are swapped: Name has tag "title", Title has tag "name"
				expectedName:     "title-value", // Name field gets value from "title" key
				expectedTitle:    "name-value",  // Title field gets value from "name" key
				expectedUntitled: "",            // No value for Untitled
			},
			{
				name: "field names matching",
				inputMap: map[string]any{
					"Name":  "Name-value",
					"Title": "Title-value",
				},
				// Case-insensitive tag matching applies
				expectedName:     "Title-value", // "Title" matches tag "title" case-insensitively
				expectedTitle:    "Name-value",  // "Name" matches tag "name" case-insensitively
				expectedUntitled: "",            // No value for Untitled
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Encode the map
				encoded, err := marshaler.Marshal(tc.inputMap)
				require.NoError(t, err)

				// Decode with fxamacker/cbor
				var decodedFx ComplexConflict
				err = unmarshaler.Unmarshal(encoded, &decodedFx)
				require.NoError(t, err)

				// Decode with our implementation
				var decodedOur ComplexConflict
				err = Unmarshal(encoded, &decodedOur)
				require.NoError(t, err)

				// Assert expected values for fxamacker
				assert.Equal(t, tc.expectedName, decodedFx.Name, "fxamacker Name field")
				assert.Equal(t, tc.expectedTitle, decodedFx.Title, "fxamacker Title field")
				assert.Equal(t, tc.expectedUntitled, decodedFx.Untitled, "fxamacker Untitled field")

				// Assert our implementation matches fxamacker
				assert.Equal(t, decodedFx, decodedOur, "our implementation should match fxamacker")
			})
		}
	})

	t.Run("case sensitivity", func(t *testing.T) {
		type CaseSensitive struct {
			FieldOne string `json:"fieldone"`
			FieldTwo string
		}

		// Test with exact match
		exactMatch := map[string]any{
			"fieldone": "value1",
			"FieldTwo": "value2",
		}

		encoded, err := marshaler.Marshal(exactMatch)
		require.NoError(t, err)

		var decoded CaseSensitive
		err = unmarshaler.Unmarshal(encoded, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "value1", decoded.FieldOne)
		assert.Equal(t, "value2", decoded.FieldTwo)

		// Test with case mismatch - fxamacker/cbor behavior
		caseMismatch := map[string]any{
			"FieldOne": "value1", // Wrong case for tag
			"fieldtwo": "value2", // Wrong case for field name
		}

		encoded, err = marshaler.Marshal(caseMismatch)
		require.NoError(t, err)

		var decodedMismatch CaseSensitive
		err = unmarshaler.Unmarshal(encoded, &decodedMismatch)
		require.NoError(t, err)

		// Verify fxamacker/cbor's case sensitivity behavior
		// Tags are case-sensitive, field names may have fallback
		assert.Equal(t, "value1", decodedMismatch.FieldOne) // Field name match
		assert.Equal(t, "value2", decodedMismatch.FieldTwo) // Case-insensitive fallback

		// Test with our implementation
		var ourDecoded CaseSensitive
		err = Unmarshal(encoded, &ourDecoded)
		require.NoError(t, err)
		assert.Equal(t, decodedMismatch, ourDecoded)
	})

	t.Run("embedded struct position", func(t *testing.T) {
		// Test embedded structs in different positions within the parent struct
		type EmbeddedFields struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Status string // no tag
		}

		// Embedded as first field
		type EmbeddedFirst struct {
			EmbeddedFields
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		// Embedded as middle field
		type EmbeddedMiddle struct {
			Name string `json:"name"`
			EmbeddedFields
			Value int `json:"value"`
		}

		// Embedded as last field
		type EmbeddedLast struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
			EmbeddedFields
		}

		// Multiple embedded structs
		type AnotherEmbedded struct {
			Code  string `json:"code"`
			Count int
		}

		type MultipleEmbedded struct {
			Name string `json:"name"`
			EmbeddedFields
			Value int `json:"value"`
			AnotherEmbedded
			Extra string
		}

		testData := map[string]any{
			"id":     "test-id",
			"type":   "test-type",
			"Status": "test-status",
			"name":   "test-name",
			"value":  42,
			"code":   "test-code",
			"Count":  99,
			"Extra":  "test-extra",
		}

		// Test embedded as first field
		t.Run("embedded first", func(t *testing.T) {
			encoded, err := marshaler.Marshal(testData)
			require.NoError(t, err)

			var fxFirst EmbeddedFirst
			err = unmarshaler.Unmarshal(encoded, &fxFirst)
			require.NoError(t, err)

			var ourFirst EmbeddedFirst
			err = Unmarshal(encoded, &ourFirst)
			require.NoError(t, err)

			// Verify both implementations handle embedded fields correctly
			assert.Equal(t, "test-id", fxFirst.ID, "fxamacker embedded ID")
			assert.Equal(t, "test-type", fxFirst.Type, "fxamacker embedded Type")
			assert.Equal(t, "test-status", fxFirst.Status, "fxamacker embedded Status")
			assert.Equal(t, "test-name", fxFirst.Name, "fxamacker Name")
			assert.Equal(t, 42, fxFirst.Value, "fxamacker Value")

			assert.Equal(t, fxFirst, ourFirst, "our implementation should match fxamacker")
		})

		// Test embedded as middle field
		t.Run("embedded middle", func(t *testing.T) {
			encoded, err := marshaler.Marshal(testData)
			require.NoError(t, err)

			var fxMiddle EmbeddedMiddle
			err = unmarshaler.Unmarshal(encoded, &fxMiddle)
			require.NoError(t, err)

			var ourMiddle EmbeddedMiddle
			err = Unmarshal(encoded, &ourMiddle)
			require.NoError(t, err)

			// Verify both implementations handle embedded fields correctly
			assert.Equal(t, "test-name", fxMiddle.Name, "fxamacker Name")
			assert.Equal(t, "test-id", fxMiddle.ID, "fxamacker embedded ID")
			assert.Equal(t, "test-type", fxMiddle.Type, "fxamacker embedded Type")
			assert.Equal(t, "test-status", fxMiddle.Status, "fxamacker embedded Status")
			assert.Equal(t, 42, fxMiddle.Value, "fxamacker Value")

			assert.Equal(t, fxMiddle, ourMiddle, "our implementation should match fxamacker")
		})

		// Test embedded as last field
		t.Run("embedded last", func(t *testing.T) {
			encoded, err := marshaler.Marshal(testData)
			require.NoError(t, err)

			var fxLast EmbeddedLast
			err = unmarshaler.Unmarshal(encoded, &fxLast)
			require.NoError(t, err)

			var ourLast EmbeddedLast
			err = Unmarshal(encoded, &ourLast)
			require.NoError(t, err)

			// Verify both implementations handle embedded fields correctly
			assert.Equal(t, "test-name", fxLast.Name, "fxamacker Name")
			assert.Equal(t, 42, fxLast.Value, "fxamacker Value")
			assert.Equal(t, "test-id", fxLast.ID, "fxamacker embedded ID")
			assert.Equal(t, "test-type", fxLast.Type, "fxamacker embedded Type")
			assert.Equal(t, "test-status", fxLast.Status, "fxamacker embedded Status")

			assert.Equal(t, fxLast, ourLast, "our implementation should match fxamacker")
		})

		// Test multiple embedded structs
		t.Run("multiple embedded", func(t *testing.T) {
			encoded, err := marshaler.Marshal(testData)
			require.NoError(t, err)

			var fxMultiple MultipleEmbedded
			err = unmarshaler.Unmarshal(encoded, &fxMultiple)
			require.NoError(t, err)

			var ourMultiple MultipleEmbedded
			err = Unmarshal(encoded, &ourMultiple)
			require.NoError(t, err)

			// Verify both implementations handle multiple embedded fields correctly
			assert.Equal(t, "test-name", fxMultiple.Name, "fxamacker Name")
			assert.Equal(t, "test-id", fxMultiple.ID, "fxamacker first embedded ID")
			assert.Equal(t, "test-type", fxMultiple.Type, "fxamacker first embedded Type")
			assert.Equal(t, "test-status", fxMultiple.Status, "fxamacker first embedded Status")
			assert.Equal(t, 42, fxMultiple.Value, "fxamacker Value")
			assert.Equal(t, "test-code", fxMultiple.Code, "fxamacker second embedded Code")
			assert.Equal(t, 99, fxMultiple.Count, "fxamacker second embedded Count")
			assert.Equal(t, "test-extra", fxMultiple.Extra, "fxamacker Extra")

			assert.Equal(t, fxMultiple, ourMultiple, "our implementation should match fxamacker")
		})

		// Test field shadowing - when parent and embedded have same field names
		t.Run("field shadowing", func(t *testing.T) {
			type ShadowEmbedded struct {
				Name string `json:"shadow_name"`
				ID   string
			}

			type ShadowParent struct {
				Name string `json:"name"` // Different tag from embedded
				ShadowEmbedded
				Value int
			}

			shadowData := map[string]any{
				"name":        "parent-name",
				"shadow_name": "embedded-name",
				"ID":          "test-id",
				"Value":       100,
			}

			encoded, err := marshaler.Marshal(shadowData)
			require.NoError(t, err)

			var fxShadow ShadowParent
			err = unmarshaler.Unmarshal(encoded, &fxShadow)
			require.NoError(t, err)

			var ourShadow ShadowParent
			err = Unmarshal(encoded, &ourShadow)
			require.NoError(t, err)

			// Parent field should shadow embedded field
			assert.Equal(t, "parent-name", fxShadow.Name, "fxamacker parent Name")
			assert.Equal(t, "embedded-name", fxShadow.ShadowEmbedded.Name, "fxamacker embedded Name")
			assert.Equal(t, "test-id", fxShadow.ID, "fxamacker embedded ID")
			assert.Equal(t, 100, fxShadow.Value, "fxamacker Value")

			assert.Equal(t, fxShadow, ourShadow, "our implementation should match fxamacker")
		})
	})
}

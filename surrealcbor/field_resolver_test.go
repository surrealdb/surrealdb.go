package surrealcbor

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFieldResolverConsistency verifies that BasicFieldResolver and CachedFieldResolver
// produce identical results for all field resolution scenarios
//
//nolint:gocyclo
func TestFieldResolverConsistency(t *testing.T) {
	basicResolver := NewBasicFieldResolver()
	cachedResolver := NewCachedFieldResolver()

	t.Run("simple struct", func(t *testing.T) {
		type Simple struct {
			Name  string `json:"name"`
			Value int    `cbor:"val"`
			Count int
		}

		s := Simple{}
		v := reflect.ValueOf(&s).Elem()

		testCases := []struct {
			fieldName string
			expected  string // expected field name or empty if not found
		}{
			{"name", "Name"},   // JSON tag match
			{"Name", "Name"},   // Case-insensitive tag match (fxamacker behavior)
			{"val", "Value"},   // CBOR tag match
			{"Val", "Value"},   // Case-insensitive tag match (fxamacker behavior)
			{"VAL", "Value"},   // Case-insensitive tag match (fxamacker behavior)
			{"Value", ""},      // Field name not matched when tag exists
			{"value", ""},      // Field name not matched when tag exists (case-insensitive)
			{"Count", "Count"}, // Exact field name match
			{"count", "Count"}, // Case-insensitive field name match
			{"COUNT", "Count"}, // Case-insensitive field name match
			{"unknown", ""},    // Non-existent field
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				if tc.expected == "" {
					assert.False(t, basicField.IsValid(), "basic resolver should not find field")
					assert.False(t, cachedField.IsValid(), "cached resolver should not find field")
				} else {
					require.True(t, basicField.IsValid(), "basic resolver should find field")
					require.True(t, cachedField.IsValid(), "cached resolver should find field")

					// Verify they found the same field by checking they point to the same struct field
					// We'll modify through one and check the other sees the change
					switch tc.expected {
					case "Name":
						basicField.SetString("test_value")
						assert.Equal(t, "test_value", s.Name, "basic resolver should set correct field")
						// Reset and test cached resolver
						s.Name = ""
						cachedField.SetString("test_value")
						assert.Equal(t, "test_value", s.Name, "cached resolver should set correct field")
					case "Value":
						basicField.SetInt(100)
						assert.Equal(t, 100, s.Value, "basic resolver should set correct field")
						// Reset and test cached resolver
						s.Value = 0
						cachedField.SetInt(100)
						assert.Equal(t, 100, s.Value, "cached resolver should set correct field")
					case "Count":
						basicField.SetInt(100)
						assert.Equal(t, 100, s.Count, "basic resolver should set correct field")
						// Reset and test cached resolver
						s.Count = 0
						cachedField.SetInt(100)
						assert.Equal(t, 100, s.Count, "cached resolver should set correct field")
					}
				}
			})
		}
	})

	t.Run("embedded struct", func(t *testing.T) {
		type Embedded struct {
			ID   string `json:"id"`
			Type string
		}

		type Container struct {
			Embedded
			Name  string `json:"name"`
			Value int
		}

		c := Container{}
		v := reflect.ValueOf(&c).Elem()

		testCases := []struct {
			fieldName  string
			expected   string // expected field name
			isEmbedded bool
		}{
			{"id", "ID", true},        // Embedded field via tag
			{"ID", "ID", true},        // Case-insensitive tag match (fxamacker behavior)
			{"Type", "Type", true},    // Embedded field direct (no tag)
			{"type", "Type", true},    // Embedded field case-insensitive (no tag)
			{"name", "Name", false},   // Container field via tag
			{"Name", "Name", false},   // Case-insensitive tag match (fxamacker behavior)
			{"Value", "Value", false}, // Container field direct (no tag)
			{"value", "Value", false}, // Container field case-insensitive (no tag)
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				if tc.expected == "" {
					assert.False(t, basicField.IsValid(), "basic resolver should not find field")
					assert.False(t, cachedField.IsValid(), "cached resolver should not find field")
					return
				}

				require.True(t, basicField.IsValid(), "basic resolver should find field")
				require.True(t, cachedField.IsValid(), "cached resolver should find field")

				// Verify they point to the same field by modifying and checking
				switch tc.expected {
				case "ID":
					basicField.SetString("test_value")
					assert.Equal(t, "test_value", c.ID)
					c.ID = ""
					cachedField.SetString("test_value")
					assert.Equal(t, "test_value", c.ID)
				case "Type":
					basicField.SetString("test_value")
					assert.Equal(t, "test_value", c.Type)
					c.Type = ""
					cachedField.SetString("test_value")
					assert.Equal(t, "test_value", c.Type)
				case "Name":
					basicField.SetString("test_value")
					assert.Equal(t, "test_value", c.Name)
					c.Name = ""
					cachedField.SetString("test_value")
					assert.Equal(t, "test_value", c.Name)
				case "Value":
					basicField.SetInt(100)
					assert.Equal(t, 100, c.Value)
					c.Value = 0
					cachedField.SetInt(100)
					assert.Equal(t, 100, c.Value)
				}
			})
		}
	})

	t.Run("deeply nested struct", func(t *testing.T) {
		type Level3 struct {
			Value string `json:"value"`
			Count int    `json:"count"`
			Name  string // Field with same name as Level2
		}

		type Level2 struct {
			Name  string `json:"name2"` // Different tag to avoid conflict
			Items Level3 `json:"items"`
			Count int    // Field with same name as Level3
		}

		type Level1 struct {
			ID     string `json:"id"`
			Nested Level2 `json:"nested"`
			Name   string // Field with same name as Level2 and Level3
		}

		type Root struct {
			Level1
			Root  string `json:"root"`
			Value string // Field with same name as Level3
		}

		r := Root{}
		v := reflect.ValueOf(&r).Elem()

		testCases := []struct {
			fieldName string
			desc      string
		}{
			{"root", "Root field via tag"},
			{"Root", "Root field direct"},
			{"id", "Level1 embedded field via tag"},
			{"ID", "Level1 embedded field direct"},
			{"Name", "Level1 embedded field (conflicts resolved by outer precedence)"},
			{"name", "Level1 embedded field case-insensitive"},
			{"Value", "Root field (takes precedence over deeper nested)"},
			{"value", "Root field case-insensitive"},
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				// Both should find the same field or both should not find it
				assert.Equal(t, basicField.IsValid(), cachedField.IsValid(),
					"both resolvers should agree on field validity for %s", tc.fieldName)

				if basicField.IsValid() && cachedField.IsValid() {
					// Verify they found the same field by checking types
					assert.Equal(t, basicField.Type(), cachedField.Type(),
						"both resolvers should find the same field type for %s", tc.fieldName)
				}
			})
		}
	})

	t.Run("multiple embedded structs with conflicts", func(t *testing.T) {
		type Base1 struct {
			ID   string `json:"id"`
			Name string `json:"name1"`
			Type string
		}

		type Base2 struct {
			Code string `json:"code"`
			Name string `json:"name2"`
			Type string
		}

		type Container struct {
			Base1
			Base2
			Name string `json:"name"` // Shadows embedded Name fields
		}

		c := Container{}
		v := reflect.ValueOf(&c).Elem()

		testCases := []struct {
			fieldName string
			desc      string
		}{
			{"id", "Base1.ID via tag"},
			{"code", "Base2.Code via tag"},
			{"name", "Container.Name via tag (shadows embedded)"},
			{"name1", "Base1.Name via its specific tag"},
			{"name2", "Base2.Name via its specific tag"},
			{"Name", "Container.Name (shadows embedded)"},
			{"Type", "Ambiguous embedded field"},
			{"type", "Ambiguous embedded field case-insensitive"},
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				// For ambiguous fields like "Type", both might find Base1.Type
				// The important thing is they both behave the same way
				assert.Equal(t, basicField.IsValid(), cachedField.IsValid(),
					"both resolvers should agree on field validity for %s", tc.fieldName)

				if basicField.IsValid() && cachedField.IsValid() {
					// They should find fields of the same type
					assert.Equal(t, basicField.Type(), cachedField.Type(),
						"both resolvers should find the same field type for %s", tc.fieldName)
				}
			})
		}
	})

	t.Run("tag precedence over field names", func(t *testing.T) {
		type TagTest struct {
			FieldA string `json:"b" cbor:"c"` // CBOR tag takes precedence
			FieldB string `json:"d"`          // JSON tag
			FieldC string // No tag
		}

		tt := TagTest{}
		v := reflect.ValueOf(&tt).Elem()

		testCases := []struct {
			fieldName  string
			shouldFind bool
			fieldSet   string // which field should be set
		}{
			{"c", true, "FieldA"},      // CBOR tag
			{"C", true, "FieldA"},      // Case-insensitive tag match
			{"b", false, ""},           // JSON tag is overridden by CBOR
			{"B", false, ""},           // JSON tag is overridden by CBOR (case-insensitive)
			{"d", true, "FieldB"},      // JSON tag
			{"D", true, "FieldB"},      // Case-insensitive tag match
			{"FieldA", false, ""},      // Field name not matched when tags exist
			{"FieldB", false, ""},      // Field name not matched when tags exist
			{"FieldC", true, "FieldC"}, // Exact field name (no tag)
			{"fielda", false, ""},      // Field name not matched when tags exist
			{"fieldb", false, ""},      // Field name not matched when tags exist
			{"fieldc", true, "FieldC"}, // Case-insensitive field name (no tag)
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				assert.Equal(t, tc.shouldFind, basicField.IsValid(),
					"basic resolver field validity for %s", tc.fieldName)
				assert.Equal(t, tc.shouldFind, cachedField.IsValid(),
					"cached resolver field validity for %s", tc.fieldName)

				if tc.shouldFind {
					// Verify they found the same field
					switch tc.fieldSet {
					case "FieldA":
						basicField.SetString("test_value")
						assert.Equal(t, "test_value", tt.FieldA)
						tt.FieldA = ""
						cachedField.SetString("test_value")
						assert.Equal(t, "test_value", tt.FieldA)
					case "FieldB":
						basicField.SetString("test_value")
						assert.Equal(t, "test_value", tt.FieldB)
						tt.FieldB = ""
						cachedField.SetString("test_value")
						assert.Equal(t, "test_value", tt.FieldB)
					case "FieldC":
						basicField.SetString("test_value")
						assert.Equal(t, "test_value", tt.FieldC)
						tt.FieldC = ""
						cachedField.SetString("test_value")
						assert.Equal(t, "test_value", tt.FieldC)
					}
				}
			})
		}
	})

	t.Run("unexported fields", func(t *testing.T) {
		type Private struct {
			Public  string `json:"public"`
			private string //nolint:unused // intentionally unused for testing unexported fields
		}

		p := Private{}
		v := reflect.ValueOf(&p).Elem()

		testCases := []struct {
			fieldName  string
			shouldFind bool
		}{
			{"public", true},   // Public field via tag
			{"Public", true},   // Case-insensitive tag match (fxamacker behavior)
			{"private", false}, // Should not be found (unexported)
			{"Private", false}, // Should not be found (unexported)
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				assert.Equal(t, tc.shouldFind, basicField.IsValid(),
					"basic resolver should%s find %s", map[bool]string{true: "", false: " not"}[tc.shouldFind], tc.fieldName)
				assert.Equal(t, tc.shouldFind, cachedField.IsValid(),
					"cached resolver should%s find %s", map[bool]string{true: "", false: " not"}[tc.shouldFind], tc.fieldName)
				assert.Equal(t, basicField.IsValid(), cachedField.IsValid(),
					"both resolvers should agree on field validity for %s", tc.fieldName)
			})
		}
	})

	t.Run("empty tags", func(t *testing.T) {
		type EmptyTags struct {
			Field1 string `json:""`           // Empty tag
			Field2 string `json:"-"`          // Ignored field
			Field3 string `json:",omitempty"` // Only options
		}

		et := EmptyTags{}
		v := reflect.ValueOf(&et).Elem()

		testCases := []struct {
			fieldName  string
			shouldFind bool
		}{
			{"Field1", true}, // Found by field name
			{"field1", true}, // Case-insensitive
			{"Field2", true}, // Found by field name (- means skip in encoding, not in field lookup)
			{"field2", true}, // Case-insensitive
			{"Field3", true}, // Found by field name
			{"field3", true}, // Case-insensitive
			{"", false},      // Empty string shouldn't match empty tag
			{"-", false},     // Dash shouldn't match as tag
		}

		for _, tc := range testCases {
			t.Run(tc.fieldName, func(t *testing.T) {
				basicField := basicResolver.FindField(v, tc.fieldName)
				cachedField := cachedResolver.FindField(v, tc.fieldName)

				assert.Equal(t, tc.shouldFind, basicField.IsValid(),
					"basic resolver field validity for %s", tc.fieldName)
				assert.Equal(t, tc.shouldFind, cachedField.IsValid(),
					"cached resolver field validity for %s", tc.fieldName)
			})
		}
	})
}

// TestCachedFieldResolverCaching verifies that the cached resolver actually caches
func TestCachedFieldResolverCaching(t *testing.T) {
	type TestStruct struct {
		Field1 string `json:"f1"`
		Field2 int    `cbor:"f2"`
		Field3 bool
	}

	cachedResolver := NewCachedFieldResolver().(*CachedFieldResolver)

	// First access - should build cache
	s1 := TestStruct{}
	v1 := reflect.ValueOf(&s1).Elem()

	field1 := cachedResolver.FindField(v1, "f1")
	assert.True(t, field1.IsValid())

	// Check that cache has been populated
	cachedResolver.cache.mu.RLock()
	_, exists := cachedResolver.cache.cache[v1.Type()]
	cachedResolver.cache.mu.RUnlock()
	assert.True(t, exists, "cache should be populated after first access")

	// Second access with same type - should use cache
	s2 := TestStruct{}
	v2 := reflect.ValueOf(&s2).Elem()

	field2 := cachedResolver.FindField(v2, "f2")
	assert.True(t, field2.IsValid())

	// Multiple fields from same struct
	field3 := cachedResolver.FindField(v2, "Field3")
	assert.True(t, field3.IsValid())

	field4 := cachedResolver.FindField(v2, "field3") // case-insensitive
	assert.True(t, field4.IsValid())
}

// TestFieldResolverThreadSafety verifies that CachedFieldResolver is thread-safe
func TestFieldResolverThreadSafety(t *testing.T) {
	type TestStruct struct {
		Field1 string `json:"f1"`
		Field2 int    `cbor:"f2"`
		Field3 bool
	}

	cachedResolver := NewCachedFieldResolver()
	basicResolver := NewBasicFieldResolver()

	// Run concurrent field resolutions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			s := TestStruct{}
			v := reflect.ValueOf(&s).Elem()

			// Try various field names
			fieldNames := []string{"f1", "f2", "Field3", "field3", "Field1", "field2"}

			for _, name := range fieldNames {
				cachedField := cachedResolver.FindField(v, name)
				basicField := basicResolver.FindField(v, name)

				// They should always agree
				if cachedField.IsValid() != basicField.IsValid() {
					t.Errorf("goroutine %d: resolvers disagree on field %s", id, name)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

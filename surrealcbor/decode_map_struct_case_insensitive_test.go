package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_map_structCaseInsensitive tests case-insensitive field matching
func TestDecode_map_structCaseInsensitive(t *testing.T) {
	type Person struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Age       int    `json:"age"`
	}

	t.Run("exact case match (should work as before)", func(t *testing.T) {
		data := map[string]any{
			"firstName": "John",
			"lastName":  "Doe",
			"email":     "john@example.com",
			"age":       30,
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded Person
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "John", decoded.FirstName)
		assert.Equal(t, "Doe", decoded.LastName)
		assert.Equal(t, "john@example.com", decoded.Email)
		assert.Equal(t, 30, decoded.Age)
	})

	t.Run("json tags are case-sensitive but field names fallback", func(t *testing.T) {
		// Tags must match exactly, but if tag doesn't match, it falls back to field name (case-insensitive)
		data := map[string]any{
			"firstname": "Jane",             // Won't match "firstName" tag, but matches FirstName field
			"lastname":  "Smith",            // Won't match "lastName" tag, but matches LastName field
			"email":     "jane@example.com", // Exact tag match
			"age":       25,                 // Exact tag match
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded Person
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		// Even though tags don't match, field names match case-insensitively
		assert.Equal(t, "Jane", decoded.FirstName)
		assert.Equal(t, "Smith", decoded.LastName)
		assert.Equal(t, "jane@example.com", decoded.Email)
		assert.Equal(t, 25, decoded.Age)
	})

	t.Run("tags take precedence over field names", func(t *testing.T) {
		type User struct {
			UserName string `json:"username"` // tag is "username"
			FullName string `json:"name"`     // tag is "name"
		}

		// "USERNAME" won't match the "username" tag (case-sensitive)
		// but will match the UserName field (case-insensitive)
		data := map[string]any{
			"USERNAME": "john123",  // Won't match tag, but matches field
			"name":     "John Doe", // Exact tag match
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded User
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "john123", decoded.UserName)
		assert.Equal(t, "John Doe", decoded.FullName)
	})

	t.Run("struct without json tags", func(t *testing.T) {
		type Product struct {
			Name        string
			Description string
			Price       float64
		}

		data := map[string]any{
			"name":        "Laptop",
			"description": "High-performance laptop",
			"price":       999.99,
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded Product
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "Laptop", decoded.Name)
		assert.Equal(t, "High-performance laptop", decoded.Description)
		assert.Equal(t, 999.99, decoded.Price)
	})

	t.Run("case insensitive match for struct field names", func(t *testing.T) {
		type Product struct {
			Name        string
			Description string
			Price       float64
		}

		// Test with uppercase keys
		data := map[string]any{
			"NAME":        "Mouse",
			"DESCRIPTION": "Wireless mouse",
			"PRICE":       29.99,
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded Product
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "Mouse", decoded.Name)
		assert.Equal(t, "Wireless mouse", decoded.Description)
		assert.Equal(t, 29.99, decoded.Price)
	})

	t.Run("exact match takes precedence over case-insensitive", func(t *testing.T) {
		type TestStruct struct {
			MyField string
			Myfield string
		}

		// Test that exact match is found before case-insensitive
		// Using different field names to avoid map ordering issues
		data := map[string]any{
			"Myfield": "exact match",
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded TestStruct
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "", decoded.MyField)
		assert.Equal(t, "exact match", decoded.Myfield)
	})

	t.Run("embedded struct with case-insensitive field names", func(t *testing.T) {
		type Address struct {
			Street string
			City   string
		}

		type Person struct {
			Address
			Name string
		}

		// Using uppercase field names - should match via case-insensitive field name matching
		data := map[string]any{
			"NAME":   "Charlie",
			"STREET": "123 Main St",
			"CITY":   "Springfield",
		}

		encoded, err := Marshal(data)
		require.NoError(t, err)

		var decoded Person
		err = Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "Charlie", decoded.Name)
		assert.Equal(t, "123 Main St", decoded.Street)
		assert.Equal(t, "Springfield", decoded.City)
	})
}

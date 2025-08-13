package surrealcbor

import (
	"testing"
)

// BenchmarkFieldResolver compares the performance of basic vs cached field resolver
func BenchmarkFieldResolver(b *testing.B) {
	// Complex struct for benchmarking
	type Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		ZipCode string `json:"zip_code"`
		Country string `json:"country"`
	}

	type Person struct {
		ID        string   `json:"id"`
		FirstName string   `json:"first_name"`
		LastName  string   `json:"last_name"`
		Email     string   `json:"email"`
		Phone     string   `json:"phone"`
		Age       int      `json:"age"`
		Address   Address  `json:"address"`
		Tags      []string `json:"tags"`
		Active    bool     `json:"active"`
		Score     float64  `json:"score"`
	}

	// Sample data with various field name cases
	data := map[string]any{
		"id":         "person-123",
		"first_name": "John",
		"last_name":  "Doe",
		"email":      "john@example.com",
		"phone":      "+1234567890",
		"age":        30,
		"address": map[string]any{
			"street":   "123 Main St",
			"city":     "Springfield",
			"zip_code": "12345",
			"country":  "USA",
		},
		"tags":   []string{"developer", "golang"},
		"active": true,
		"score":  95.5,
	}

	// Encode the data once
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("BasicFieldResolver", func(b *testing.B) {
		basicResolver := NewBasicFieldResolver()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var p Person
			d := &decoder{
				data:          encoded,
				pos:           0,
				fieldResolver: basicResolver,
			}
			if decodeErr := d.decode(&p); decodeErr != nil {
				b.Fatal(decodeErr)
			}
		}
	})

	b.Run("CachedFieldResolver", func(b *testing.B) {
		cachedResolver := NewCachedFieldResolver()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var p Person
			d := &decoder{
				data:          encoded,
				pos:           0,
				fieldResolver: cachedResolver,
			}
			if decodeErr := d.decode(&p); decodeErr != nil {
				b.Fatal(decodeErr)
			}
		}
	})

	// Test with case-insensitive field names
	dataCaseInsensitive := map[string]any{
		"ID":         "person-456",
		"FIRST_NAME": "Jane",
		"LAST_NAME":  "Smith",
		"EMAIL":      "jane@example.com",
		"PHONE":      "+0987654321",
		"AGE":        25,
		"ADDRESS": map[string]any{
			"STREET":   "456 Oak Ave",
			"CITY":     "Metropolis",
			"ZIP_CODE": "67890",
			"COUNTRY":  "Canada",
		},
		"TAGS":   []string{"manager", "python"},
		"ACTIVE": false,
		"SCORE":  88.0,
	}

	encodedCaseInsensitive, err := Marshal(dataCaseInsensitive)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("BasicFieldResolver_CaseInsensitive", func(b *testing.B) {
		basicResolver := NewBasicFieldResolver()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var p Person
			d := &decoder{
				data:          encodedCaseInsensitive,
				pos:           0,
				fieldResolver: basicResolver,
			}
			if decodeErr := d.decode(&p); decodeErr != nil {
				b.Fatal(decodeErr)
			}
		}
	})

	b.Run("CachedFieldResolver_CaseInsensitive", func(b *testing.B) {
		cachedResolver := NewCachedFieldResolver()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var p Person
			d := &decoder{
				data:          encodedCaseInsensitive,
				pos:           0,
				fieldResolver: cachedResolver,
			}
			if decodeErr := d.decode(&p); decodeErr != nil {
				b.Fatal(decodeErr)
			}
		}
	})
}

// BenchmarkFieldResolverDeepStruct tests performance with deeply nested structs
func BenchmarkFieldResolverDeepStruct(b *testing.B) {
	type Level3 struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	}

	type Level2 struct {
		Name  string `json:"name"`
		Items Level3 `json:"items"`
	}

	type Level1 struct {
		ID     string `json:"id"`
		Nested Level2 `json:"nested"`
	}

	type RootStruct struct {
		Root   string `json:"root"`
		Level1 Level1 `json:"level1"`
	}

	data := map[string]any{
		"root": "root-value",
		"level1": map[string]any{
			"id": "level1-id",
			"nested": map[string]any{
				"name": "level2-name",
				"items": map[string]any{
					"value": "level3-value",
					"count": 42,
				},
			},
		},
	}

	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("BasicFieldResolver", func(b *testing.B) {
		basicResolver := NewBasicFieldResolver()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var r RootStruct
			d := &decoder{
				data:          encoded,
				pos:           0,
				fieldResolver: basicResolver,
			}
			if decodeErr := d.decode(&r); decodeErr != nil {
				b.Fatal(decodeErr)
			}
		}
	})

	b.Run("CachedFieldResolver", func(b *testing.B) {
		cachedResolver := NewCachedFieldResolver()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var r RootStruct
			d := &decoder{
				data:          encoded,
				pos:           0,
				fieldResolver: cachedResolver,
			}
			if decodeErr := d.decode(&r); decodeErr != nil {
				b.Fatal(decodeErr)
			}
		}
	})
}
